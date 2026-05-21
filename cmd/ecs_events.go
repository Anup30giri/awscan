package cmd

import (
	"context"
	"fmt"
	"time"

	ecsprovider "github.com/anupgiri/awscan/internal/providers/ecs"
	"github.com/spf13/cobra"
)

type ecsEventsFlags struct {
	cluster        string
	service        string
	nonInteractive bool
}

func newECSEventsCommand(env *commandEnv, root *rootFlags) *cobra.Command {
	flags := ecsEventsFlags{}
	cmd := &cobra.Command{
		Use:   "events",
		Short: "Show recent ECS service events",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runECSEvents(cmd.Context(), env, root, flags)
		},
	}
	cmd.Flags().StringVar(&flags.cluster, "cluster", "", "ECS cluster ARN or name")
	cmd.Flags().StringVar(&flags.service, "service", "", "ECS service ARN or name")
	cmd.Flags().BoolVar(&flags.nonInteractive, "non-interactive", false, "Fail instead of prompting for missing values")
	return cmd
}

func runECSEvents(ctx context.Context, env *commandEnv, root *rootFlags, flags ecsEventsFlags) error {
	runtime, err := resolveShellRuntime(ctx, env, root, flags.nonInteractive)
	if err != nil {
		return err
	}
	provider := ecsprovider.New(runtime.Config, runtime.Profile, runtime.Region, env.runner)
	adapter := runtimeAdapter{profile: runtime.Profile, region: runtime.Region, account: accountID(runtime)}
	cluster, service, err := resolveECSClusterService(ctx, env, provider, adapter, flags.cluster, flags.service, flags.nonInteractive, "awscan ecs events")
	if err != nil {
		return err
	}
	events, err := provider.ListServiceEvents(ctx, cluster, service)
	if err != nil {
		return err
	}
	for _, event := range events {
		fmt.Printf("%s %s\n", event.CreatedAt.Format(time.RFC3339), event.Message)
	}
	return nil
}
