package cmd

import (
	"context"
	"fmt"
	"os"

	internalaws "github.com/anupgiri/awscan/internal/aws"
	appconfig "github.com/anupgiri/awscan/internal/config"
	"github.com/anupgiri/awscan/internal/diagnostics"
	ec2provider "github.com/anupgiri/awscan/internal/providers/ec2"
	ecsprovider "github.com/anupgiri/awscan/internal/providers/ecs"
	"github.com/anupgiri/awscan/internal/tui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type savedAddFlags struct {
	kind          string
	cluster       string
	service       string
	task          string
	container     string
	command       string
	since         string
	follow        bool
	allContainers bool
	instance      string
	localPort     int
	remoteHost    string
	remotePort    int
	overwrite     bool
}

func newSavedCommand(env *commandEnv, root *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "saved",
		Short: "Manage saved workflows for repetitive AWS actions",
	}

	cmd.AddCommand(newSavedListCommand(env))
	cmd.AddCommand(newSavedInspectCommand(env))
	cmd.AddCommand(newSavedRemoveCommand(env))
	cmd.AddCommand(newSavedRunCommand(env, root))
	cmd.AddCommand(newSavedAddCommand(env, root))
	return cmd
}

func newSavedListCommand(env *commandEnv) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List saved workflows",
		RunE: func(cmd *cobra.Command, args []string) error {
			names := env.prefs.SavedNames()
			if len(names) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No saved workflows configured.")
				return nil
			}
			for _, name := range names {
				entry := env.prefs.Saved[name]
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", name, entry.Kind, savedWorkflowSummary(entry))
			}
			return nil
		},
	}
}

func newSavedInspectCommand(env *commandEnv) *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <name>",
		Short: "Show a saved workflow definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			entry, err := env.prefs.ValidateSavedWorkflow(args[0])
			if err != nil {
				return err
			}
			output := map[string]appconfig.SavedWorkflow{
				args[0]: entry,
			}
			data, err := yaml.Marshal(output)
			if err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write(data)
			return err
		},
	}
}

func newSavedRemoveCommand(env *commandEnv) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a saved workflow",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if _, err := env.prefs.ValidateSavedWorkflow(name); err != nil {
				return err
			}
			if !yes {
				ok, err := confirmAction(os.Stdin, cmd.OutOrStdout(), fmt.Sprintf("Remove saved workflow %q?", name))
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("remove cancelled")
				}
			}
			delete(env.prefs.Saved, name)
			if err := env.app.Config.Save(env.prefs); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed saved workflow %q.\n", name)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt")
	return cmd
}

func newSavedRunCommand(env *commandEnv, root *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "run <name>",
		Short: "Run a saved workflow",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSavedWorkflowByName(cmd.Context(), env, root, args[0])
		},
	}
}

func newSavedAddCommand(env *commandEnv, root *rootFlags) *cobra.Command {
	flags := savedAddFlags{
		follow: true,
		since:  "10m",
	}

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add or update a saved workflow",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := appconfig.ValidateSavedWorkflowName(name); err != nil {
				return err
			}

			entry := buildSavedWorkflowFromFlags(env, root, flags)
			if err := entry.Validate(); err != nil {
				return err
			}

			if _, exists := env.prefs.Saved[name]; exists && !flags.overwrite {
				ok, err := confirmAction(os.Stdin, cmd.OutOrStdout(), fmt.Sprintf("Saved workflow %q already exists. Overwrite?", name))
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("save cancelled")
				}
			}

			env.prefs.Saved[name] = entry
			if err := env.app.Config.Save(env.prefs); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved workflow %q (%s).\n", name, entry.Kind)
			return nil
		},
	}

	cmd.Flags().StringVar(&flags.kind, "kind", "", "Workflow kind: ecs-shell, ecs-logs, ec2-shell, ec2-port-forward")
	cmd.Flags().StringVar(&flags.cluster, "cluster", "", "ECS cluster ARN or name")
	cmd.Flags().StringVar(&flags.service, "service", "", "ECS service ARN or name")
	cmd.Flags().StringVar(&flags.task, "task", "", "ECS task ARN or short ID")
	cmd.Flags().StringVar(&flags.container, "container", "", "Container name")
	cmd.Flags().StringVar(&flags.command, "command", "", "Shell command to execute")
	cmd.Flags().StringVar(&flags.since, "since", "10m", "Logs window, for example 10m or 1h")
	cmd.Flags().BoolVar(&flags.follow, "follow", true, "Follow log output")
	cmd.Flags().BoolVar(&flags.allContainers, "all-containers", false, "Tail all ECS container log streams on the task")
	cmd.Flags().StringVar(&flags.instance, "instance", "", "EC2 instance ID or Name tag")
	cmd.Flags().IntVar(&flags.localPort, "local-port", 0, "Local port for EC2 port forwarding")
	cmd.Flags().StringVar(&flags.remoteHost, "remote-host", "", "Optional remote host behind EC2 instance")
	cmd.Flags().IntVar(&flags.remotePort, "remote-port", 0, "Remote port for EC2 port forwarding")
	cmd.Flags().BoolVar(&flags.overwrite, "overwrite", false, "Overwrite existing saved workflow without prompting")
	_ = cmd.MarkFlagRequired("kind")
	return cmd
}

