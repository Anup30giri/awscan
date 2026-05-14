package app

import (
	"log/slog"
	"os"

	appconfig "github.com/anupgiri/awscan/internal/config"
)

type App struct {
	Logger *slog.Logger
	Config *appconfig.Manager
}

func New() (*App, error) {
	cfg, err := appconfig.NewManager()
	if err != nil {
		return nil, err
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	return &App{
		Logger: logger,
		Config: cfg,
	}, nil
}
