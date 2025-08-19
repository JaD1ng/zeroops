package middleware

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
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
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// 不创建span，直接使用原始上下文
			// 这样可以避免生成trace_id和span_id，只收集指标
			ctx := r.Context()

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

			// 不设置span属性，因为不创建span
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
