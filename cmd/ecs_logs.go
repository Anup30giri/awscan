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
	allContainers  bool
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
	cmd.Flags().BoolVar(&flags.allContainers, "all-containers", false, "Tail all container log streams on selected task")
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
	if len(targets) == 0 {
		return fmt.Errorf("selected task does not expose awslogs targets. Check task definition log driver and awslogs-group/awslogs-stream-prefix settings")
	}

	logGroups := map[string]bool{}
	var logGroup string
	streams := []string{}
	for _, target := range targets {
		if !flags.allContainers && target.ContainerName != container {
			continue
		}
		logGroup = target.LogGroup
		logGroups[target.LogGroup] = true
		streams = append(streams, target.LogStream)
	}
	if logGroup == "" || len(streams) == 0 {
		return fmt.Errorf("no awslogs target found for container %q on selected task", container)
	}
	if flags.allContainers && len(logGroups) > 1 {
		return fmt.Errorf("all-containers mode requires all selected containers to share same CloudWatch log group")
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
		if cluster == "" || service == "" {
			return "", "", "", "", errors.New("cluster and service must be provided in --non-interactive mode")
		}
		if task == "" {
			latest, err := provider.ResolveLatestTask(ctx, cluster, service)
			if err != nil {
				return "", "", "", "", err
			}
			task = latest.Arn
		}
		if !flags.allContainers && container == "" {
			return "", "", "", "", errors.New("container must be provided in --non-interactive mode unless --all-containers is used")
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

	if task == "" && cluster != "" && service != "" {
		if latest, resolveErr := provider.ResolveLatestTask(ctx, cluster, service); resolveErr == nil {
			task = latest.Arn
			state.Task = task
		}
	}

	steps := buildECSSelectionSteps(ctx, env, provider, runtime, cluster, service, task, container, !flags.allContainers)
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

func resolveECSClusterService(ctx context.Context, env *commandEnv, provider ecsprovider.Provider, runtime runtimeAdapter, cluster, service string, nonInteractive bool, title string) (string, string, error) {
	if nonInteractive {
		if cluster == "" || service == "" {
			return "", "", fmt.Errorf("cluster and service must be provided in --non-interactive mode")
		}
		return cluster, service, nil
	}
	state := tui.WorkflowState{
		Profile: runtime.ProfileName(),
		Region:  runtime.RegionName(),
		Account: runtime.AccountID(),
		Target:  "ecs",
		Cluster: cluster,
		Service: service,
	}
	steps := buildECSSelectionSteps(ctx, env, provider, runtime, cluster, service, "", "", false)
	output, err := tui.RunWorkflow(ctx, tui.WorkflowInput{
		Title: title,
		Steps: steps,
		State: state,
	})
	if err != nil {
		return "", "", err
	}
	return firstNonEmpty(output.State.Cluster, cluster), firstNonEmpty(output.State.Service, service), nil
}
