package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/rs/zerolog/log"
)

type Config struct {
	Server   ServerConfig   `json:"server"`
	Database DatabaseConfig `json:"database"`
	Logging  LoggingConfig  `json:"logging"`
	Redis    RedisConfig    `json:"redis"`
	Alerting AlertingConfig `json:"alerting"`
}

type ServerConfig struct {
	BindAddr string `json:"bindAddr"`
}

type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
	SSLMode  string `json:"sslmode"`
}

type LoggingConfig struct {
	Level string `json:"level"`
}

type RedisConfig struct {
	Addr     string `json:"addr"`
	Password string `json:"password"`
	DB       int    `json:"db"`
}

type AlertingConfig struct {
	Healthcheck HealthcheckConfig `json:"healthcheck"`
	Remediation RemediationConfig `json:"remediation"`
	Prometheus  PrometheusConfig  `json:"prometheus"`
	Ruleset     RulesetConfig     `json:"ruleset"`
	Receiver    ReceiverConfig    `json:"receiver"`
}

type HealthcheckConfig struct {
	Interval      string `json:"interval"` // e.g. "10s"
	Batch         int    `json:"batch"`
	Workers       int    `json:"workers"`
	AlertChanSize int    `json:"alertChanSize"`
}

type RemediationConfig struct {
	RollbackSleep string `json:"rollbackSleep"` // e.g. "30s"
	RollbackURL   string `json:"rollbackURL"`   // fmt template, optional
}

type PrometheusConfig struct {
	URL               string `json:"url"`
	QueryTimeout      string `json:"queryTimeout"`
	AnomalyAPIURL     string `json:"anomalyAPIUrl"`
	AnomalyAPITimeout string `json:"anomalyAPITimeout"`
	SchedulerInterval string `json:"schedulerInterval"`
	QueryStep         string `json:"queryStep"`
	QueryRange        string `json:"queryRange"`
}

type RulesetConfig struct {
	ConfigFile string `json:"configFile"`
	APIBase    string `json:"apiBase"`
	APITimeout string `json:"apiTimeout"`
}

type ReceiverConfig struct {
	BasicUser string `json:"basicUser"`
	BasicPass string `json:"basicPass"`
	Bearer    string `json:"bearer"`
}

func Load() (*Config, error) {
	configFile := flag.String("f", "", "Path to configuration file")
	flag.Parse()

	cfg := &Config{
		Server: ServerConfig{
			BindAddr: getEnv("SERVER_BIND_ADDR", "0.0.0.0:8080"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "admin"),
			Password: getEnv("DB_PASSWORD", "password"),
			DBName:   getEnv("DB_NAME", "zeroops"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Logging: LoggingConfig{
			Level: getEnv("LOG_LEVEL", "debug"),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		Alerting: AlertingConfig{
			Healthcheck: HealthcheckConfig{
				Interval:      getEnv("HC_SCAN_INTERVAL", "10s"),
				Batch:         getEnvInt("HC_SCAN_BATCH", 200),
				Workers:       getEnvInt("HC_WORKERS", 1),
				AlertChanSize: getEnvInt("REMEDIATION_ALERT_CHAN_SIZE", 1024),
			},
			Remediation: RemediationConfig{
				RollbackSleep: getEnv("REMEDIATION_ROLLBACK_SLEEP", "30s"),
				RollbackURL:   getEnv("REMEDIATION_ROLLBACK_URL", ""),
			},
			Prometheus: PrometheusConfig{
				URL:               getEnv("PROMETHEUS_URL", "http://localhost:9090"),
				QueryTimeout:      getEnv("PROMETHEUS_QUERY_TIMEOUT", "30s"),
				AnomalyAPIURL:     getEnv("ANOMALY_DETECTION_API_URL", "http://localhost:8081/api/v1/anomaly/detect"),
				AnomalyAPITimeout: getEnv("ANOMALY_DETECTION_API_TIMEOUT", "10s"),
				SchedulerInterval: getEnv("PROMETHEUS_ANOMALY_INTERVAL", "6h"),
				QueryStep:         getEnv("PROMETHEUS_QUERY_STEP", "1m"),
				QueryRange:        getEnv("PROMETHEUS_QUERY_RANGE", "6h"),
			},
			Ruleset: RulesetConfig{
				ConfigFile: getEnv("ALERT_RULES_CONFIG_FILE", ""),
				APIBase:    getEnv("RULESET_API_BASE", ""),
				APITimeout: getEnv("RULESET_API_TIMEOUT", "10s"),
			},
			Receiver: ReceiverConfig{
				BasicUser: getEnv("ALERT_WEBHOOK_BASIC_USER", ""),
				BasicPass: getEnv("ALERT_WEBHOOK_BASIC_PASS", ""),
				Bearer:    getEnv("ALERT_WEBHOOK_BEARER", ""),
			},
		},
	}

	if *configFile != "" {
		if err := loadFromFile(cfg, *configFile); err != nil {
			log.Err(err)
			return nil, err
		}
	}

	// fill reasonable defaults when fields omitted in file
	if cfg.Server.BindAddr == "" {
		cfg.Server.BindAddr = "0.0.0.0:8080"
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "debug"
	}
	if cfg.Redis.Addr == "" {
		cfg.Redis.Addr = "localhost:6379"
	}
	if cfg.Alerting.Healthcheck.Interval == "" {
		cfg.Alerting.Healthcheck.Interval = "10s"
	}
	if cfg.Alerting.Healthcheck.Batch == 0 {
		cfg.Alerting.Healthcheck.Batch = 200
	}
	if cfg.Alerting.Healthcheck.Workers == 0 {
		cfg.Alerting.Healthcheck.Workers = 1
	}
	if cfg.Alerting.Healthcheck.AlertChanSize == 0 {
		cfg.Alerting.Healthcheck.AlertChanSize = 1024
	}
	if cfg.Alerting.Prometheus.QueryTimeout == "" {
		cfg.Alerting.Prometheus.QueryTimeout = "30s"
	}
	if cfg.Alerting.Prometheus.SchedulerInterval == "" {
		cfg.Alerting.Prometheus.SchedulerInterval = "6h"
	}
	if cfg.Alerting.Prometheus.QueryStep == "" {
		cfg.Alerting.Prometheus.QueryStep = "1m"
	}
	if cfg.Alerting.Prometheus.QueryRange == "" {
		cfg.Alerting.Prometheus.QueryRange = "6h"
	}
	if cfg.Alerting.Ruleset.APITimeout == "" {
		cfg.Alerting.Ruleset.APITimeout = "10s"
	}

	return cfg, nil
}

func loadFromFile(cfg *Config, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", filePath, err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("failed to parse config file %s: %w", filePath, err)
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
