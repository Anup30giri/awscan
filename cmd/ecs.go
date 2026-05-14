package cmd

import "github.com/spf13/cobra"

func newECSCommand(env *commandEnv, root *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ecs",
		Short: "ECS workflows",
	}

	cmd.AddCommand(newECSShellCommand(env, root))
	return cmd
}
