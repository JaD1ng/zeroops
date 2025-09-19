// Package observability 提供统一的可观测性组件
package observability

import (
	"context"
	"fmt"
	"mocks3/shared/observability/config"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Setup 设置所有可观测性组件
func Setup(serviceName string, configPath string) (*Providers, *MetricCollector, *HTTPMiddleware, error) {
	// 从配置文件加载配置
	config, err := config.LoadObservabilityConfig(serviceName, configPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	// 创建providers
	providers, err := NewProviders(config)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create providers: %w", err)
	}

	// 创建指标收集器
	collector, err := NewMetricCollector(providers.Meter, providers.Logger)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create metric collector: %w", err)
	}

	// 设置服务信息到指标收集器
	collector.SetServiceInfo(config.ServiceName, config.ServiceVersion)

	// 创建HTTP中间件
	httpMiddleware := NewHTTPMiddleware(collector, providers.Logger)

	return providers, collector, httpMiddleware, nil
}

// StartSystemMetrics 启动系统指标收集（应该在服务启动时调用）
func StartSystemMetrics(ctx context.Context, collector *MetricCollector, logger *Logger) {
	if collector != nil {
		collector.RecordSystemMetrics(ctx)

		if logger != nil {
			logger.Info(ctx, "System metrics collection started")
		}
	}
}

// SetupGinMiddlewares 设置Gin中间件
func SetupGinMiddlewares(router *gin.Engine, serviceName string, httpMiddleware *HTTPMiddleware) {
	// 添加追踪中间件
	router.Use(httpMiddleware.GinTracingMiddleware(serviceName))

	// 添加HTTP日志中间件
	router.Use(httpMiddleware.GinMetricsMiddleware())

	// 添加 Prometheus 指标端点
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

// Shutdown 优雅关闭所有组件
func Shutdown(ctx context.Context, providers *Providers) error {
	if providers != nil {
		return providers.Shutdown(ctx)
	}
	return nil
}

// GetLogger 从providers获取logger
func GetLogger(providers *Providers) *Logger {
	if providers != nil {
		return providers.Logger
	}
	return nil
}

// GetCollector 从setup结果获取collector
func GetCollector(collector *MetricCollector) *MetricCollector {
	return collector
}
