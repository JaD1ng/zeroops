package observability

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// LatencyInjector 延迟注入器接口
type LatencyInjector interface {
	InjectLatency(ctx context.Context, path string) time.Duration
}

// HTTPMiddleware HTTP监控中间件
type HTTPMiddleware struct {
	collector       *MetricCollector
	logger          *Logger
	latencyInjector LatencyInjector
}

// NewHTTPMiddleware 创建HTTP中间件
func NewHTTPMiddleware(collector *MetricCollector, logger *Logger) *HTTPMiddleware {
	return &HTTPMiddleware{
		collector: collector,
		logger:    logger,
	}
}

// SetLatencyInjector 设置延迟注入器
func (m *HTTPMiddleware) SetLatencyInjector(injector LatencyInjector) {
	m.latencyInjector = injector
}

// GinMetricsMiddleware Gin指标中间件
func (m *HTTPMiddleware) GinMetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取请求路径
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		// 在请求处理前注入延迟（如果配置了延迟注入器）
		var injectedLatency time.Duration
		if m.latencyInjector != nil {
			injectedLatency = m.latencyInjector.InjectLatency(c.Request.Context(), path)
		}

		start := time.Now()

		// 处理请求
		c.Next()

		// 计算请求时延（包含注入的延迟）
		duration := time.Since(start)
		statusCode := c.Writer.Status()

		// 记录 HTTP 请求时延指标（以秒为单位）
		if m.collector != nil {
			durationSeconds := duration.Seconds()
			m.collector.RecordHTTPRequestDuration(
				c.Request.Context(),
				durationSeconds,
				c.Request.Method,
				path,
				statusCode,
			)
		}

		// 只记录错误请求的日志
		if statusCode >= 400 {
			m.logger.Warn(c.Request.Context(), "HTTP request completed with error",
				String("method", c.Request.Method),
				String("path", path),
				Int("status", statusCode),
				Duration("duration", duration),
				Duration("injected_latency", injectedLatency),
			)
		}

		// 记录请求信息（如果有注入延迟，记录在日志中）
		if injectedLatency > 0 {
			m.logger.Info(c.Request.Context(), "HTTP request completed with injected latency",
				String("method", c.Request.Method),
				String("path", path),
				Int("status", statusCode),
				Duration("duration", duration),
				Duration("injected_latency", injectedLatency),
			)
		} else {
			m.logger.Info(c.Request.Context(), "HTTP request completed",
				String("method", c.Request.Method),
				String("path", path),
				Int("status", statusCode),
				Duration("duration", duration),
			)
		}
	}
}

// GinTracingMiddleware Gin追踪中间件
func (m *HTTPMiddleware) GinTracingMiddleware(serviceName string) gin.HandlerFunc {
	return otelgin.Middleware(serviceName)
}
