package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// RequestIDKey 请求ID的上下文键
const RequestIDKey = "request_id"

// RequestIDMiddleware 生成请求ID的中间件
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 从请求头中获取request_id，如果没有则生成新的
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		// 将request_id添加到响应头中
		w.Header().Set("X-Request-ID", requestID)

		// 将request_id添加到上下文中
		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// generateRequestID 生成请求ID
func generateRequestID() string {
	return uuid.New().String()
}

// GetRequestID 从上下文中获取请求ID
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// WithRequestID 为上下文添加请求ID
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}