func buildSavedWorkflowFromFlags(env *commandEnv, root *rootFlags, flags savedAddFlags) appconfig.SavedWorkflow {
	entry := appconfig.SavedWorkflow{
		Kind:          appconfig.SavedWorkflowKind(flags.kind),
		Profile:       firstNonEmpty(root.profile, env.prefs.DefaultProfile),
		Region:        firstNonEmpty(root.region, env.prefs.DefaultRegion),
		Cluster:       firstNonEmpty(flags.cluster, env.prefs.Recent.ECS.Cluster),
		Service:       firstNonEmpty(flags.service, env.prefs.Recent.ECS.Service),
		Task:          flags.task,
		Container:     firstNonEmpty(flags.container, env.prefs.Recent.ECS.Container),
		Command:       flags.command,
		Since:         flags.since,
		Follow:        flags.follow,
		AllContainers: flags.allContainers,
		Instance:      firstNonEmpty(flags.instance, env.prefs.Recent.EC2.InstanceID),
		LocalPort:     flags.localPort,
		RemoteHost:    firstNonEmpty(flags.remoteHost, env.prefs.Recent.EC2.RemoteHost),
		RemotePort:    flags.remotePort,
	}
	if entry.Command == "" {
		switch entry.Kind {
		case appconfig.SavedWorkflowKindECSShell:
			entry.Command = firstNonEmpty(env.prefs.DefaultShells[entry.Container], "/bin/sh")
		case appconfig.SavedWorkflowKindEC2Shell:
			entry.Command = firstNonEmpty(env.prefs.DefaultShells[entry.Instance], "/bin/sh")
		}
	}
	if entry.LocalPort == 0 {
		entry.LocalPort = env.prefs.Recent.EC2.LocalPort
	}
	if entry.RemotePort == 0 {
		entry.RemotePort = env.prefs.Recent.EC2.RemotePort
	}
	return entry
}

func runSavedWorkflowByName(ctx context.Context, env *commandEnv, root *rootFlags, name string) error {
	entry, err := env.prefs.ValidateSavedWorkflow(name)
	if err != nil {
		return err
	}
	entry.Profile = firstNonEmpty(root.profile, entry.Profile)
	entry.Region = firstNonEmpty(root.region, entry.Region)
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("saved workflow %q is invalid after overrides: %w", name, err)
	}
	return executeSavedWorkflow(ctx, env, name, entry)
}

func executeSavedWorkflow(ctx context.Context, env *commandEnv, name string, entry appconfig.SavedWorkflow) error {
	runtime, _, _, err := resolveRuntime(ctx, env, entry.Profile, entry.Region, false)
	if err != nil {
		return err
	}

	switch entry.Kind {
	case appconfig.SavedWorkflowKindECSShell:
		provider := savedECSProviderFactory(runtime, env)
		return executeECSShellRequest(ctx, env, runtime, provider, ECSShellRequest{
			Profile:   runtime.Profile,
			Region:    runtime.Region,
			Cluster:   entry.Cluster,
			Service:   entry.Service,
			Task:      entry.Task,
			Container: entry.Container,
			Command:   entry.Command,
		})
	case appconfig.SavedWorkflowKindECSLogs:
		provider := savedECSProviderFactory(runtime, env)
		return executeECSLogsRequest(ctx, env, runtime, provider, ECSLogsRequest{
			Profile:       runtime.Profile,
			Region:        runtime.Region,
			Cluster:       entry.Cluster,
			Service:       entry.Service,
			Task:          entry.Task,
			Container:     entry.Container,
			AllContainers: entry.AllContainers,
			Follow:        entry.Follow,
			Since:         firstNonEmpty(entry.Since, "10m"),
		})
	case appconfig.SavedWorkflowKindEC2Shell:
		provider := savedEC2ProviderFactory(runtime, env)
		return executeEC2ShellRequest(ctx, env, runtime, provider, EC2ShellRequest{
			Profile:  runtime.Profile,
			Region:   runtime.Region,
			Instance: entry.Instance,
			Command:  entry.Command,
		})
	case appconfig.SavedWorkflowKindEC2PortForward:
		provider := savedEC2ProviderFactory(runtime, env)
		return executeEC2PortForwardRequest(ctx, env, runtime, provider, EC2PortForwardRequest{
			Profile:    runtime.Profile,
			Region:     runtime.Region,
			Instance:   entry.Instance,
			RemoteHost: entry.RemoteHost,
			LocalPort:  entry.LocalPort,
			RemotePort: entry.RemotePort,
		})
	default:
		return fmt.Errorf("saved workflow %q uses unsupported kind %q", name, entry.Kind)
	}
}

