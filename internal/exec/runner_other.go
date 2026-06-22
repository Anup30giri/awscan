//go:build !unix

package exec

import (
	"os"
	osexec "os/exec"
)

func prepareInteractiveCommand(cmd *osexec.Cmd) error {
	return nil
}

func interactiveSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}

func forwardInteractiveSignal(process *os.Process, sig os.Signal) error {
	return process.Signal(sig)
}

func terminateInteractiveProcess(process *os.Process) error {
	return process.Kill()
}
