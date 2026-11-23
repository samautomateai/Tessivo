package proctree

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"
)

// The test binary doubles as the processes of the tree:
//
//	parent   starts a grandchild that inherits stdout, prints "ready", sleeps
//	sleeper  sleeps
const modeEnv = "PROCTREE_TEST_MODE"

func TestMain(m *testing.M) {
	switch os.Getenv(modeEnv) {
	case "parent":
		exe, _ := os.Executable()
		child := exec.Command(exe)
		child.Env = append(os.Environ(), modeEnv+"=sleeper")
		child.Stdout = os.Stdout
		if err := child.Start(); err != nil {
			os.Exit(2)
		}
		_, _ = os.Stdout.WriteString("ready\n")
		time.Sleep(time.Minute)
		os.Exit(0)
	case "sleeper":
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// startTree starts the parent and waits until its grandchild runs. The
// returned reader reaches EOF only when every process holding stdout is gone.
func startTree(t *testing.T) (*exec.Cmd, *Tree, *bufio.Reader) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), modeEnv+"=parent")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	Prepare(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	tree, err := Attach(cmd)
	if err != nil {
		_ = cmd.Process.Kill()
		t.Fatal(err)
	}
	r := bufio.NewReader(out)
	if line, err := r.ReadString('\n'); err != nil || line != "ready\n" {
		_ = tree.Kill()
		t.Fatalf("parent: %q %v", line, err)
	}
	return cmd, tree, r
}

func TestKillStopsTheWholeTree(t *testing.T) {
	cmd, tree, r := startTree(t)
	defer func() { _ = tree.Release() }()
	if err := tree.Kill(); err != nil {
		t.Fatal(err)
	}
	// Read before Wait: Wait closes the pipe. EOF means no process holds
	// stdout any more, so the grandchild is gone too.
	eof := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, r)
		eof <- err
	}()
	select {
	case err := <-eof:
		if err != nil {
			t.Fatalf("read: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("the grandchild kept running after Kill")
	}
	_ = cmd.Wait()
	if err := tree.Kill(); err != nil {
		t.Fatalf("killing a dead tree: %v", err)
	}
	if err := tree.Release(); err != nil {
		t.Fatal(err)
	}
	if tree.Release() != nil || tree.Kill() != nil {
		t.Fatal("Release and Kill after Release must be no-ops")
	}
}

func TestAttachNeedsAStartedCommand(t *testing.T) {
	if _, err := Attach(exec.Command("unused")); err == nil {
		t.Fatal("want an error")
	}
}
