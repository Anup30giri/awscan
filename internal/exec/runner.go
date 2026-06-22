package exec

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"os/signal"
)

type Runner interface {
	LookPath(name string) (string, error)
	RunInteractive(ctx context.Context, name string, args []string, env []string) error
}

type CommandRunner struct{}

func NewRunner() *CommandRunner {
	return &CommandRunner{}
}

func (r *CommandRunner) LookPath(name string) (string, error) {
	return osexec.LookPath(name)
}

func (r *CommandRunner) RunInteractive(ctx context.Context, name string, args []string, env []string) error {
	cmd := osexec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	if err := prepareInteractiveCommand(cmd); err != nil {
		return fmt.Errorf("prepare %s: %w", name, err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", name, err)
	}

	signalCh := make(chan os.Signal, 4)
	signal.Notify(signalCh, interactiveSignals()...)
	defer signal.Stop(signalCh)

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	for {
		select {
		case err := <-waitCh:
			if err != nil {
				return fmt.Errorf("run %s: %w", name, err)
			}
			return nil
		case sig := <-signalCh:
			if err := forwardInteractiveSignal(cmd.Process, sig); err != nil {
				return fmt.Errorf("forward signal to %s: %w", name, err)
			}
		case <-ctx.Done():
			if err := terminateInteractiveProcess(cmd.Process); err != nil {
				return fmt.Errorf("stop %s: %w", name, err)
			}
			err := <-waitCh
			if err != nil {
				return fmt.Errorf("run %s: %w", name, err)
			}
			return ctx.Err()
		}
	}
}

func EnsureBinary(r Runner, name string) error {
	if _, err := r.LookPath(name); err != nil {
		return fmt.Errorf("%s is missing from PATH", name)
	}
	return nil
}
