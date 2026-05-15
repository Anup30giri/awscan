package cmd

import (
	"context"
	"errors"
	"fmt"

	ecsprovider "github.com/anupgiri/awscan/internal/providers/ecs"
	"github.com/anupgiri/awscan/internal/tui"
	"github.com/spf13/cobra"
)

type ecsLogsFlags struct {
	cluster        string
	service        string
	task           string
	container      string
	follow         bool
	since          string
	nonInteractive bool
}

func newECSLogsCommand(env *commandEnv, root *rootFlags) *cobra.Command {
	flags := ecsLogsFlags{}

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Tail CloudWatch logs for ECS container",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runECSLogs(cmd.Context(), env, root, flags)
		},
	}

	cmd.Flags().StringVar(&flags.cluster, "cluster", "", "ECS cluster ARN or name")
	cmd.Flags().StringVar(&flags.service, "service", "", "ECS service ARN or name")
	cmd.Flags().StringVar(&flags.task, "task", "", "ECS task ARN or short ID")
	cmd.Flags().StringVar(&flags.container, "container", "", "Container name")
	cmd.Flags().BoolVar(&flags.follow, "follow", true, "Follow log output")
	cmd.Flags().StringVar(&flags.since, "since", "10m", "Show logs since duration, for example 10m or 1h")
	cmd.Flags().BoolVar(&flags.nonInteractive, "non-interactive", false, "Fail instead of prompting for missing values")
	return cmd
}

func runECSLogs(ctx context.Context, env *commandEnv, root *rootFlags, flags ecsLogsFlags) error {
	runtime, err := resolveShellRuntime(ctx, env, root, flags.nonInteractive)
	if err != nil {
		return err
	}

	provider := ecsprovider.New(runtime.Config, runtime.Profile, runtime.Region, env.runner)
	adapter := runtimeAdapter{profile: runtime.Profile, region: runtime.Region, account: accountID(runtime)}
	cluster, _, task, container, err := resolveECSLogSelections(ctx, env, provider, adapter, flags)
	if err != nil {
		return err
	}

	targets, err := provider.ResolveLogTargets(ctx, cluster, task)
	if err != nil {
		return err
	}

	var logGroup string
	streams := []string{}
	for _, target := range targets {
		if target.ContainerName != container {
			continue
		}
		logGroup = target.LogGroup
		streams = append(streams, target.LogStream)
	}
	if logGroup == "" || len(streams) == 0 {
		return fmt.Errorf("no awslogs target found for container %q on selected task", container)
	}

	saveGlobalPreferences(env, runtime.Profile, runtime.Region)
	return provider.TailLogs(ctx, ecsprovider.TailLogsInput{
		Profile:    runtime.Profile,
		Region:     runtime.Region,
		LogGroup:   logGroup,
		LogStreams: streams,
		Follow:     flags.follow,
		Since:      flags.since,
	})
}

func resolveECSLogSelections(ctx context.Context, env *commandEnv, provider ecsprovider.Provider, runtime runtimeAdapter, flags ecsLogsFlags) (string, string, string, string, error) {
	cluster := flags.cluster
	service := flags.service
	task := flags.task
	container := flags.container

	if flags.nonInteractive {
		if cluster == "" || service == "" || task == "" || container == "" {
			return "", "", "", "", errors.New("cluster, service, task, and container must be provided in --non-interactive mode")
		}
		return cluster, service, task, container, nil
	}

	state := tui.WorkflowState{
		Profile:   runtime.ProfileName(),
		Region:    runtime.RegionName(),
		Account:   runtime.AccountID(),
		Target:    "ecs",
		Cluster:   cluster,
		Service:   service,
		Task:      task,
		Container: container,
	}

	steps := buildECSSelectionSteps(ctx, env, provider, runtime, cluster, service, task, container, false)
	output, err := tui.RunWorkflow(ctx, tui.WorkflowInput{
		Title: "awscan ecs logs",
		Steps: steps,
		State: state,
	})
	if err != nil {
		return "", "", "", "", err
	}

	return firstNonEmpty(output.State.Cluster, cluster),
		firstNonEmpty(output.State.Service, service),
		firstNonEmpty(output.State.Task, task),
		firstNonEmpty(output.State.Container, container),
		nil
}
