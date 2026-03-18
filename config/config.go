package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ServiceConfig holds deployment details for a specific microservice.
type ServiceConfig struct {
	WorkingDirectory string `yaml:"working_directory,omitempty"` // Overrides project directory if set
	DeployCommand    string `yaml:"deploy_command"`
}

// ProjectConfig holds settings for a project and its services.
type ProjectConfig struct {
	WorkingDirectory string                   `yaml:"working_directory"` // Default directory for services
	DeployCommand    string                   `yaml:"deploy_command,omitempty"`
	Services         map[string]ServiceConfig `yaml:"services"`
}

// Config represents the top-level configuration file structure.
type Config struct {
	APIToken string                   `yaml:"api_token"`
	Projects map[string]ProjectConfig `yaml:"projects"`
}

// Load reads and unmarshals the configuration from the given file path.
func Load(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open config file: %w", err)
	}
	defer file.Close()

	var cfg Config
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("could not decode yaml config: %w", err)
	}

	if cfg.APIToken == "" {
		return nil, fmt.Errorf("api_token is required in config")
	}

	return &cfg, nil
}
