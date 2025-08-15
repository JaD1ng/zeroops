package opentelemetry

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// Config OpenTelemetry配置
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	MetricsPort    int
	MetricsPath    string
	EnableTracing  bool // 是否启用追踪
}

// TelemetryProvider OpenTelemetry提供者
type TelemetryProvider struct {
	config           Config
	metricsCollector *MetricsCollector
	logger           *Logger
}

// InitOpenTelemetry 初始化OpenTelemetry
func InitOpenTelemetry(config Config) (*TelemetryProvider, error) {
	// 创建资源
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion(config.ServiceVersion),
			semconv.DeploymentEnvironment(config.Environment),
		),
	)
	if err != nil {
		return nil, err
	}

	// 初始化Tracer（可选）
	if config.EnableTracing {
		if err := initTracer(res); err != nil {
			return nil, err
		}
	}

	// 创建指标收集器
	metricsCollector := NewMetricsCollector(config)

	// 创建日志记录器
	logger := NewLogger(config)

	// 创建提供者
	provider := &TelemetryProvider{
		config:           config,
		metricsCollector: metricsCollector,
		logger:           logger,
	}

	return provider, nil
}

// GetMetricsHandler 获取指标HTTP处理器
func (tp *TelemetryProvider) GetMetricsHandler() http.Handler {
	return promhttp.Handler()
}

// GetMetricsCollector 获取指标收集器
func (tp *TelemetryProvider) GetMetricsCollector() *MetricsCollector {
	return tp.metricsCollector
}

// GetLogger 获取日志记录器
func (tp *TelemetryProvider) GetLogger() *Logger {
	return tp.logger
}

// initTracer 初始化追踪器
func initTracer(res *resource.Resource) error {
	// 创建stdout导出器（禁用pretty print以减少输出）
	exporter, err := stdouttrace.New(
	// stdouttrace.WithPrettyPrint(), // 注释掉pretty print
	)
	if err != nil {
		return err
	}

	// 创建TracerProvider
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter,
			trace.WithBatchTimeout(5*time.Second),
		),
		trace.WithResource(res),
	)

	// 设置全局TracerProvider
	otel.SetTracerProvider(tp)

	return nil
}
