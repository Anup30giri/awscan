//go:build unix

package exec

import (
	"os"
	osexec "os/exec"
	"syscall"
)

func prepareInteractiveCommand(cmd *osexec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	return nil
}

func interactiveSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT}
}

func forwardInteractiveSignal(process *os.Process, sig os.Signal) error {
	sysSig, ok := sig.(syscall.Signal)
	if !ok {
		return process.Signal(sig)
	}
	return syscall.Kill(-process.Pid, sysSig)
}

func terminateInteractiveProcess(process *os.Process) error {
	return syscall.Kill(-process.Pid, syscall.SIGTERM)
}
