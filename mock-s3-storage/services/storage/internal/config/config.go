package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Database      DatabaseConfig      `yaml:"database"`
	OpenTelemetry OpenTelemetryConfig `yaml:"opentelemetry"`
	Logging       LoggingConfig       `yaml:"logging"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host             string        `yaml:"host" default:"127.0.0.1"`
	Port             int           `yaml:"port" default:"8080"`
	ReadTimeout      time.Duration `yaml:"read_timeout" default:"30s"`
	WriteTimeout     time.Duration `yaml:"write_timeout" default:"30s"`
	IdleTimeout      time.Duration `yaml:"idle_timeout" default:"60s"`
	GracefulShutdown time.Duration `yaml:"graceful_shutdown" default:"30s"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host            string        `yaml:"host" default:"localhost"`
	Port            int           `yaml:"port" default:"5432"`
	Database        string        `yaml:"database" default:"mock"`
	Username        string        `yaml:"username" default:"postgres"`
	Password        string        `yaml:"password" default:"123456"`
	SSLMode         string        `yaml:"ssl_mode" default:"disable"`
	MaxOpenConns    int           `yaml:"max_open_conns" default:"25"`
	MaxIdleConns    int           `yaml:"max_idle_conns" default:"25"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" default:"5m"`
	TableName       string        `yaml:"table_name" default:"files"`
}

// MetricsConfig 指标配置
type MetricsConfig struct {
	ServiceName string            `yaml:"service_name" default:"file-storage-service"`
	ServiceVer  string            `yaml:"service_version" default:"1.0.0"`
	Namespace   string            `yaml:"namespace" default:"storage"`
	Labels      map[string]string `yaml:"labels"`
	Enabled     bool              `yaml:"enabled" default:"true"`
	Port        int               `yaml:"port" default:"1080"`
	Path        string            `yaml:"path" default:"/metrics"`
}

// OpenTelemetryConfig OpenTelemetry配置
type OpenTelemetryConfig struct {
	ServiceName    string `json:"service_name" yaml:"service_name"`
	ServiceVersion string `json:"service_version" yaml:"service_version"`
	Environment    string `json:"environment" yaml:"environment"`
	MetricsPort    int    `json:"metrics_port" yaml:"metrics_port"`
	MetricsPath    string `json:"metrics_path" yaml:"metrics_path"`
	EnableTracing  bool   `json:"enable_tracing" yaml:"enable_tracing"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level         string              `yaml:"level" default:"info"`
	Format        string              `yaml:"format" default:"json"`
	Output        []string            `yaml:"output" default:"[stdout]"`
	Elasticsearch ElasticsearchConfig `yaml:"elasticsearch"`
}

// ElasticsearchConfig Elasticsearch配置
type ElasticsearchConfig struct {
	Enabled    bool   `yaml:"enabled" default:"false"`
	Host       string `yaml:"host" default:"localhost"`
	Port       int    `yaml:"port" default:"9200"`
	Index      string `yaml:"index" default:"logs"`
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
	UseSSL     bool   `yaml:"use_ssl" default:"false"`
	MaxRetries int    `yaml:"max_retries" default:"3"`
}

// LoadConfig 加载配置
func LoadConfig() (*Config, error) {
	// 尝试从YAML文件加载配置
	config := &Config{}

	// 读取config.yaml文件
	data, err := os.ReadFile("config.yaml")
	if err == nil {
		// 成功读取文件，解析YAML
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("解析YAML配置文件失败: %v", err)
		}
	} else {
		// 文件不存在，使用默认值
		config = &Config{
			Server: ServerConfig{
				Host:             getEnv("HOST", "127.0.0.1"),
				Port:             getEnvAsInt("PORT", 8081),
				ReadTimeout:      getEnvAsDuration("READ_TIMEOUT", 30*time.Second),
				WriteTimeout:     getEnvAsDuration("WRITE_TIMEOUT", 30*time.Second),
				IdleTimeout:      getEnvAsDuration("IDLE_TIMEOUT", 60*time.Second),
				GracefulShutdown: getEnvAsDuration("GRACEFUL_SHUTDOWN", 30*time.Second),
			},
			Database: DatabaseConfig{
				Host:            getEnv("DB_HOST", "localhost"),
				Port:            getEnvAsInt("DB_PORT", 5432),
				Database:        getEnv("DB_NAME", "mock"),
				Username:        getEnv("DB_USER", "postgres"),
				Password:        getEnv("DB_PASSWORD", "123456"),
				SSLMode:         getEnv("DB_SSL_MODE", "disable"),
				MaxOpenConns:    getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
				MaxIdleConns:    getEnvAsInt("DB_MAX_IDLE_CONNS", 25),
				ConnMaxLifetime: getEnvAsDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
				TableName:       getEnv("TABLE_NAME", "files"),
			},
			OpenTelemetry: OpenTelemetryConfig{
				ServiceName:    getEnv("OTEL_SERVICE_NAME", "storage-service"),
				ServiceVersion: getEnv("OTEL_SERVICE_VERSION", "1.0.0"),
				Environment:    getEnv("OTEL_ENVIRONMENT", "development"),
				MetricsPort:    getEnvAsInt("OTEL_METRICS_PORT", 1080),
				MetricsPath:    getEnv("OTEL_METRICS_PATH", "/metrics"),
				EnableTracing:  getEnvAsBool("OTEL_ENABLE_TRACING", false),
			},
		}
	}

	// 环境变量覆盖YAML配置
	if envPort := getEnvAsInt("OTEL_METRICS_PORT", 0); envPort > 0 {
		config.OpenTelemetry.MetricsPort = envPort
	}
	if envServiceName := getEnv("OTEL_SERVICE_NAME", ""); envServiceName != "" {
		config.OpenTelemetry.ServiceName = envServiceName
	}
	if envServiceVersion := getEnv("OTEL_SERVICE_VERSION", ""); envServiceVersion != "" {
		config.OpenTelemetry.ServiceVersion = envServiceVersion
	}
	if envEnvironment := getEnv("OTEL_ENVIRONMENT", ""); envEnvironment != "" {
		config.OpenTelemetry.Environment = envEnvironment
	}
	if envMetricsPath := getEnv("OTEL_METRICS_PATH", ""); envMetricsPath != "" {
		config.OpenTelemetry.MetricsPath = envMetricsPath
	}
	if envEnableTracing := getEnv("OTEL_ENABLE_TRACING", "false"); envEnableTracing != "" {
		config.OpenTelemetry.EnableTracing = getEnvAsBool("OTEL_ENABLE_TRACING", false)
	}

	return config, nil
}

// GetConnectionString 获取数据库连接字符串
func (c *Config) GetConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host,
		c.Database.Port,
		c.Database.Username,
		c.Database.Password,
		c.Database.Database,
		c.Database.SSLMode,
	)
}

// GetServerAddr 获取服务器地址
func (c *Config) GetServerAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// GetMetricsAddr 获取指标服务地址
func (c *Config) GetMetricsAddr() string {
	return fmt.Sprintf(":%d", c.OpenTelemetry.MetricsPort)
}

// 辅助函数
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
