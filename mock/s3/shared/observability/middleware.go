package observability

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// HTTPMiddleware HTTP监控中间件
type HTTPMiddleware struct {
	collector *MetricCollector
	logger    *Logger
}

// NewHTTPMiddleware 创建HTTP中间件
func NewHTTPMiddleware(collector *MetricCollector, logger *Logger) *HTTPMiddleware {
	return &HTTPMiddleware{
		collector: collector,
		logger:    logger,
	}
}

// GinMetricsMiddleware Gin指标中间件
func (m *HTTPMiddleware) GinMetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 处理请求
		c.Next()

		// 计算请求时延
		duration := time.Since(start)
		statusCode := c.Writer.Status()

		// 记录 HTTP 请求时延指标（以秒为单位）
		if m.collector != nil {
			durationSeconds := duration.Seconds()
			path := c.FullPath()
			if path == "" {
				path = c.Request.URL.Path // 如果没有匹配的路由，使用原始路径
			}
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
				String("path", c.FullPath()),
				Int("status", statusCode),
				Duration("duration", duration),
			)
		}

		m.logger.Info(c.Request.Context(), "HTTP request completed",
			String("method", c.Request.Method),
			String("path", c.FullPath()),
			Int("status", statusCode),
			Duration("duration", duration),
		)
	}
}

// GinTracingMiddleware Gin追踪中间件
func (m *HTTPMiddleware) GinTracingMiddleware(serviceName string) gin.HandlerFunc {
	return otelgin.Middleware(serviceName)
}
