package screens

import "github.com/anupgiri/awscan/internal/tui"

func ContainerStep(options []tui.Option, selected string) tui.Step {
	return tui.Step{
		Key:          "container",
		Title:        "Select container",
		Options:      options,
		DefaultValue: selected,
	}
}
