package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type SSHDefaults struct {
	User    string        `yaml:"user"`
	Key     string        `yaml:"key"`
	Timeout time.Duration `yaml:"timeout"`
}

type Host struct {
	Hostname string `yaml:"hostname"`
	SSHUser  string `yaml:"ssh_user,omitempty"`
	SSHKey   string `yaml:"ssh_key,omitempty"`
}

type Step struct {
	Name  string   `yaml:"name"`
	Type  string   `yaml:"type"` // "dependency", "application", or "command"
	Hosts []string `yaml:"hosts"`

	Start string `yaml:"start,omitempty"`
	Check string `yaml:"check,omitempty"`
	Stop  string `yaml:"stop,omitempty"`
	Run   string `yaml:"run,omitempty"`
}

type Environment struct {
	SSHDefaults SSHDefaults     `yaml:"ssh_defaults"`
	Hosts       map[string]Host `yaml:"hosts"`
	Sequence    []Step          `yaml:"sequence"`
}

type Config struct {
	Environments map[string]Environment `yaml:"environments"`
}

func LoadConfig(filePath string) (*Config, error) {
	// Read the YAML configuration file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file '%s': %w", filePath, err)
	}

	// Unmarshal the YAML into the Config struct
	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file '%s': %w", filePath, err)
	}

	return &cfg, nil
}
