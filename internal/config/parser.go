package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the overall configuration structure.
type Config struct {
	Environments map[string]Environment `yaml:"environments"`
}

// Environment represents a specific environment configuration.
type Environment struct {
	RemoteUser   string        `yaml:"remote_user"`
	Applications []Application `yaml:"applications"`
}

// Application represents an individual application's configuration.
type Application struct {
	Name          string `yaml:"name"`
	Host          string `yaml:"host"`
	StartCommand  string `yaml:"start_command"`
	StopCommand   string `yaml:"stop_command"`
	CheckCommand  string `yaml:"check_command"`
	CheckInterval int    `yaml:"check_interval"`
}

// LoadConfig reads and parses the YAML configuration file.
func LoadConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling YAML: %w", err)
	}

	for envName, env := range cfg.Environments {
		if env.RemoteUser == "" {
			return nil, fmt.Errorf("remote_user is missing for environment '%s'", envName)
		}
		if len(env.Applications) == 0 {
			return nil, fmt.Errorf("no applications defined for environment '%s'", envName)
		}
		for _, app := range env.Applications {
			if app.Name == "" || app.Host == "" || app.StartCommand == "" || app.StopCommand == "" || app.CheckCommand == "" || app.CheckInterval <= 0 {
				return nil, fmt.Errorf("invalid configuration for application '%s' in environment '%s'", app.Name, envName)
			}
		}
	}

	return &cfg, nil
}
