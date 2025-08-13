package middleware

import (
	"net/http"
	"shared/telemetry/metrics"
	"strconv"
	"time"
)

// MetricsMiddleware 指标收集中间件
func MetricsMiddleware(m metrics.Metrics) func(http.Handler) http.Handler {
	if m == nil {
		// 如果没有指标收集器，返回无操作中间件
		return func(next http.Handler) http.Handler {
			return next
		}
	}
	
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// 创建响应记录器来捕获状态码
			recorder := &responseRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}
			
			// 处理请求
			next.ServeHTTP(recorder, r)
			
			// 记录指标
			duration := time.Since(start)
			method := r.Method
			endpoint := r.URL.Path
			status := strconv.Itoa(recorder.statusCode)
			
			// 记录HTTP请求指标
			m.RecordHTTPRequest(method, endpoint, status, duration)
			
			// 增加请求计数器
			m.IncCounter("http_requests_total", 
				"method", method,
				"endpoint", endpoint, 
				"status", status,
			)
			
			// 记录请求延迟直方图
			m.ObserveHistogram("http_request_duration_seconds", 
				duration.Seconds(),
				"method", method,
				"endpoint", endpoint,
			)
		})
	}
}

// responseRecorder 用于捕获HTTP响应状态码
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}