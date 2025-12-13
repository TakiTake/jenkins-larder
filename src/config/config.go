package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Storage  StorageConfig  `yaml:"storage"`
	Upstream UpstreamConfig `yaml:"upstream"`
	Server   ServerConfig   `yaml:"server"`
	Admin    AdminConfig    `yaml:"admin"`
	TTL      TTLConfig      `yaml:"ttl"`
}

type StorageConfig struct {
	LimitBytes int64  `yaml:"limit_bytes"`
	Path       string `yaml:"path"`
}

type UpstreamConfig struct {
	URL            string `yaml:"url"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

type ServerConfig struct {
	Port        int `yaml:"port"`
	MetricsPort int `yaml:"metrics_port"`
}

type AdminConfig struct {
	Port int `yaml:"port"`
}

type TTLConfig struct {
	Enabled      bool `yaml:"enabled"`
	DefaultHours int  `yaml:"default_hours"`
}

// Load reads configuration from YAML file
// TODO (T008): Implement YAML file loading
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// TODO (T009): Call validation
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if err := ValidateStorageConfig(&c.Storage); err != nil {
		return err
	}
	if err := ValidateUpstreamConfig(&c.Upstream); err != nil {
		return err
	}
	if err := ValidateServerPorts(&c.Server, &c.Admin); err != nil {
		return err
	}
	return nil
}
