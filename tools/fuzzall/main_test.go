package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

const listing = `FuzzVerify
ok  	example.com/m/audit	0.070s
?   	example.com/m/cmd	[no test files]
ok  	example.com/m/chat	0.045s
FuzzCSV
FuzzOffice
ok  	example.com/m/extract	(cached)
`

type fakeGo struct {
	calls []string
	fail  map[string]bool
	list  string
}

func (f *fakeGo) run(_ context.Context, _ bool, args ...string) (string, error) {
	line := strings.Join(args, " ")
	f.calls = append(f.calls, line)
	if strings.HasPrefix(line, "test -list") {
		if f.fail["list"] {
			return "build failed", errors.New("exit 1")
		}
		return f.list, nil
	}
	for k := range f.fail {
		if strings.Contains(line, k) {
			return "", errors.New("exit 1")
		}
	}
	return "", nil
}

func TestRunFuzzesEveryTarget(t *testing.T) {
	f := &fakeGo{list: listing}
	var out bytes.Buffer
	if code := run(context.Background(), []string{"-fuzztime", "3s"}, &out, f.run); code != 0 {
		t.Fatalf("code %d\n%s", code, &out)
	}
	want := []string{
		"test -list ^Fuzz ./...",
		"test -run ^$ -fuzz ^FuzzVerify$ -fuzztime 3s example.com/m/audit",
		"test -run ^$ -fuzz ^FuzzCSV$ -fuzztime 3s example.com/m/extract",
		"test -run ^$ -fuzz ^FuzzOffice$ -fuzztime 3s example.com/m/extract",
	}
	if strings.Join(f.calls, "\n") != strings.Join(want, "\n") {
		t.Errorf("calls:\n%s", strings.Join(f.calls, "\n"))
	}
	if !strings.Contains(out.String(), "3 target(s), 0 failed") {
		t.Errorf("output:\n%s", &out)
	}
}

func TestRunReportsFailuresAndContinues(t *testing.T) {
	f := &fakeGo{list: listing, fail: map[string]bool{"FuzzCSV": true}}
	var out bytes.Buffer
	if code := run(context.Background(), []string{"./internal/..."}, &out, f.run); code != 1 {
		t.Fatalf("code %d", code)
	}
	if !strings.Contains(out.String(), "FAIL example.com/m/extract FuzzCSV") || len(f.calls) != 4 {
		t.Errorf("output:\n%s\ncalls %v", &out, f.calls)
	}
	if f.calls[0] != "test -list ^Fuzz ./internal/..." {
		t.Errorf("packages not passed: %s", f.calls[0])
	}
}

func TestRunErrors(t *testing.T) {
	var out bytes.Buffer
	if code := run(context.Background(), []string{"-nope"}, &out, (&fakeGo{}).run); code != 2 {
		t.Errorf("bad flag: %d", code)
	}
	f := &fakeGo{fail: map[string]bool{"list": true}}
	code := run(context.Background(), nil, &out, f.run)
	if code != 2 || !strings.Contains(out.String(), "build failed") {
		t.Errorf("list failure: %d\n%s", code, &out)
	}
	f = &fakeGo{list: "FuzzX\n"}
	if code := run(context.Background(), nil, &out, f.run); code != 2 {
		t.Errorf("orphan target: %d", code)
	}
}

func TestParseList(t *testing.T) {
	if _, err := parseList("FuzzA\nok\n"); err == nil {
		t.Error("short ok line accepted")
	}
	if got, err := parseList("ok  \tx\t0.1s\n"); err != nil || len(got) != 0 {
		t.Errorf("no targets: %v %v", got, err)
	}
	if _, err := parseList(strings.Repeat("FuzzA\n", maxTargets+1) + "ok x 0s\n"); err == nil {
		t.Error("too many targets accepted")
	}
}

func TestExecGo(t *testing.T) {
	var out bytes.Buffer
	g := execGo(&out)
	got, err := g(context.Background(), true, "env", "GOOS")
	if err != nil || strings.TrimSpace(got) == "" || out.Len() == 0 {
		t.Skipf("go not runnable here: %v", err)
	}
	if _, err := g(context.Background(), false, "no-such-command"); err == nil {
		t.Error("bad command succeeded")
	}
}
