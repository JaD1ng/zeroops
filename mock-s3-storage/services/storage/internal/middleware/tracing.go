package middleware

import (
	"net/http"
	"shared/telemetry/tracing"
)

// TracingMiddleware 链路追踪中间件
func TracingMiddleware(tracer tracing.Tracer) func(http.Handler) http.Handler {
	if tracer == nil {
		// 如果没有追踪器，返回无操作中间件
		return func(next http.Handler) http.Handler {
			return next
		}
	}
	
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 从HTTP头中提取追踪上下文
			ctx := tracer.Extract(r.Context(), r.Header)
			
			// 开始新的span
			spanName := r.Method + " " + r.URL.Path
			ctx, span := tracer.StartSpan(ctx, spanName)
			defer span.End()
			
			// 设置span属性
			span.SetAttribute("http.method", r.Method)
			span.SetAttribute("http.url", r.URL.String())
			span.SetAttribute("http.scheme", r.URL.Scheme)
			span.SetAttribute("http.host", r.Host)
			span.SetAttribute("http.target", r.URL.Path)
			span.SetAttribute("http.user_agent", r.UserAgent())
			span.SetAttribute("http.remote_addr", r.RemoteAddr)
			
			// 创建响应记录器来捕获状态码
			recorder := &tracingResponseRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}
			
			// 注入追踪上下文到响应头
			if err := tracer.Inject(ctx, w.Header()); err != nil {
				span.SetError(err)
			}
			
			// 使用新的上下文处理请求
			r = r.WithContext(ctx)
			next.ServeHTTP(recorder, r)
			
			// 设置响应相关的span属性
			span.SetAttribute("http.status_code", recorder.statusCode)
			span.SetAttribute("http.response.size", recorder.bytesWritten)
			
			// 如果是错误状态码，标记span为错误
			if recorder.statusCode >= 400 {
				span.SetAttribute("error", true)
			}
		})
	}
}

// tracingResponseRecorder 用于捕获HTTP响应信息
type tracingResponseRecorder struct {
	http.ResponseWriter
	statusCode    int
	bytesWritten  int64
}

func (r *tracingResponseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *tracingResponseRecorder) Write(data []byte) (int, error) {
	n, err := r.ResponseWriter.Write(data)
	r.bytesWritten += int64(n)
	return n, err
}