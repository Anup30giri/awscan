package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	ecsprovider "github.com/anupgiri/awscan/internal/providers/ecs"
	"github.com/anupgiri/awscan/internal/tui"
	"github.com/spf13/cobra"
)

type ecsInspectFlags struct {
	cluster        string
	service        string
	task           string
	container      string
	nonInteractive bool
}

func newECSInspectCommand(env *commandEnv, root *rootFlags) *cobra.Command {
	flags := ecsInspectFlags{}
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect ECS service, task, and container details",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runECSInspect(cmd.Context(), env, root, flags)
		},
	}
	cmd.Flags().StringVar(&flags.cluster, "cluster", "", "ECS cluster ARN or name")
	cmd.Flags().StringVar(&flags.service, "service", "", "ECS service ARN or name")
	cmd.Flags().StringVar(&flags.task, "task", "", "ECS task ARN or short ID")
	cmd.Flags().StringVar(&flags.container, "container", "", "Container name")
	cmd.Flags().BoolVar(&flags.nonInteractive, "non-interactive", false, "Fail instead of prompting for missing values")
	return cmd
}

func runECSInspect(ctx context.Context, env *commandEnv, root *rootFlags, flags ecsInspectFlags) error {
	runtime, err := resolveShellRuntime(ctx, env, root, flags.nonInteractive)
	if err != nil {
		return err
	}
	adapter := runtimeAdapter{profile: runtime.Profile, region: runtime.Region, account: accountID(runtime)}
	provider := ecsprovider.New(runtime.Config, runtime.Profile, runtime.Region, env.runner)

	cluster, service, task, container, err := resolveECSInspectSelections(ctx, env, provider, adapter, flags)
	if err != nil {
		return err
	}

	detail, err := provider.DescribeService(ctx, cluster, service)
	if err != nil {
		return err
	}

	var taskDetail *ecsprovider.TaskDetail
	if strings.TrimSpace(task) != "" {
		taskDetail, err = provider.DescribeTask(ctx, cluster, task)
		if err != nil {
			return err
		}
	}

	saveGlobalPreferences(env, runtime.Profile, runtime.Region)
	if err := saveECSPreferences(env, runtime.Profile, runtime.Region, cluster, service, firstNonEmpty(container, env.prefs.Recent.ECS.Container), env.prefs.DefaultShells[firstNonEmpty(container, env.prefs.Recent.ECS.Container)]); err != nil {
		return err
	}

	fmt.Print(renderECSInspect(detail, taskDetail, container))
	return nil
}

func resolveECSInspectSelections(ctx context.Context, env *commandEnv, provider ecsprovider.Provider, runtime runtimeAdapter, flags ecsInspectFlags) (string, string, string, string, error) {
	cluster, service, task, container := flags.cluster, flags.service, flags.task, flags.container
	if flags.nonInteractive {
		if cluster == "" || service == "" {
			return "", "", "", "", fmt.Errorf("cluster and service must be provided in --non-interactive mode")
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
	steps := buildECSSelectionSteps(ctx, env, provider, runtime, cluster, service, task, container, task != "" || container != "")
	output, err := tui.RunWorkflow(ctx, tui.WorkflowInput{
		Title: "awscan ecs inspect",
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

func renderECSInspect(detail *ecsprovider.ServiceDetail, taskDetail *ecsprovider.TaskDetail, selectedContainer string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Service: %s\nARN: %s\nTask Definition: %s\nDesired: %d Running: %d Pending: %d\nExec Enabled: %t\n",
		detail.Service.Name, detail.Service.Arn, detail.TaskDefinitionArn, detail.Service.DesiredCount, detail.Service.RunningCount, detail.Service.PendingCount, detail.Service.ExecEnabled)
	if len(detail.Deployments) > 0 {
		b.WriteString("\nDeployments:\n")
		for _, deployment := range detail.Deployments {
			fmt.Fprintf(&b, "- %s status=%s rollout=%s desired=%d running=%d pending=%d taskDef=%s\n",
				deployment.ID, deployment.Status, deployment.RolloutState, deployment.DesiredCount, deployment.RunningCount, deployment.PendingCount, deployment.TaskDefArn)
		}
	}
	if taskDetail != nil {
		fmt.Fprintf(&b, "\nTask: %s status=%s desired=%s launch=%s started=%s\n",
			taskDetail.Task.ShortID, taskDetail.Task.LastStatus, taskDetail.Task.DesiredStatus, taskDetail.Task.LaunchType, taskDetail.Task.StartedAt.Format(time.RFC3339))
		if len(taskDetail.Containers) > 0 {
			b.WriteString("Containers:\n")
			for _, container := range taskDetail.Containers {
				if selectedContainer != "" && container.Name != selectedContainer {
					continue
				}
				fmt.Fprintf(&b, "- %s status=%s runtime=%s", container.Name, container.LastStatus, container.RuntimeID)
				if container.ExitCode != nil {
					fmt.Fprintf(&b, " exit=%d", *container.ExitCode)
				}
				if container.Reason != "" {
					fmt.Fprintf(&b, " reason=%s", container.Reason)
				}
				b.WriteString("\n")
			}
		}
	}
	return b.String()
}
