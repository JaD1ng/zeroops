package middleware

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	// HTTP请求计数器
	requestCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path"},
	)

	// HTTP请求持续时间
	requestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status_code"},
	)

	// HTTP请求大小
	requestSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_size_bytes",
			Help:    "HTTP request size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 8),
		},
		[]string{"method", "path"},
	)

	// HTTP响应大小
	responseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 8),
		},
		[]string{"method", "path", "status_code"},
	)
)

// OpenTelemetryMiddleware OpenTelemetry中间件
func OpenTelemetryMiddleware() func(http.Handler) http.Handler {
	tracer := otel.GetTracerProvider().Tracer("http-server")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// 创建span（自动包含trace_id和span_id）
			ctx, span := tracer.Start(r.Context(), r.URL.Path,
				trace.WithAttributes(
					attribute.String("http.method", r.Method),
					attribute.String("http.url", r.URL.String()),
					attribute.String("http.user_agent", r.UserAgent()),
					attribute.String("http.remote_addr", r.RemoteAddr),
				))
			defer span.End()

			// 包装ResponseWriter
			wrappedWriter := &responseWriter{ResponseWriter: w, statusCode: 200}

			// 记录请求指标
			requestCounter.WithLabelValues(r.Method, r.URL.Path).Inc()

			// 记录请求大小
			if r.ContentLength > 0 {
				requestSize.WithLabelValues(r.Method, r.URL.Path).Observe(float64(r.ContentLength))
			}

			// 调用下一个处理器
			next.ServeHTTP(wrappedWriter, r.WithContext(ctx))

			// 记录请求指标
			duration := time.Since(start)
			requestDuration.WithLabelValues(r.Method, r.URL.Path, string(rune(wrappedWriter.statusCode))).Observe(duration.Seconds())

			// 记录响应大小
			if wrappedWriter.size > 0 {
				responseSize.WithLabelValues(r.Method, r.URL.Path, string(rune(wrappedWriter.statusCode))).Observe(float64(wrappedWriter.size))
			}

			// 设置span属性
			span.SetAttributes(
				attribute.Int("http.status_code", wrappedWriter.statusCode),
				attribute.Int64("http.response_size", wrappedWriter.size),
				attribute.Int64("http.request_duration_ms", duration.Milliseconds()),
			)

			// 根据状态码设置span状态
			// if wrappedWriter.statusCode >= 400 {
			// 	span.SetStatus(trace.StatusError, "HTTP error")
			// }
		})
	}
}

// responseWriter 包装ResponseWriter
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(data []byte) (int, error) {
	rw.size += int64(len(data))
	return rw.ResponseWriter.Write(data)
}
