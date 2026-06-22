//go:build unix

package exec

import (
	"os"
	osexec "os/exec"
	"syscall"
	"testing"
	"time"
)

func TestForwardInteractiveSignal(t *testing.T) {
	t.Parallel()

	cmd := osexec.Command("sh", "-c", "trap 'exit 130' INT; while :; do sleep 1; done")
	if err := prepareInteractiveCommand(cmd); err != nil {
		t.Fatalf("prepareInteractiveCommand() error = %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("cmd.Start() error = %v", err)
	}
	t.Cleanup(func() {
		_ = terminateInteractiveProcess(cmd.Process)
		_, _ = cmd.Process.Wait()
	})

	time.Sleep(100 * time.Millisecond)
	if err := forwardInteractiveSignal(cmd.Process, os.Interrupt); err != nil {
		t.Fatalf("forwardInteractiveSignal() error = %v", err)
	}

	err := cmd.Wait()
	if err == nil {
		t.Fatal("cmd.Wait() succeeded, want signaled exit")
	}

	exitErr, ok := err.(*osexec.ExitError)
	if !ok {
		t.Fatalf("err type = %T, want *exec.ExitError", err)
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		t.Fatalf("exit status type = %T", exitErr.Sys())
	}
	if status.ExitStatus() != 130 {
		t.Fatalf("exit status = %d, want 130", status.ExitStatus())
	}
}
