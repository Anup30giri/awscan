package screens

import "github.com/anupgiri/awscan/internal/tui"

func RegionStep(options []tui.Option, selected string) tui.Step {
	return tui.Step{
		Key:          "region",
		Title:        "Select AWS region",
		Options:      options,
		DefaultValue: selected,
	}
}
