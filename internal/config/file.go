package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	appDirName = "awscan"
	configFile = "config.yaml"
)

type Manager struct {
	path string
}

func NewManager() (*Manager, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user config dir: %w", err)
	}

	return &Manager{
		path: filepath.Join(dir, appDirName, configFile),
	}, nil
}

func NewManagerForPath(path string) *Manager {
	return &Manager{path: path}
}

func (m *Manager) Path() string {
	return m.path
}

func (m *Manager) Load() (*Preferences, error) {
	data, err := os.ReadFile(m.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultPreferences(), nil
		}
		return nil, fmt.Errorf("read config file %q: %w", m.path, err)
	}

	cfg := DefaultPreferences()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file %q: %w", m.path, err)
	}

	cfg.Normalize()
	return cfg, nil
}

func (m *Manager) Save(cfg *Preferences) error {
	if cfg == nil {
		return errors.New("preferences are nil")
	}

	cfg.Normalize()

	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(m.path, data, 0o600); err != nil {
		return fmt.Errorf("write config file %q: %w", m.path, err)
	}

	return nil
}
