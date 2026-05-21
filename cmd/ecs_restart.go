package cmd

import (
	"context"
	"fmt"
	"os"

	ecsprovider "github.com/anupgiri/awscan/internal/providers/ecs"
	"github.com/spf13/cobra"
)

type ecsRestartFlags struct {
	cluster        string
	service        string
	yes            bool
	nonInteractive bool
}

func newECSRestartCommand(env *commandEnv, root *rootFlags) *cobra.Command {
	flags := ecsRestartFlags{}
	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Trigger ECS service force-new-deployment restart",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runECSRestart(cmd.Context(), env, root, flags)
		},
	}
	cmd.Flags().StringVar(&flags.cluster, "cluster", "", "ECS cluster ARN or name")
	cmd.Flags().StringVar(&flags.service, "service", "", "ECS service ARN or name")
	cmd.Flags().BoolVar(&flags.yes, "yes", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&flags.nonInteractive, "non-interactive", false, "Fail instead of prompting for missing values")
	return cmd
}

func runECSRestart(ctx context.Context, env *commandEnv, root *rootFlags, flags ecsRestartFlags) error {
	runtime, err := resolveShellRuntime(ctx, env, root, flags.nonInteractive)
	if err != nil {
		return err
	}
	provider := ecsprovider.New(runtime.Config, runtime.Profile, runtime.Region, env.runner)
	adapter := runtimeAdapter{profile: runtime.Profile, region: runtime.Region, account: accountID(runtime)}
	cluster, service, err := resolveECSClusterService(ctx, env, provider, adapter, flags.cluster, flags.service, flags.nonInteractive, "awscan ecs restart")
	if err != nil {
		return err
	}
	if !flags.yes {
		if flags.nonInteractive {
			return fmt.Errorf("--yes is required in --non-interactive mode")
		}
		confirmed, err := confirmAction(os.Stdin, os.Stdout, fmt.Sprintf("Force new deployment for service %s?", service))
		if err != nil {
			return err
		}
		if !confirmed {
			return fmt.Errorf("restart cancelled")
		}
	}
	if err := provider.ForceNewDeployment(ctx, cluster, service); err != nil {
		return err
	}
	fmt.Printf("Triggered force-new-deployment for %s\n", service)
	return nil
}
