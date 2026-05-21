package screens

import "github.com/anupgiri/awscan/internal/tui"

func ContainerStep(options []tui.Option, selected string) tui.Step {
	return tui.Step{
		Key:          "container",
		Title:        "Select container",
		Placeholder:  "search container by name or runtime",
		Options:      options,
		DefaultValue: selected,
	}
}
