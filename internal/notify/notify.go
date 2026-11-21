// Package notify shows desktop notifications through each operating system's
// own tool, so no notification library is needed: AppleScript on macOS (the
// script arrives on stdin, never argv), notify-send on Linux and a fixed
// PowerShell toast script on Windows that reads its text from environment
// variables. Text is never interpolated into a command line or a shell.
package notify

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Limits keep notifications short and the tools fast.
const (
	MaxTitle = 100 // runes
	MaxBody  = 300 // runes
	timeout  = 5 * time.Second
)

// ErrUnavailable means this system has no supported notification tool.
var ErrUnavailable = errors.New("desktop notifications are not available on this system")

// Note is one notification.
type Note struct {
	Title string
	Body  string
}

// Notifier shows notifications.
type Notifier interface {
	Notify(ctx context.Context, n Note) error
}

// runFunc runs a program with stdin and extra environment; tests replace it.
type runFunc func(ctx context.Context, name string, args, env []string, stdin []byte) error

// System notifies through the current OS's tool.
type System struct {
	goos string
	run  runFunc
	look func(string) (string, error)
}

// OS returns the notifier for this operating system.
func OS() *System { return &System{goos: runtime.GOOS, run: execRun, look: exec.LookPath} }

// Notify shows n. Text is cleaned (no control characters) and shortened first.
func (s *System) Notify(ctx context.Context, n Note) error {
	title, body := Clean(n.Title, MaxTitle), Clean(n.Body, MaxBody)
	if title == "" {
		title = "Tessivo"
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	switch s.goos {
	case "darwin":
		return s.run(ctx, "/usr/bin/osascript", nil, nil, []byte(appleScript(title, body)))
	case "windows":
		env := []string{"TESSIVO_NOTE_TITLE=" + title, "TESSIVO_NOTE_BODY=" + body}
		args := []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", toastScript}
		return s.run(ctx, "powershell.exe", args, env, nil)
	default:
		path, err := s.look("notify-send")
		if err != nil {
			return ErrUnavailable
		}
		return s.run(ctx, path, []string{"--app-name=Tessivo", "--", title, body}, nil, nil)
	}
}

// appleScript builds the script fed to osascript on stdin. Only the two
// quoted literals vary, and Clean already removed control characters.
func appleScript(title, body string) string {
	return fmt.Sprintf("display notification %s with title %s\n", appleQuote(body), appleQuote(title))
}

func appleQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// toastScript is fixed; the text comes from environment variables and
// CreateTextNode escapes it as XML. The AppUserModelID is PowerShell's, which
// Windows accepts without app registration.
const toastScript = `$ErrorActionPreference = 'Stop'
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null
$x = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent(` +
	`[Windows.UI.Notifications.ToastTemplateType]::ToastText02)
$t = $x.GetElementsByTagName('text')
$t.Item(0).AppendChild($x.CreateTextNode($env:TESSIVO_NOTE_TITLE)) > $null
$t.Item(1).AppendChild($x.CreateTextNode($env:TESSIVO_NOTE_BODY)) > $null
$id = '{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe'
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($id).Show(` +
	`[Windows.UI.Notifications.ToastNotification]::new($x))`

// Clean makes s safe to display: valid UTF-8, control characters (including
// newlines) turned into spaces, runs of spaces collapsed, at most max runes.
func Clean(s string, maxRunes int) string {
	s = strings.ToValidUTF8(s, "")
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == ' ' || r == ' ' {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > maxRunes {
		r := []rune(s)
		s = strings.TrimSpace(string(r[:maxRunes-1])) + "…"
	}
	return s
}

func execRun(ctx context.Context, name string, args, env []string, stdin []byte) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = bytes.NewReader(stdin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := Clean(stderr.String(), 200)
		if msg == "" {
			return fmt.Errorf("notify: %w", err)
		}
		return fmt.Errorf("notify: %w: %s", err, msg)
	}
	return nil
}
