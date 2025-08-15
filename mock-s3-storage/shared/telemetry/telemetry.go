package telemetry

import (
	"context"
	"shared/telemetry/logger"
	"shared/telemetry/metrics"
)

// TelemetryProvider 创建抽象接口，为后续迁移做准备
type TelemetryProvider interface {
	Log(ctx context.Context, level, message string, fields map[string]any)
	Metric(name string, value float64, labels map[string]string)
	Trace(ctx context.Context, name string, fn func(context.Context))
}

// CurrentTelemetryProvider 基于现有Elasticsearch + Prometheus的实现
type CurrentTelemetryProvider struct {
	logger  logger.Logger
	metrics metrics.Metrics
}
