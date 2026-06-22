package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	ecsprovider "github.com/anupgiri/awscan/internal/providers/ecs"
	"github.com/anupgiri/awscan/internal/tui"
	"github.com/anupgiri/awscan/internal/tui/screens"
	"github.com/spf13/cobra"
)

type ecsShellFlags struct {
	cluster        string
	service        string
	task           string
	container      string
	command        string
	nonInteractive bool
}

func newECSShellCommand(env *commandEnv, root *rootFlags) *cobra.Command {
	flags := ecsShellFlags{}

	cmd := &cobra.Command{
		Use:   "shell",
		Short: "Open a shell into a running ECS container using ECS Exec",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runECSShell(cmd.Context(), env, root, flags)
		},
	}

	cmd.Flags().StringVar(&flags.cluster, "cluster", "", "ECS cluster ARN or name")
	cmd.Flags().StringVar(&flags.service, "service", "", "ECS service ARN or name")
	cmd.Flags().StringVar(&flags.task, "task", "", "ECS task ARN or short ID")
	cmd.Flags().StringVar(&flags.container, "container", "", "Container name")
	cmd.Flags().StringVar(&flags.command, "command", "", "Shell command to execute, for example /bin/sh or /bin/bash")
	cmd.Flags().BoolVar(&flags.nonInteractive, "non-interactive", false, "Fail instead of prompting for missing values")

	return cmd
}

func runECSShell(ctx context.Context, env *commandEnv, root *rootFlags, flags ecsShellFlags) error {
	runtime, err := resolveShellRuntime(ctx, env, root, flags.nonInteractive)
	if err != nil {
		return err
	}
	adapter := runtimeAdapter{profile: runtime.Profile, region: runtime.Region, account: accountID(runtime)}

	provider := ecsprovider.New(runtime.Config, runtime.Profile, runtime.Region, env.runner)

	req := ECSShellRequest{
		Profile:   runtime.Profile,
		Region:    runtime.Region,
		Cluster:   flags.cluster,
		Service:   flags.service,
		Task:      flags.task,
		Container: flags.container,
		Command:   flags.command,
	}
	command := req.Command
	container := req.Container
	if command == "" && container != "" {
		command = env.prefs.DefaultShells[container]
	}
	req.Command = command

	if flags.nonInteractive {
		if req.Cluster == "" || req.Service == "" || req.Container == "" {
			return errors.New("cluster, service, and container must be provided in --non-interactive mode")
		}
	} else {
		req.Cluster, req.Service, req.Task, req.Container, req.Command, err = resolveECSSelectionsInteractively(ctx, env, provider, adapter, req.Cluster, req.Service, req.Task, req.Container, req.Command)
		if err != nil {
			return err
		}
	}

	return executeECSShellRequest(ctx, env, runtime, provider, req)
}

type RuntimeLike interface {
	ProfileName() string
	RegionName() string
	AccountID() string
}

type runtimeAdapter struct {
	profile string
	region  string
	account string
}

func (r runtimeAdapter) ProfileName() string { return r.profile }
func (r runtimeAdapter) RegionName() string  { return r.region }
func (r runtimeAdapter) AccountID() string   { return r.account }

func resolveECSSelectionsInteractively(ctx context.Context, env *commandEnv, provider ecsprovider.Provider, runtime runtimeAdapter, cluster, service, task, container, command string) (string, string, string, string, string, error) {
	state := tui.WorkflowState{
		Profile:   runtime.ProfileName(),
		Region:    runtime.RegionName(),
		Account:   runtime.AccountID(),
		Target:    "ecs",
		Cluster:   cluster,
		Service:   service,
		Task:      task,
		Container: container,
		Command:   command,
	}

	steps := buildECSSelectionSteps(ctx, env, provider, runtime, cluster, service, task, container, true)

	if flagsCommandMissing(command) {
		steps = append(steps, screens.CommandStep(buildCommandOptions(), command))
	}

	if len(steps) == 0 {
		return cluster, service, task, container, command, nil
	}

	output, err := tui.RunWorkflow(ctx, tui.WorkflowInput{
		Title: "awscan ecs shell",
		Steps: steps,
		State: state,
	})
	if err != nil {
		return "", "", "", "", "", err
	}

	finalState := output.State
	selectedCommand := firstNonEmpty(finalState.Command, command)
	if selectedCommand == "" {
		selectedCommand = env.prefs.DefaultShells[firstNonEmpty(finalState.Container, container)]
	}
	if selectedCommand == "" {
		selectedCommand = "/bin/sh"
	}

	return firstNonEmpty(finalState.Cluster, cluster),
		firstNonEmpty(finalState.Service, service),
		firstNonEmpty(finalState.Task, task),
		firstNonEmpty(finalState.Container, container),
		selectedCommand,
		nil
}

