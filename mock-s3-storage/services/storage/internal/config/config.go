package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config 应用配置
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Metrics  MetricsConfig  `yaml:"metrics"`
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

// LoadConfig 加载配置
func LoadConfig() (*Config, error) {
	config := &Config{
		Server: ServerConfig{
			Host:             getEnv("HOST", "127.0.0.1"),
			Port:             getEnvAsInt("PORT", 8080),
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
		Metrics: MetricsConfig{
			ServiceName: getEnv("SERVICE_NAME", "file-storage-service"),
			ServiceVer:  getEnv("SERVICE_VERSION", "1.0.0"),
			Namespace:   getEnv("METRICS_NAMESPACE", "storage"),
			Enabled:     getEnvAsBool("METRICS_ENABLED", true),
			Port:        getEnvAsInt("METRICS_PORT", 1080),
			Path:        getEnv("METRICS_PATH", "/metrics"),
		},
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
	return fmt.Sprintf(":%d", c.Metrics.Port)
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
