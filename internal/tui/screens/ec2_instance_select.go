package screens

import "github.com/anupgiri/awscan/internal/tui"

func InstanceStep(options []tui.Option, selected string) tui.Step {
	return tui.Step{
		Key:          "instance",
		Title:        "Select EC2 instance",
		Placeholder:  "search instance by name, ID, IP, AZ, or SSM state",
		Options:      options,
		DefaultValue: selected,
	}
}
