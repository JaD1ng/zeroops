package config

import (
	"context"
	"time"
)

// DefaultHTTPServerConfig 创建默认HTTP服务器配置
func DefaultHTTPServerConfig(serviceName string, port int) HTTPServerConfig {
	return HTTPServerConfig{
		Name:             serviceName,
		Host:             "0.0.0.0",
		Port:             port,
		ReadTimeout:      30 * time.Second,
		WriteTimeout:     30 * time.Second,
		IdleTimeout:      60 * time.Second,
		GracefulShutdown: 30 * time.Second,
	}
}

// DefaultHTTPClientConfig 创建默认HTTP客户端配置
func DefaultHTTPClientConfig(serviceName string) HTTPClientConfig {
	return HTTPClientConfig{
		Name:            serviceName,
		Timeout:         30 * time.Second,
		MaxIdleConns:    100,
		MaxConnsPerHost: 100,
	}
}

// DefaultDatabaseConfig 创建默认数据库配置
func DefaultDatabaseConfig(dbType, host string, port int) DatabaseConfig {
	return DatabaseConfig{
		Type:            dbType,
		Host:            host,
		Port:            port,
		MaxOpenConns:    25,
		MaxIdleConns:    25,
		ConnMaxLifetime: 5 * time.Minute,
	}
}

// DefaultMetricsConfig 创建默认指标配置
func DefaultMetricsConfig(serviceName, version string) MetricsConfig {
	return MetricsConfig{
		ServiceName: serviceName,
		ServiceVer:  version,
		Namespace:   "mock_s3",
		Labels:      make(map[string]string),
		Enabled:     true,
		Port:        9090,
		Path:        "/metrics",
	}
}

// DefaultLoggingConfig 创建默认日志配置
func DefaultLoggingConfig() LoggingConfig {
	return LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: []string{"stdout"},
		Elasticsearch: ElasticsearchConfig{
			Enabled:    false,
			Host:       "localhost",
			Port:       9200,
			Index:      "logs",
			UseSSL:     false,
			MaxRetries: 3,
		},
	}
}

// DefaultTracingConfig 创建默认追踪配置
func DefaultTracingConfig(serviceName string) TracingConfig {
	return TracingConfig{
		Enabled:      true,
		ServiceName:  serviceName,
		SamplingRate: 1.0,
	}
}

// DefaultConsulConfig 创建默认Consul配置
func DefaultConsulConfig() ConsulConfig {
	return ConsulConfig{
		Address:    "localhost:8500",
		Scheme:     "http",
		Datacenter: "dc1",
		Timeout:    10 * time.Second,
	}
}

// LoadConfig 从环境变量和文件加载配置
func LoadConfig(configFile string, target any) error {
	loader := NewLoader(
		SourceConfig{Type: SourceEnv, Config: map[string]any{"prefix": "MOCK_S3_"}},
		SourceConfig{Type: SourceFile, Config: map[string]any{"path": configFile}},
	)
	defer loader.Close()

	return loader.Load(context.Background(), target)
}
