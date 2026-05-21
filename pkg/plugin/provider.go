package plugin

import (
	"context"

	"github.com/spf13/cobra"
)

type ServiceRegistration struct {
	ID          string
	Name        string
	Description string
	BuildRoot   func() *cobra.Command
	DefaultRun  func(ctx context.Context) error
}

func (s ServiceRegistration) TargetOption() (label, details, value string) {
	return s.Name, s.Description, s.ID
}
