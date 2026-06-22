package config

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type Preferences struct {
	DefaultProfile string                   `yaml:"default_profile,omitempty"`
	DefaultRegion  string                   `yaml:"default_region,omitempty"`
	Recent         RecentPreferences        `yaml:"recent,omitempty"`
	DefaultShells  map[string]string        `yaml:"default_shells,omitempty"`
	Saved          map[string]SavedWorkflow `yaml:"saved,omitempty"`
}

type RecentPreferences struct {
	ECS ECSRecentPreferences `yaml:"ecs,omitempty"`
	EC2 EC2RecentPreferences `yaml:"ec2,omitempty"`
}

type ECSRecentPreferences struct {
	Cluster   string `yaml:"cluster,omitempty"`
	Service   string `yaml:"service,omitempty"`
	Container string `yaml:"container,omitempty"`
}

type EC2RecentPreferences struct {
	InstanceID string `yaml:"instance_id,omitempty"`
	RemoteHost string `yaml:"remote_host,omitempty"`
	LocalPort  int    `yaml:"local_port,omitempty"`
	RemotePort int    `yaml:"remote_port,omitempty"`
}

type SavedWorkflowKind string

const (
	SavedWorkflowKindECSShell       SavedWorkflowKind = "ecs-shell"
	SavedWorkflowKindECSLogs        SavedWorkflowKind = "ecs-logs"
	SavedWorkflowKindEC2Shell       SavedWorkflowKind = "ec2-shell"
	SavedWorkflowKindEC2PortForward SavedWorkflowKind = "ec2-port-forward"
)

type SavedWorkflow struct {
	Kind          SavedWorkflowKind `yaml:"kind"`
	Profile       string            `yaml:"profile,omitempty"`
	Region        string            `yaml:"region,omitempty"`
	Cluster       string            `yaml:"cluster,omitempty"`
	Service       string            `yaml:"service,omitempty"`
	Task          string            `yaml:"task,omitempty"`
	Container     string            `yaml:"container,omitempty"`
	Command       string            `yaml:"command,omitempty"`
	Since         string            `yaml:"since,omitempty"`
	Follow        bool              `yaml:"follow,omitempty"`
	AllContainers bool              `yaml:"all_containers,omitempty"`
	Instance      string            `yaml:"instance,omitempty"`
	LocalPort     int               `yaml:"local_port,omitempty"`
	RemoteHost    string            `yaml:"remote_host,omitempty"`
	RemotePort    int               `yaml:"remote_port,omitempty"`
}

var savedNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func DefaultPreferences() *Preferences {
	return &Preferences{
		DefaultShells: map[string]string{},
		Saved:         map[string]SavedWorkflow{},
	}
}

func (p *Preferences) Normalize() {
	if p.DefaultShells == nil {
		p.DefaultShells = map[string]string{}
	}
	if p.Saved == nil {
		p.Saved = map[string]SavedWorkflow{}
	}
}

func ValidateSavedWorkflowName(name string) error {
	if !savedNamePattern.MatchString(name) {
		return fmt.Errorf("saved workflow name %q is invalid. Use lowercase letters, numbers, and hyphens only", name)
	}
	return nil
}

func (w SavedWorkflow) Validate() error {
	if strings.TrimSpace(w.Profile) == "" {
		return fmt.Errorf("profile is required")
	}
	if strings.TrimSpace(w.Region) == "" {
		return fmt.Errorf("region is required")
	}

	switch w.Kind {
	case SavedWorkflowKindECSShell:
		if strings.TrimSpace(w.Cluster) == "" || strings.TrimSpace(w.Service) == "" || strings.TrimSpace(w.Container) == "" {
			return fmt.Errorf("cluster, service, and container are required for %s", w.Kind)
		}
	case SavedWorkflowKindECSLogs:
		if strings.TrimSpace(w.Cluster) == "" || strings.TrimSpace(w.Service) == "" {
			return fmt.Errorf("cluster and service are required for %s", w.Kind)
		}
		if !w.AllContainers && strings.TrimSpace(w.Container) == "" {
			return fmt.Errorf("container is required for %s unless all_containers is true", w.Kind)
		}
	case SavedWorkflowKindEC2Shell:
		if strings.TrimSpace(w.Instance) == "" {
			return fmt.Errorf("instance is required for %s", w.Kind)
		}
	case SavedWorkflowKindEC2PortForward:
		if strings.TrimSpace(w.Instance) == "" || w.LocalPort <= 0 || w.RemotePort <= 0 {
			return fmt.Errorf("instance, local_port, and remote_port are required for %s", w.Kind)
		}
	default:
		return fmt.Errorf("unsupported saved workflow kind %q", w.Kind)
	}
	return nil
}

func (p *Preferences) ValidateSavedWorkflow(name string) (SavedWorkflow, error) {
	p.Normalize()
	if err := ValidateSavedWorkflowName(name); err != nil {
		return SavedWorkflow{}, err
	}
	workflow, ok := p.Saved[name]
	if !ok {
		return SavedWorkflow{}, fmt.Errorf("saved workflow %q was not found", name)
	}
	if err := workflow.Validate(); err != nil {
		return SavedWorkflow{}, fmt.Errorf("saved workflow %q is invalid: %w", name, err)
	}
	return workflow, nil
}

func (p *Preferences) SavedNames() []string {
	p.Normalize()
	names := make([]string, 0, len(p.Saved))
	for name := range p.Saved {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
