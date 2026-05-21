package cmd

import "github.com/spf13/cobra"

func newEC2Command(env *commandEnv, root *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ec2",
		Short: "EC2 and Session Manager workflows",
	}

	cmd.AddCommand(newEC2ShellCommand(env, root))
	cmd.AddCommand(newEC2InspectCommand(env, root))
	cmd.AddCommand(newEC2DocumentsCommand(env, root))
	cmd.AddCommand(newEC2PortForwardCommand(env, root))
	return cmd
}
