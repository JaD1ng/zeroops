package config

import (
	"fmt"
	"time"
)

// ServiceConfig 服务配置基础接口
type ServiceConfig interface {
	GetServiceName() string
	GetVersion() string
	Validate() error
}

// HTTPServerConfig HTTP服务器配置
type HTTPServerConfig struct {
	Name             string        `json:"name" yaml:"name"`
	Host             string        `json:"host" yaml:"host" default:"0.0.0.0"`
	Port             int           `json:"port" yaml:"port"`
	ReadTimeout      time.Duration `json:"read_timeout" yaml:"read_timeout" default:"30s"`
	WriteTimeout     time.Duration `json:"write_timeout" yaml:"write_timeout" default:"30s"`
	IdleTimeout      time.Duration `json:"idle_timeout" yaml:"idle_timeout" default:"60s"`
	GracefulShutdown time.Duration `json:"graceful_shutdown" yaml:"graceful_shutdown" default:"30s"`
}

// HTTPClientConfig HTTP客户端配置
type HTTPClientConfig struct {
	Name            string        `json:"name" yaml:"name"`
	Timeout         time.Duration `json:"timeout" yaml:"timeout" default:"30s"`
	MaxIdleConns    int           `json:"max_idle_conns" yaml:"max_idle_conns" default:"100"`
	MaxConnsPerHost int           `json:"max_conns_per_host" yaml:"max_conns_per_host" default:"100"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Type            string        `json:"type" yaml:"type"`
	Host            string        `json:"host" yaml:"host"`
	Port            int           `json:"port" yaml:"port"`
	Database        string        `json:"database" yaml:"database"`
	Username        string        `json:"username" yaml:"username"`
	Password        string        `json:"password" yaml:"password"`
	MaxOpenConns    int           `json:"max_open_conns" yaml:"max_open_conns" default:"25"`
	MaxIdleConns    int           `json:"max_idle_conns" yaml:"max_idle_conns" default:"25"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime" yaml:"conn_max_lifetime" default:"5m"`
}

// MetricsConfig 指标配置
type MetricsConfig struct {
	ServiceName string            `json:"service_name" yaml:"service_name"`
	ServiceVer  string            `json:"service_version" yaml:"service_version"`
	Namespace   string            `json:"namespace" yaml:"namespace"`
	Labels      map[string]string `json:"labels" yaml:"labels"`
	Enabled     bool              `json:"enabled" yaml:"enabled" default:"true"`
	Port        int               `json:"port" yaml:"port" default:"9090"`
	Path        string            `json:"path" yaml:"path" default:"/metrics"`
}

// Validate 验证MetricsConfig
func (c MetricsConfig) Validate() error {
	if c.ServiceName == "" {
		return fmt.Errorf("service_name cannot be empty")
	}
	if c.Namespace == "" {
		return fmt.Errorf("namespace cannot be empty")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if c.Path == "" {
		return fmt.Errorf("path cannot be empty")
	}
	return nil
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level  string   `json:"level" yaml:"level" default:"info"`
	Format string   `json:"format" yaml:"format" default:"json"`
	Output []string `json:"output" yaml:"output" default:"[stdout]"`
	// es配置
	Elasticsearch ElasticsearchConfig `json:"elasticsearch" yaml:"elasticsearch"`
}

// TracingConfig 链路追踪配置
type TracingConfig struct {
	Enabled      bool    `json:"enabled" yaml:"enabled" default:"true"`
	ServiceName  string  `json:"service_name" yaml:"service_name"`
	SamplingRate float64 `json:"sampling_rate" yaml:"sampling_rate" default:"1.0"`
}

// ConsulConfig Consul配置
type ConsulConfig struct {
	Address    string        `json:"address" yaml:"address" default:"localhost:8500"`
	Scheme     string        `json:"scheme" yaml:"scheme" default:"http"`
	Datacenter string        `json:"datacenter" yaml:"datacenter" default:"dc1"`
	Token      string        `json:"token" yaml:"token"`
	Timeout    time.Duration `json:"timeout" yaml:"timeout" default:"10s"`
}

// ElasticsearchConfig Elasticsearch配置
type ElasticsearchConfig struct {
	Enabled    bool   `json:"enabled" yaml:"enabled" default:"false"`
	Host       string `json:"host" yaml:"host" default:"localhost"`
	Port       int    `json:"port" yaml:"port" default:"9200"`
	Index      string `json:"index" yaml:"index" default:"logs"`
	Username   string `json:"username" yaml:"username"`
	Password   string `json:"password" yaml:"password"`
	UseSSL     bool   `json:"use_ssl" yaml:"use_ssl" default:"false"`
	MaxRetries int    `json:"max_retries" yaml:"max_retries" default:"3"`
}
