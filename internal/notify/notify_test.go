package notify

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

type call struct {
	name      string
	args, env []string
	stdin     string
}

func fake(goos string, fail error) (*System, *[]call) {
	var calls []call
	s := &System{
		goos: goos,
		run: func(_ context.Context, name string, args, env []string, stdin []byte) error {
			calls = append(calls, call{name, args, env, string(stdin)})
			return fail
		},
		look: func(name string) (string, error) { return "/usr/bin/" + name, nil },
	}
	return s, &calls
}

func TestMacOSScriptGoesThroughStdin(t *testing.T) {
	s, calls := fake("darwin", nil)
	note := Note{Title: `Ops "weekly"`, Body: "Report\\ready\" & do shell script \"rm -rf ~\"\nnext"}
	if err := s.Notify(context.Background(), note); err != nil {
		t.Fatal(err)
	}
	c := (*calls)[0]
	if c.name != "/usr/bin/osascript" || len(c.args) != 0 {
		t.Fatalf("text must not reach argv: %+v", c)
	}
	want := `display notification "Report\\ready\" & do shell script \"rm -rf ~\" next" ` +
		`with title "Ops \"weekly\""` + "\n"
	if c.stdin != want {
		t.Fatalf("script:\n%q\nwant\n%q", c.stdin, want)
	}
}

func TestWindowsTextGoesThroughEnvironment(t *testing.T) {
	s, calls := fake("windows", nil)
	if err := s.Notify(context.Background(), Note{Title: "$(evil)", Body: "<toast/>"}); err != nil {
		t.Fatal(err)
	}
	c := (*calls)[0]
	if c.name != "powershell.exe" || c.args[len(c.args)-1] != toastScript {
		t.Fatalf("command %+v", c)
	}
	if strings.Contains(strings.Join(c.args, " "), "evil") {
		t.Fatal("text reached the command line")
	}
	if c.env[0] != "TESSIVO_NOTE_TITLE=$(evil)" || c.env[1] != "TESSIVO_NOTE_BODY=<toast/>" {
		t.Fatalf("env %v", c.env)
	}
}

func TestLinuxUsesNotifySendWithSeparator(t *testing.T) {
	s, calls := fake("linux", nil)
	if err := s.Notify(context.Background(), Note{Title: "", Body: "--help"}); err != nil {
		t.Fatal(err)
	}
	c := (*calls)[0]
	if c.name != "/usr/bin/notify-send" || strings.Join(c.args, "|") != "--app-name=Tessivo|--|Tessivo|--help" {
		t.Fatalf("command %+v", c)
	}
	s.look = func(string) (string, error) { return "", exec.ErrNotFound }
	if err := s.Notify(context.Background(), Note{Title: "x"}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing tool: %v", err)
	}
	boom := errors.New("boom")
	s2, _ := fake("linux", boom)
	if err := s2.Notify(context.Background(), Note{Title: "x"}); !errors.Is(err, boom) {
		t.Fatalf("tool failure: %v", err)
	}
}

func TestClean(t *testing.T) {
	cases := map[string]string{
		"a\nb\r\tc":        "a b c",
		"  spaced   out  ": "spaced out",
		"bad\xffutf8":      "badutf8",
		"line separator":   "line separator",
	}
	for in, want := range cases {
		if got := Clean(in, 100); got != want {
			t.Errorf("Clean(%q) = %q, want %q", in, got, want)
		}
	}
	if got := Clean(strings.Repeat("é", 10), 5); got != "éééé…" {
		t.Fatalf("truncate %q", got)
	}
	if OS().goos != runtime.GOOS {
		t.Fatal("OS() should target this system")
	}
}

func TestExecRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX tools")
	}
	if err := execRun(context.Background(), "true", nil, []string{"X=1"}, nil); err != nil {
		t.Fatal(err)
	}
	err := execRun(context.Background(), "sh", []string{"-c", "echo nope >&2; exit 3"}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("stderr not reported: %v", err)
	}
	if err := execRun(context.Background(), "false", nil, nil, nil); err == nil {
		t.Fatal("failure not reported")
	}
}

type recorder struct {
	mu    sync.Mutex
	notes []Note
	err   error
	got   chan struct{}
}

func (r *recorder) Notify(_ context.Context, n Note) error {
	r.mu.Lock()
	r.notes = append(r.notes, n)
	r.mu.Unlock()
	r.got <- struct{}{}
	return r.err
}

func TestDispatcherRateLimitsAndSummarizesDrops(t *testing.T) {
	rec := &recorder{got: make(chan struct{}, 10)}
	clock := time.Unix(0, 0)
	var mu sync.Mutex
	d := &Dispatcher{N: rec, Burst: 2, Window: time.Minute}
	d.init()
	d.now = func() time.Time { mu.Lock(); defer mu.Unlock(); return clock }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go d.Run(ctx)

	for i := range 4 {
		d.Send(Note{Title: "t", Body: string(rune('a' + i))})
	}
	<-rec.got
	<-rec.got
	time.Sleep(50 * time.Millisecond) // let the two over-limit notes be counted
	mu.Lock()
	clock = clock.Add(2 * time.Minute)
	mu.Unlock()
	d.Send(Note{Title: "t", Body: "e"})
	<-rec.got
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.notes) != 3 || rec.notes[2].Body != "e (+2 more)" {
		t.Fatalf("notes %+v", rec.notes)
	}
}

func TestDispatcherQueueFullAndErrors(t *testing.T) {
	d := &Dispatcher{N: &recorder{got: make(chan struct{}, 100)}}
	for range queueSize {
		if !d.Send(Note{}) {
			t.Fatal("queue should accept up to its size")
		}
	}
	if d.Send(Note{}) || d.dropped != 1 {
		t.Fatalf("overflow not dropped: %d", d.dropped)
	}

	rec := &recorder{got: make(chan struct{}, 1), err: errors.New("no display")}
	errs := make(chan error, 1)
	d2 := &Dispatcher{N: rec, OnErr: func(err error) { errs <- err }}
	ctx, cancel := context.WithCancel(context.Background())
	go d2.Run(ctx)
	d2.Send(Note{Title: "x"})
	if err := <-errs; err == nil || err.Error() != "no display" {
		t.Fatalf("error not reported: %v", err)
	}
	cancel()
}