func applySavedWorkflowToDoctorFlags(flags *diagnostics.Options, entry appconfig.SavedWorkflow) {
	switch entry.Kind {
	case appconfig.SavedWorkflowKindECSShell:
		flags.Target = firstNonEmpty(flags.Target, "ecs")
		flags.Check = firstNonEmpty(flags.Check, "shell")
		flags.Cluster = firstNonEmpty(flags.Cluster, entry.Cluster)
		flags.Service = firstNonEmpty(flags.Service, entry.Service)
		flags.Task = firstNonEmpty(flags.Task, entry.Task)
	case appconfig.SavedWorkflowKindECSLogs:
		flags.Target = firstNonEmpty(flags.Target, "ecs")
		flags.Check = firstNonEmpty(flags.Check, "logs")
		flags.Cluster = firstNonEmpty(flags.Cluster, entry.Cluster)
		flags.Service = firstNonEmpty(flags.Service, entry.Service)
		flags.Task = firstNonEmpty(flags.Task, entry.Task)
	case appconfig.SavedWorkflowKindEC2Shell:
		flags.Target = firstNonEmpty(flags.Target, "ec2")
		flags.Check = firstNonEmpty(flags.Check, "shell")
		flags.Instance = firstNonEmpty(flags.Instance, entry.Instance)
	case appconfig.SavedWorkflowKindEC2PortForward:
		flags.Target = firstNonEmpty(flags.Target, "ec2")
		flags.Check = firstNonEmpty(flags.Check, "port-forward")
		flags.Instance = firstNonEmpty(flags.Instance, entry.Instance)
	}
}

var savedECSProviderFactory = func(runtime internalaws.Runtime, env *commandEnv) ecsprovider.Provider {
	return ecsprovider.New(runtime.Config, runtime.Profile, runtime.Region, env.runner)
}

var savedEC2ProviderFactory = func(runtime internalaws.Runtime, env *commandEnv) ec2provider.Provider {
	return ec2provider.New(runtime.Config, env.runner)
}

func savedWorkflowSummary(entry appconfig.SavedWorkflow) string {
	switch entry.Kind {
	case appconfig.SavedWorkflowKindECSShell:
		return fmt.Sprintf("%s / %s / %s", entry.Cluster, entry.Service, entry.Container)
	case appconfig.SavedWorkflowKindECSLogs:
		if entry.AllContainers {
			return fmt.Sprintf("%s / %s / all containers", entry.Cluster, entry.Service)
		}
		return fmt.Sprintf("%s / %s / %s", entry.Cluster, entry.Service, entry.Container)
	case appconfig.SavedWorkflowKindEC2Shell:
		return entry.Instance
	case appconfig.SavedWorkflowKindEC2PortForward:
		return fmt.Sprintf("%s %d->%s:%d", entry.Instance, entry.LocalPort, firstNonEmpty(entry.RemoteHost, "localhost"), entry.RemotePort)
	default:
		return string(entry.Kind)
	}
}

func savedWorkflowOptions(prefs *appconfig.Preferences) []tui.Option {
	names := prefs.SavedNames()
	options := make([]tui.Option, 0, len(names))
	for _, name := range names {
		entry := prefs.Saved[name]
		options = append(options, tui.Option{
			Label:   name,
			Details: fmt.Sprintf("%s | %s", entry.Kind, savedWorkflowSummary(entry)),
			Value:   "saved:" + name,
			Meta: map[string]string{
				"kind":    string(entry.Kind),
				"profile": entry.Profile,
				"region":  entry.Region,
			},
		})
	}
	return options
}
