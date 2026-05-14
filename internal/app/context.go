package app

import (
	"context"

	internalaws "github.com/anupgiri/awscan/internal/aws"
	appconfig "github.com/anupgiri/awscan/internal/config"
)

type RuntimeContext struct {
	Context      context.Context
	AWS          internalaws.Runtime
	Preferences  *appconfig.Preferences
	NonInteractive bool
}
