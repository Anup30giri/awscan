package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/anupgiri/awscan/internal/app"
	internalaws "github.com/anupgiri/awscan/internal/aws"
	appconfig "github.com/anupgiri/awscan/internal/config"
	internalexec "github.com/anupgiri/awscan/internal/exec"
)

type commandEnv struct {
	app      *app.App
	prefs    *appconfig.Preferences
	resolver internalaws.Resolver
	runner   internalexec.Runner
}

type rootFlags struct {
	profile string
	region  string
}

func Execute(ctx context.Context) error {
	application, err := app.New()
	if err != nil {
		return err
	}

	prefs, err := application.Config.Load()
	if err != nil {
		return err
	}

	env := &commandEnv{
		app:      application,
		prefs:    prefs,
		resolver: internalaws.NewConfigResolver(prefs),
		runner:   internalexec.NewRunner(),
	}

	root := newRootCommand(env)
	root.SetContext(ctx)
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}

func newRootCommand(env *commandEnv) *cobra.Command {
	flags := &rootFlags{}
	services := registeredServices(env, flags)

	cmd := &cobra.Command{
		Use:           "awscan",
		Short:         "AWS terminal navigator focused on ECS Exec workflows",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVar(&flags.profile, "profile", "", "AWS profile to use")
	cmd.PersistentFlags().StringVar(&flags.region, "region", "", "AWS region to use")

	cmd.AddCommand(newDoctorCommand(env, flags))
	for _, service := range services {
		cmd.AddCommand(service.BuildRoot())
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		runtime, err := resolveShellRuntime(cmd.Context(), env, flags, false)
		if err != nil {
			return err
		}
		target, err := selectDefaultTarget(cmd.Context(), runtime, services)
		if err != nil {
			return err
		}
		service, err := serviceCommandByID(services, target)
		if err != nil {
			return err
		}
		return service.DefaultRun(cmd.Context())
	}

	return cmd
}
