package plugin

import "github.com/spf13/cobra"

type ResourceProvider interface {
	Name() string
}

type ShellTargetProvider interface {
	Name() string
	ShellCommand() *cobra.Command
}
