package screens

import "github.com/anupgiri/awscan/internal/tui"

func TaskStep(options []tui.Option, selected string) tui.Step {
	return tui.Step{
		Key:          "task",
		Title:        "Select running ECS task",
		Placeholder:  "search task by ID, status, or launch type",
		Options:      options,
		DefaultValue: selected,
	}
}