func buildECSSelectionSteps(ctx context.Context, env *commandEnv, provider ecsprovider.Provider, runtime runtimeAdapter, cluster, service, task, container string, includeContainer bool) []tui.Step {
	steps := []tui.Step{}
	if cluster == "" {
		steps = append(steps, screens.ClusterStep(nil, env.prefs.Recent.ECS.Cluster))
		steps[len(steps)-1].Load = func(state tui.WorkflowState) ([]tui.Option, error) {
			clusters, err := provider.ListClusters(ctx)
			if err != nil {
				return nil, err
			}
			if len(clusters) == 0 {
				return nil, fmt.Errorf("no ECS clusters found in %s", runtime.RegionName())
			}
			options := make([]tui.Option, 0, len(clusters))
			for _, cluster := range clusters {
				options = append(options, tui.Option{Label: cluster.Name, Details: cluster.Arn, Value: cluster.Arn})
			}
			return options, nil
		}
	}
	if service == "" {
		steps = append(steps, screens.ServiceStep(nil, env.prefs.Recent.ECS.Service))
		steps[len(steps)-1].Load = func(state tui.WorkflowState) ([]tui.Option, error) {
			clusterValue := firstNonEmpty(state.Cluster, cluster)
			services, err := provider.ListServices(ctx, clusterValue)
			if err != nil {
				return nil, err
			}
			if len(services) == 0 {
				return nil, fmt.Errorf("no ECS services found in cluster %s", clusterValue)
			}
			options := make([]tui.Option, 0, len(services))
			for _, service := range services {
				options = append(options, tui.Option{
					Label: service.Name,
					Details: fmt.Sprintf("desired=%d running=%d pending=%d exec=%t",
						service.DesiredCount, service.RunningCount, service.PendingCount, service.ExecEnabled),
					Value: service.Arn,
					Meta: map[string]string{
						"desired": fmt.Sprintf("%d", service.DesiredCount),
						"running": fmt.Sprintf("%d", service.RunningCount),
						"pending": fmt.Sprintf("%d", service.PendingCount),
						"exec":    fmt.Sprintf("%t", service.ExecEnabled),
					},
				})
			}
			return options, nil
		}
	}
	if task == "" {
		steps = append(steps, screens.TaskStep(nil, ""))
		steps[len(steps)-1].Load = func(state tui.WorkflowState) ([]tui.Option, error) {
			clusterValue := firstNonEmpty(state.Cluster, cluster)
			serviceValue := firstNonEmpty(state.Service, service)
			tasks, err := provider.ListTasks(ctx, clusterValue, serviceValue)
			if err != nil {
				return nil, err
			}
			if len(tasks) == 0 {
				return nil, fmt.Errorf("no running tasks found for this service")
			}
			options := make([]tui.Option, 0, len(tasks))
			for _, task := range tasks {
				startedAt := "unknown"
				if !task.StartedAt.IsZero() {
					startedAt = task.StartedAt.Format(time.RFC3339)
				}
				options = append(options, tui.Option{
					Label: task.ShortID,
					Details: fmt.Sprintf("%s | desired=%s | launch=%s | started=%s",
						task.LastStatus, task.DesiredStatus, task.LaunchType, startedAt),
					Value: task.Arn,
					Meta: map[string]string{
						"status":  task.LastStatus,
						"desired": task.DesiredStatus,
						"launch":  task.LaunchType,
						"task":    task.ShortID,
					},
				})
			}
			return options, nil
		}
	}
	if includeContainer && container == "" {
		steps = append(steps, screens.ContainerStep(nil, env.prefs.Recent.ECS.Container))
		steps[len(steps)-1].Load = func(state tui.WorkflowState) ([]tui.Option, error) {
			clusterValue := firstNonEmpty(state.Cluster, cluster)
			taskValue := firstNonEmpty(state.Task, task)
			taskDetail, err := provider.DescribeTask(ctx, clusterValue, taskValue)
			if err != nil {
				return nil, err
			}
			if len(taskDetail.Containers) == 0 {
				return nil, fmt.Errorf("no containers found for selected task")
			}
			options := make([]tui.Option, 0, len(taskDetail.Containers))
			for _, container := range taskDetail.Containers {
				options = append(options, tui.Option{
					Label:   container.Name,
					Details: fmt.Sprintf("%s | runtime=%s", container.LastStatus, container.RuntimeID),
					Value:   container.Name,
					Meta: map[string]string{
						"status":  container.LastStatus,
						"runtime": container.RuntimeID,
					},
				})
			}
			return options, nil
		}
	}
	return steps
}

func saveECSPreferences(env *commandEnv, profile, region, cluster, service, container, command string) error {
	saveGlobalPreferences(env, profile, region)
	env.prefs.Recent.ECS.Cluster = cluster
	env.prefs.Recent.ECS.Service = service
	env.prefs.Recent.ECS.Container = container
	if container != "" && command != "" {
		if env.prefs.DefaultShells == nil {
			env.prefs.DefaultShells = map[string]string{}
		}
		env.prefs.DefaultShells[container] = command
	}
	return env.app.Config.Save(env.prefs)
}

func findContainer(containers []ecsprovider.Container, name string) (*ecsprovider.Container, error) {
	for i := range containers {
		if containers[i].Name == name {
			return &containers[i], nil
		}
	}
	return nil, fmt.Errorf("the selected container %q was not found on this task", name)
}

func flagsCommandMissing(command string) bool {
	return strings.TrimSpace(command) == ""
}

func executeECSShellWithFallback(ctx context.Context, provider ecsprovider.Provider, runtime runtimeAdapter, cluster, task, container, command string) error {
	candidates := []string{command}
	if command == "/bin/bash" {
		candidates = append(candidates, "/bin/sh")
	}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if seen[candidate] || strings.TrimSpace(candidate) == "" {
			continue
		}
		seen[candidate] = true
		err := provider.ExecuteCommand(ctx, ecsprovider.ExecuteCommandInput{
			Profile:       runtime.ProfileName(),
			Region:        runtime.RegionName(),
			ClusterArn:    cluster,
			TaskArn:       task,
			ContainerName: container,
			Command:       candidate,
			Interactive:   true,
		})
		if err == nil {
			return nil
		}
		if candidate == "/bin/bash" && strings.Contains(strings.ToLower(err.Error()), "not found") {
			continue
		}
		return err
	}
	return fmt.Errorf("no usable shell command found for container %q", container)
}
