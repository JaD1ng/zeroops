package observability

import (
	"context"
	"fmt"
	"mocks3/shared/observability/config"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	otrace "go.opentelemetry.io/otel/trace"

	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// Providers 统一的可观测性提供者
type Providers struct {
	config         *config.ObservabilityConfig
	resource       *resource.Resource
	logProvider    *sdklog.LoggerProvider
	metricProvider *sdkmetric.MeterProvider
	traceProvider  *trace.TracerProvider

	// 公共接口
	Logger *Logger
	Meter  metric.Meter
	Tracer otrace.Tracer
}

// NewProviders 创建统一的可观测性提供者
func NewProviders(config *config.ObservabilityConfig) (*Providers, error) {
	// 创建资源
	res, err := createResource(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	providers := &Providers{
		config:   config,
		resource: res,
	}

	// 初始化各个组件
	if err := providers.initLogProvider(); err != nil {
		return nil, fmt.Errorf("failed to init log provider: %w", err)
	}

	if err := providers.initMetricProvider(); err != nil {
		return nil, fmt.Errorf("failed to init metric provider: %w", err)
	}

	if err := providers.initTraceProvider(); err != nil {
		return nil, fmt.Errorf("failed to init trace provider: %w", err)
	}

	// 创建公共接口
	providers.Logger = NewLogger(config.ServiceName, config.LogLevel)
	providers.Meter = providers.metricProvider.Meter(config.ServiceName)
	providers.Tracer = providers.traceProvider.Tracer(config.ServiceName)

	return providers, nil
}

// initLogProvider 初始化日志提供者
func (p *Providers) initLogProvider() error {
	exporter, err := otlploghttp.New(context.Background(),
		otlploghttp.WithEndpoint(p.config.OTLPEndpoint),
		otlploghttp.WithInsecure(),
	)
	if err != nil {
		return err
	}

	p.logProvider = sdklog.NewLoggerProvider(
		sdklog.WithResource(p.resource),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
	)

	global.SetLoggerProvider(p.logProvider)
	return nil
}

// initMetricProvider 初始化指标提供者
func (p *Providers) initMetricProvider() error {
	// OTLP 导出器
	otlpExporter, err := otlpmetrichttp.New(context.Background(),
		otlpmetrichttp.WithEndpoint(p.config.OTLPEndpoint),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		return err
	}

	// Prometheus 导出器
	prometheusExporter, err := prometheus.New()
	if err != nil {
		return err
	}

	p.metricProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(p.resource),
		// OTLP 导出器用于发送到 OTEL Collector
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(otlpExporter,
			sdkmetric.WithInterval(p.config.ExportInterval),
		)),
		// Prometheus 导出器用于 /metrics 端点
		sdkmetric.WithReader(prometheusExporter),
	)

	otel.SetMeterProvider(p.metricProvider)
	return nil
}

// initTraceProvider 初始化追踪提供者
func (p *Providers) initTraceProvider() error {
	exporter, err := otlptracehttp.New(context.Background(),
		otlptracehttp.WithEndpoint(p.config.OTLPEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return err
	}

	// 创建采样器
	var sampler trace.Sampler
	if p.config.SamplingRatio <= 0 {
		sampler = trace.NeverSample()
	} else if p.config.SamplingRatio >= 1.0 {
		sampler = trace.AlwaysSample()
	} else {
		sampler = trace.TraceIDRatioBased(p.config.SamplingRatio)
	}

	p.traceProvider = trace.NewTracerProvider(
		trace.WithResource(p.resource),
		trace.WithBatcher(exporter),
		trace.WithSampler(sampler),
	)

	otel.SetTracerProvider(p.traceProvider)

	// 设置传播器
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return nil
}

// Shutdown 关闭所有提供者
func (p *Providers) Shutdown(ctx context.Context) error {
	var errs []error

	if err := p.logProvider.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("log provider shutdown: %w", err))
	}

	if err := p.metricProvider.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("metric provider shutdown: %w", err))
	}

	if err := p.traceProvider.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("trace provider shutdown: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}

// createResource 创建OTEL资源
func createResource(config *config.ObservabilityConfig) (*resource.Resource, error) {
	return resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion(config.ServiceVersion),
			semconv.DeploymentEnvironment(config.Environment),
		),
	)
}
