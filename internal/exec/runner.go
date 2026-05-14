package exec

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
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
	cmd := osexec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s: %w", name, err)
	}
	return nil
}

func EnsureBinary(r Runner, name string) error {
	if _, err := r.LookPath(name); err != nil {
		return fmt.Errorf("%s is missing from PATH", name)
	}
	return nil
}
