package reenvoy

import (
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/go-gatedio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fileWaitSleepDelay = 500 * time.Millisecond

func testProcess(t *testing.T) *Process {

	p := &Process{
		Command:      "echo",
		Args:         []string{"hello", "world"},
		ReloadSignal: os.Interrupt,
		KillSignal:   os.Kill,
		KillTimeout:  2 * time.Second,
		Splay:        0 * time.Second,
	}

	return p
}

func TestStart(t *testing.T) {
	t.Parallel()

	c := testProcess(t)

	// set our own reader and writer so we can verify they are wired to process
	stdin := gatedio.NewByteBuffer()
	stdout := gatedio.NewByteBuffer()
	stderr := gatedio.NewByteBuffer()
	c.Stdin = stdin
	c.Stdout = stdout
	c.StdErr = stderr

	// Custom env and command
	c.Env = []string{"a=b", "c=d"}
	c.Command = "env"
	c.Args = nil

	assert.Nil(t, c.Start())
	defer c.Stop()

	//Wait process finish
	select {
	case <-c.ExitCh():
	case <-time.After(fileWaitSleepDelay):
		t.Fatal("process should have exited")
	}

	expected := "a=b\nc=d\n"
	assert.Equal(t, expected, stdout.String())
}

func TestSignal(t *testing.T) {
	t.Parallel()

	c := testProcess(t)
	c.Command = "bash"
	c.Args = []string{"-c", "trap 'echo one; exit' SIGUSR1; while true; do sleep 0.2; done"}

	out := gatedio.NewByteBuffer()
	c.Stdout, c.StdErr = out, out

	require.Nil(t, c.Start())

	defer c.Stop()

	// For some reason bash doesn't start immediately
	time.Sleep(fileWaitSleepDelay)

	if err := c.Signal(syscall.SIGUSR1); err != nil {
		t.Fatal(err)
	}
	fmt.Println("run", c.running())

	// Give time for the file to flush
	time.Sleep(fileWaitSleepDelay)

	expected := "one\n"
	assert.Equal(t, expected, out.String())
}

func TestReloadSignal(t *testing.T) {
	t.Parallel()

	c := testProcess(t)
	c.Command = "bash"
	c.Args = []string{"-c", "trap 'echo one; exit' SIGUSR1; while true; do sleep 0.2; done"}
	c.ReloadSignal = syscall.SIGUSR1

	out := gatedio.NewByteBuffer()
	c.Stdout, c.StdErr = out, out

	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	// For some reason bash doesn't start immediately
	time.Sleep(fileWaitSleepDelay)

	if err := c.Restart(); err != nil {
		t.Fatal(err)
	}

	// Give time for the file to flush
	time.Sleep(fileWaitSleepDelay)

	expected := "one\n"
	if out.String() != expected {
		t.Errorf("expected %q to be %q", out.String(), expected)
	}
}

func TestRestart(t *testing.T) {
	t.Parallel()

	c := testProcess(t)
	c.Command = "bash"
	c.Args = []string{"-c", "sleep 2; echo abc"}
	c.ReloadSignal = nil
	out := gatedio.NewByteBuffer()
	c.Stdout, c.StdErr = out, out

	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	// For some reason bash doesn't start immediately
	time.Sleep(fileWaitSleepDelay)

	if err := c.Restart(); err != nil {
		t.Fatal(err)
	}

	// Give time for the file to flush
	time.Sleep(1 * time.Second)

	expected := "abc\n"
	assert.Equal(t, expected, out.String())
}

func TestReloadNoSignal(t *testing.T) {
	t.Parallel()

	c := testProcess(t)
	c.Command = "bash"
	c.Args = []string{"-c", "while true; do sleep 0.2; done"}
	c.KillTimeout = 10 * time.Millisecond
	c.ReloadSignal = nil

	out := gatedio.NewByteBuffer()
	c.Stdout, c.StdErr = out, out

	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	// For some reason bash doesn't start immediately
	time.Sleep(fileWaitSleepDelay)

	// Grab the original pid
	opid := c.exec.Process.Pid

	if err := c.Restart(); err != nil {
		t.Fatal(err)
	}

	// Give time for the file to flush
	time.Sleep(fileWaitSleepDelay)

	// Get the new pid
	npid := c.exec.Process.Pid

	// Stop the child now
	c.Stop()

	if opid == npid {
		t.Error("expected new process to restart")
	}
}

func TestProcess_Restart(t *testing.T) {
	t.Parallel()

	c := testProcess(t)
	c.ReloadSignal = syscall.SIGUSR1
	if err := c.Restart(); err != nil {
		t.Fatal(err)
	}
}

func TestProcess_GetPid(t *testing.T) {
	t.Parallel()

	c := testProcess(t)
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	pid := c.GetPID()
	if pid == 0 {
		t.Error("expected pid to not be 0")
	}
}

func TestProcess_ExitCh(t *testing.T) {
	t.Parallel()

	c := testProcess(t)
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	ch := c.ExitCh()
	if ch == nil {
		t.Error("expected ch to exist")
	}
}

func TestKill_signal(t *testing.T) {
	t.Parallel()

	c := testProcess(t)
	c.Command = "bash"
	c.Args = []string{"-c", "trap 'echo one; exit' SIGUSR1; while true; do sleep 0.2; done"}
	c.KillSignal = syscall.SIGUSR1

	out := gatedio.NewByteBuffer()
	c.Stdout, c.StdErr = out, out

	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	// For some reason bash doesn't start immediately
	time.Sleep(fileWaitSleepDelay)

	c.Kill()

	// Give time for the file to flush
	time.Sleep(fileWaitSleepDelay)

	expected := "one\n"
	if out.String() != expected {
		t.Errorf("expected %q to be %q", out.String(), expected)
	}
}

func TestKill_noSignal(t *testing.T) {
	t.Parallel()

	c := testProcess(t)
	c.Command = "bash"
	c.Args = []string{"-c", "while true; do sleep 0.2; done"}
	c.KillTimeout = 20 * time.Millisecond
	c.KillSignal = nil

	out := gatedio.NewByteBuffer()
	c.Stdout, c.StdErr = out, out

	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	// For some reason bash doesn't start immediately
	time.Sleep(fileWaitSleepDelay)

	c.Kill()

	// Give time for the file to flush
	time.Sleep(fileWaitSleepDelay)

	if c.exec != nil {
		t.Errorf("expected cmd to be nil")
	}
}

func TestKill_noProcess(t *testing.T) {
	t.Parallel()

	c := testProcess(t)
	c.KillSignal = syscall.SIGUSR1
	c.Kill()
}
