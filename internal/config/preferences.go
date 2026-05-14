package config

type Preferences struct {
	DefaultProfile string            `yaml:"default_profile,omitempty"`
	DefaultRegion  string            `yaml:"default_region,omitempty"`
	Recent         RecentPreferences `yaml:"recent,omitempty"`
	DefaultShells  map[string]string `yaml:"default_shells,omitempty"`
}

type RecentPreferences struct {
	ECS ECSRecentPreferences `yaml:"ecs,omitempty"`
}

type ECSRecentPreferences struct {
	Cluster   string `yaml:"cluster,omitempty"`
	Service   string `yaml:"service,omitempty"`
	Container string `yaml:"container,omitempty"`
}

func DefaultPreferences() *Preferences {
	return &Preferences{
		DefaultShells: map[string]string{},
	}
}

func (p *Preferences) Normalize() {
	if p.DefaultShells == nil {
		p.DefaultShells = map[string]string{}
	}
}
