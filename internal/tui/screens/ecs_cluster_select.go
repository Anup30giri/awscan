package screens

import "github.com/anupgiri/awscan/internal/tui"

func ClusterStep(options []tui.Option, selected string) tui.Step {
	return tui.Step{
		Key:          "cluster",
		Title:        "Select ECS cluster",
		Placeholder:  "search cluster by name or ARN",
		Options:      options,
		DefaultValue: selected,
	}
}
