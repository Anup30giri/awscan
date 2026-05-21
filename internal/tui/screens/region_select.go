package screens

import "github.com/anupgiri/awscan/internal/tui"

func RegionStep(options []tui.Option, selected string) tui.Step {
	return tui.Step{
		Key:          "region",
		Title:        "Select AWS region",
		Placeholder:  "search region, for example ap-south-1",
		Options:      options,
		DefaultValue: selected,
	}
}
