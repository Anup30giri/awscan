package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	internalaws "github.com/anupgiri/awscan/internal/aws"
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
	profile := root.profile
	region := root.region

	if !flags.nonInteractive {
		selectedProfile, selectedRegion, err := resolveProfileAndRegionInteractively(ctx, env, profile, region)
		if err != nil {
			return err
		}
		profile = selectedProfile
		region = selectedRegion
	}

	runtime, err := env.resolver.Resolve(ctx, internalaws.ResolveOptions{
		Profile: profile,
		Region:  region,
	})
	if err != nil {
		return err
	}

	provider := ecsprovider.New(runtime.Config, runtime.Profile, runtime.Region, env.runner)

	cluster := flags.cluster
	service := flags.service
	task := flags.task
	container := flags.container
	command := flags.command
	if command == "" && container != "" {
		command = env.prefs.DefaultShells[container]
	}

	if flags.nonInteractive {
		if cluster == "" || service == "" || task == "" || container == "" {
			return errors.New("cluster, service, task, and container must be provided in --non-interactive mode")
		}
	} else {
		cluster, service, task, container, command, err = resolveECSSelectionsInteractively(ctx, env, provider, runtime, cluster, service, task, container, command)
		if err != nil {
			return err
		}
	}

	taskDetail, err := provider.DescribeTask(ctx, cluster, task)
	if err != nil {
		return err
	}

	readiness, err := provider.CheckExecReadiness(ctx, cluster, service, task)
	if err != nil {
		return err
	}
	if !readiness.ServiceExecEnabled || !readiness.TaskExecEnabled {
		return fmt.Errorf("this service/task does not have ECS Exec enabled. Run: aws ecs update-service --cluster %s --service %s --enable-execute-command --force-new-deployment", cluster, service)
	}

	if _, err := findContainer(taskDetail.Containers, container); err != nil {
		return err
	}

	if err := savePreferences(env, runtime.Profile, runtime.Region, cluster, service, container, command); err != nil {
		return err
	}

	return provider.ExecuteCommand(ctx, ecsprovider.ExecuteCommandInput{
		Profile:       runtime.Profile,
		Region:        runtime.Region,
		ClusterArn:    cluster,
		TaskArn:       task,
		ContainerName: container,
		Command:       command,
		Interactive:   true,
	})
}

func resolveProfileAndRegionInteractively(ctx context.Context, env *commandEnv, profile, region string) (string, string, error) {
	if profile != "" && region != "" {
		return profile, region, nil
	}

	profiles, err := internalaws.LoadProfiles(internalaws.DefaultSharedConfigPaths())
	if err != nil {
		return "", "", err
	}

	state := tui.WorkflowState{
		Profile: profile,
		Region:  region,
	}

	steps := []tui.Step{}
	if profile == "" {
		options := buildProfileOptions(profiles)
		if len(options) == 0 {
			return "", "", errors.New("no AWS profiles or environment credentials were found. Run `aws login`, `aws sso login`, or set environment credentials first")
		}
		steps = append(steps, screens.ProfileStep(options, env.prefs.DefaultProfile))
	}
	if region == "" {
		steps = append(steps, screens.RegionStep(buildRegionOptions(), env.prefs.DefaultRegion))
	}
	if len(steps) == 0 {
		return profile, region, nil
	}

	output, err := tui.RunWorkflow(ctx, tui.WorkflowInput{
		Title: "awscan ecs shell",
		Steps: steps,
		State: state,
	})
	if err != nil {
		return "", "", err
	}

	return firstNonEmpty(output.State.Profile, profile), firstNonEmpty(output.State.Region, region), nil
}

func resolveECSSelectionsInteractively(
	ctx context.Context,
	env *commandEnv,
	provider ecsprovider.Provider,
	runtime internalaws.Runtime,
	cluster, service, task, container, command string,
) (string, string, string, string, string, error) {
	state := tui.WorkflowState{
		Profile:   runtime.Profile,
		Region:    runtime.Region,
		Cluster:   cluster,
		Service:   service,
		Task:      task,
		Container: container,
		Command:   command,
	}

	steps := []tui.Step{}

	if cluster == "" {
		steps = append(steps, screens.ClusterStep(nil, env.prefs.Recent.ECS.Cluster))
		steps[len(steps)-1].Load = func(state tui.WorkflowState) ([]tui.Option, error) {
			clusters, err := provider.ListClusters(ctx)
			if err != nil {
				return nil, err
			}
			if len(clusters) == 0 {
				return nil, fmt.Errorf("no ECS clusters found in %s", runtime.Region)
			}
			options := make([]tui.Option, 0, len(clusters))
			for _, cluster := range clusters {
				options = append(options, tui.Option{
					Label:   cluster.Name,
					Details: cluster.Arn,
					Value:   cluster.Arn,
				})
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
				})
			}
			return options, nil
		}
	}

	if container == "" {
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
				})
			}
			return options, nil
		}
	}

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

func savePreferences(env *commandEnv, profile, region, cluster, service, container, command string) error {
	env.prefs.DefaultProfile = profile
	env.prefs.DefaultRegion = region
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

func buildProfileOptions(profiles []internalaws.Profile) []tui.Option {
	options := make([]tui.Option, 0, len(profiles)+1)
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != "" {
		options = append(options, tui.Option{
			Label:   "environment",
			Details: "Use AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY from the current shell",
			Value:   "",
		})
	}
	for _, profile := range profiles {
		details := fmt.Sprintf("type=%s region=%s", profile.Kind, firstNonEmpty(profile.Region, "-"))
		options = append(options, tui.Option{
			Label:   profile.Name,
			Details: details,
			Value:   profile.Name,
		})
	}
	return options
}

func buildRegionOptions() []tui.Option {
	regions := internalaws.KnownRegions()
	options := make([]tui.Option, 0, len(regions))
	for _, region := range regions {
		options = append(options, tui.Option{
			Label:   region,
			Details: "AWS region",
			Value:   region,
		})
	}
	return options
}

func buildCommandOptions() []tui.Option {
	return []tui.Option{
		{Label: "/bin/sh", Details: "Portable POSIX shell", Value: "/bin/sh"},
		{Label: "/bin/bash", Details: "Bash shell if present in the container", Value: "/bin/bash"},
	}
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
