package middleware

import (
	"encoding/json"
	"net/http"
	"shared/utils"
)

// writeJSONResponse 写入JSON响应
func writeJSONResponse(w http.ResponseWriter, data any) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(data)
}

// CORS 跨域中间件
func CORS() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
			
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}
			
			next.ServeHTTP(w, r)
		})
	}
}

// RequestID 请求ID中间件
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				// 可以使用UUID生成请求ID
				requestID = "req-" + generateID()
			}
			
			// 设置响应头
			w.Header().Set("X-Request-ID", requestID)
			
			// 添加到上下文
			ctx := r.Context()
			// 这里可以将requestID添加到context中
			
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// generateID 生成请求ID
func generateID() string {
	return utils.GenerateRequestID()
}