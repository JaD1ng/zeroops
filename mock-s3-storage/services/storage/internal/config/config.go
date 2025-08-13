package config

import (
	"context"
	"fmt"
	"shared/config"
)

// StorageConfig 存储服务配置
type StorageConfig struct {
	Service   ServiceConfig            `json:"service" yaml:"service"`
	Server    config.HTTPServerConfig  `json:"server" yaml:"server"`
	Storage   StorageBackendConfig     `json:"storage" yaml:"storage"`
	Metrics   config.MetricsConfig     `json:"metrics" yaml:"metrics"`
	Tracing   config.TracingConfig     `json:"tracing" yaml:"tracing"`
	Discovery config.ConsulConfig      `json:"discovery" yaml:"discovery"`
	Logging   config.LoggingConfig     `json:"logging" yaml:"logging"`
}

// ServiceConfig 存储服务专用配置
type ServiceConfig struct {
	Name        string `json:"name" yaml:"name" default:"storage-service"`
	Version     string `json:"version" yaml:"version" default:"1.0.0"`
	MaxFileSize int64  `json:"max_file_size" yaml:"max_file_size" default:"1048576"` // 1MB
}

// StorageBackendConfig 存储后端配置
type StorageBackendConfig struct {
	Type    string `json:"type" yaml:"type" default:"filesystem"`        // filesystem, s3, etc.
	BaseDir string `json:"base_dir" yaml:"base_dir" default:"./storage"` // 文件系统存储基础目录
}

// GetServiceName 实现 config.ServiceConfig 接口
func (c ServiceConfig) GetServiceName() string {
	return c.Name
}

// GetVersion 实现 config.ServiceConfig 接口  
func (c ServiceConfig) GetVersion() string {
	return c.Version
}

// Validate 验证配置
func (c ServiceConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("service name cannot be empty")
	}
	if c.Version == "" {
		return fmt.Errorf("service version cannot be empty")
	}
	if c.MaxFileSize <= 0 {
		return fmt.Errorf("max file size must be positive")
	}
	return nil
}

// LoadConfig 加载配置
func LoadConfig() (*StorageConfig, error) {
	cfg := &StorageConfig{}
	
	// 使用环境变量配置加载器
	loader := config.NewEnvLoader("STORAGE")
	
	// 加载配置
	if err := loader.Load(context.TODO(), cfg); err != nil {
		return nil, fmt.Errorf("load config failed: %w", err)
	}
	
	// 验证配置
	if err := cfg.Service.Validate(); err != nil {
		return nil, fmt.Errorf("invalid service config: %w", err)
	}
	
	if err := cfg.Metrics.Validate(); err != nil {
		return nil, fmt.Errorf("invalid metrics config: %w", err)
	}
	
	return cfg, nil
}

// GetDefaultConfig 获取默认配置
func GetDefaultConfig() *StorageConfig {
	return &StorageConfig{
		Service: ServiceConfig{
			Name:        "storage-service",
			Version:     "1.0.0",
			MaxFileSize: 1048576, // 1MB
		},
		Server: config.HTTPServerConfig{
			Name: "storage-server",
			Host: "0.0.0.0",
			Port: 8080,
		},
		Storage: StorageBackendConfig{
			Type:    "filesystem",
			BaseDir: "./storage",
		},
		Metrics: config.MetricsConfig{
			ServiceName: "storage-service",
			ServiceVer:  "1.0.0",
			Namespace:   "storage",
			Enabled:     true,
			Port:        9090,
			Path:        "/metrics",
		},
		Tracing: config.TracingConfig{
			Enabled:      true,
			ServiceName:  "storage-service",
			SamplingRate: 1.0,
		},
		Discovery: config.ConsulConfig{
			Address:    "localhost:8500",
			Scheme:     "http",
			Datacenter: "dc1",
		},
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: []string{"stdout"},
		},
	}
}