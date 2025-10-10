package config

import (
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// PrometheusAdapterConfig Prometheus Adapter 配置
type PrometheusAdapterConfig struct {
	Prometheus   PrometheusConfig   `yaml:"prometheus"`
	AlertWebhook AlertWebhookConfig `yaml:"alert_webhook"`
	AlertRules   AlertRulesConfig   `yaml:"alert_rules"`
	Server       ServerConfig       `yaml:"server"`
}

// PrometheusConfig Prometheus 服务配置
type PrometheusConfig struct {
	Address       string `yaml:"address"`        // Prometheus 地址
	ContainerName string `yaml:"container_name"` // 容器名称
}

// AlertWebhookConfig 告警 Webhook 配置
type AlertWebhookConfig struct {
	URL             string `yaml:"url"`              // Webhook URL
	PollingInterval string `yaml:"polling_interval"` // 轮询间隔
}

// AlertRulesConfig 告警规则配置
type AlertRulesConfig struct {
	LocalFile          string `yaml:"local_file"`           // 本地规则文件
	PrometheusRulesDir string `yaml:"prometheus_rules_dir"` // Prometheus 规则目录
}

// ServerConfig 服务器配置
type ServerConfig struct {
	BindAddr string `yaml:"bind_addr"` // 监听地址
}

// LoadConfig 加载配置文件
func LoadConfig(configPath string) (*PrometheusAdapterConfig, error) {
	// 如果没有指定配置文件，尝试多个默认路径
	if configPath == "" {
		// 尝试的路径列表（按优先级）
		possiblePaths := []string{
			"config/prometheus_adapter.yml",                             // 部署环境：相对于工作目录
			"internal/prometheus_adapter/config/prometheus_adapter.yml", // 开发环境：源码目录
			"./prometheus_adapter.yml",                                  // 当前目录
		}

		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				configPath = path
				log.Info().Str("path", path).Msg("Found config file")
				break
			}
		}

		// 如果都找不到，使用第一个路径（稍后会返回默认配置）
		if configPath == "" {
			configPath = possiblePaths[0]
		}
	}

	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		// 如果文件不存在，返回默认配置
		if os.IsNotExist(err) {
			log.Warn().Msg("Config file not found, using default configuration")
			return getDefaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 解析配置
	var config PrometheusAdapterConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 应用环境变量覆盖
	applyEnvOverrides(&config)

	// 验证配置
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	log.Info().
		Str("config_file", configPath).
		Msg("Configuration loaded successfully")

	return &config, nil
}

// getDefaultConfig 获取默认配置
func getDefaultConfig() *PrometheusAdapterConfig {
	return &PrometheusAdapterConfig{
		Prometheus: PrometheusConfig{
			Address:       "http://10.210.10.33:9090",
			ContainerName: "mock-s3-prometheus",
		},
		AlertWebhook: AlertWebhookConfig{
			URL:             "http://alert-module:8080/v1/integrations/alertmanager/webhook",
			PollingInterval: "10s",
		},
		AlertRules: AlertRulesConfig{
			LocalFile:          "../rules/alert_rules.yml",
			PrometheusRulesDir: "/etc/prometheus/rules/",
		},
		Server: ServerConfig{
			BindAddr: "0.0.0.0:9999",
		},
	}
}

// applyEnvOverrides 应用环境变量覆盖
func applyEnvOverrides(config *PrometheusAdapterConfig) {
	// Prometheus 配置
	if addr := os.Getenv("PROMETHEUS_ADDRESS"); addr != "" {
		config.Prometheus.Address = addr
	}
	if container := os.Getenv("PROMETHEUS_CONTAINER"); container != "" {
		config.Prometheus.ContainerName = container
	}

	// Alert Webhook 配置
	if url := os.Getenv("ALERT_WEBHOOK_URL"); url != "" {
		config.AlertWebhook.URL = url
	}
	if interval := os.Getenv("ALERT_POLLING_INTERVAL"); interval != "" {
		config.AlertWebhook.PollingInterval = interval
	}

	// Server 配置
	if bindAddr := os.Getenv("SERVER_BIND_ADDR"); bindAddr != "" {
		config.Server.BindAddr = bindAddr
	}
}

// validateConfig 验证配置
func validateConfig(config *PrometheusAdapterConfig) error {
	// 验证 Prometheus 地址
	if config.Prometheus.Address == "" {
		return fmt.Errorf("prometheus address is required")
	}

	// 验证轮询间隔
	if config.AlertWebhook.PollingInterval != "" {
		if _, err := time.ParseDuration(config.AlertWebhook.PollingInterval); err != nil {
			return fmt.Errorf("invalid polling interval: %w", err)
		}
	}

	// 验证服务器地址
	if config.Server.BindAddr == "" {
		return fmt.Errorf("server bind address is required")
	}

	return nil
}

// GetPollingInterval 获取轮询间隔的 Duration
func (c *AlertWebhookConfig) GetPollingInterval() time.Duration {
	if c.PollingInterval == "" {
		return 10 * time.Second
	}

	duration, err := time.ParseDuration(c.PollingInterval)
	if err != nil {
		log.Warn().
			Err(err).
			Str("interval", c.PollingInterval).
			Msg("Invalid polling interval, using default")
		return 10 * time.Second
	}

	return duration
}
