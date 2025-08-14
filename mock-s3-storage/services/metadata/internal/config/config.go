package config

import (
	"fmt"
	"shared/config"
	"time"
)

type MetadataConfig struct {
	Service   ServiceConfig           `json:"service" yaml:"service"`
	HTTP      config.HTTPServerConfig `json:"http" yaml:"http"`
	Database  config.DatabaseConfig   `json:"database" yaml:"database"`
	Cache     config.DatabaseConfig   `json:"cache" yaml:"cache"`
	Metrics   config.MetricsConfig    `json:"metrics" yaml:"metrics"`
	Logging   config.LoggingConfig    `json:"logging" yaml:"logging"`
	Tracing   config.TracingConfig    `json:"tracing" yaml:"tracing"`
	Discovery config.ConsulConfig     `json:"discovery" yaml:"discovery"`
}

type ServiceConfig struct {
	Name            string        `json:"name" yaml:"name"`
	Version         string        `json:"version" yaml:"version"`
	Environment     string        `json:"environment" yaml:"environment"`
	Region          string        `json:"region" yaml:"region"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout" yaml:"shutdown_timeout"`
}

func (c *ServiceConfig) GetServiceName() string {
	return c.Name
}

func (c *ServiceConfig) GetVersion() string {
	return c.Version
}

func (c *ServiceConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("service name cannot be empty")
	}
	return nil
}

func DefaultConfig() *MetadataConfig {
	return &MetadataConfig{
		Service: ServiceConfig{
			Name:            "metadata-service",
			Version:         "1.0.0",
			Environment:     "development",
			Region:          "us-west-2",
			ShutdownTimeout: 30 * time.Second,
		},
		HTTP:      config.DefaultHTTPServerConfig("metadata-service", 8080),
		Database:  config.DefaultDatabaseConfig("postgres", "localhost", 5432),
		Cache:     config.DefaultDatabaseConfig("redis", "localhost", 6379),
		Metrics:   config.DefaultMetricsConfig("metadata-service", "1.0.0"),
		Logging:   config.DefaultLoggingConfig(),
		Tracing:   config.DefaultTracingConfig("metadata-service"),
		Discovery: config.DefaultConsulConfig(),
	}
}

func LoadConfig(configFile string) (*MetadataConfig, error) {
	cfg := DefaultConfig()

	if configFile != "" {
		err := config.LoadConfig(configFile, cfg)
		if err != nil {
			return nil, err
		}
	}

	if err := cfg.Service.Validate(); err != nil {
		return nil, err
	}

	if err := cfg.Metrics.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}
