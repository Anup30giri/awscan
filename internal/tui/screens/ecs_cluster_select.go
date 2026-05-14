package screens

import "github.com/anupgiri/awscan/internal/tui"

func ClusterStep(options []tui.Option, selected string) tui.Step {
	return tui.Step{
		Key:          "cluster",
		Title:        "Select ECS cluster",
		Options:      options,
		DefaultValue: selected,
	}
}
