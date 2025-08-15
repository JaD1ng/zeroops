package logger

import (
	"context"
	"shared/config"
)

// Logger 简化的日志接口
type Logger interface {
	Info(ctx context.Context, message string, fields map[string]any)
	Error(ctx context.Context, message string, err error, fields map[string]any)
	Debug(ctx context.Context, message string, fields map[string]any)
	Warn(ctx context.Context, message string, fields map[string]any)
}

// NewLogger 创建日志器
func NewLogger(config config.LoggingConfig) Logger {
	// 如果启用了Elasticsearch，创建Elasticsearch Logger
	if config.Elasticsearch.Enabled {
		return NewElasticsearchLogger(config.Elasticsearch, "storage-service")
	}

	// 否则返回nil（可以后续添加其他实现）
	return nil
}
