package handlers

import (
	"net/http"
	"time"
)

// HealthHandler 健康检查处理器
type HealthHandler struct{}

// NewHealthHandler 创建健康检查处理器
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// HealthCheck 健康检查接口
// GET /api/health
func (h *HealthHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "只支持GET方法")
		return
	}

	response := map[string]any{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"service":   "storage-service",
		"version":   "1.0.0",
	}

	writeJSONResponse(w, http.StatusOK, response)
}

// ReadinessCheck 就绪检查接口
// GET /api/ready
func (h *HealthHandler) ReadinessCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "只支持GET方法")
		return
	}

	// 这里可以检查依赖服务的状态，比如数据库连接
	// 暂时简单返回就绪状态
	response := map[string]any{
		"status":    "ready",
		"timestamp": time.Now().Format(time.RFC3339),
		"service":   "storage-service",
		"checks": map[string]string{
			"database": "ok",
			"storage":  "ok",
		},
	}

	writeJSONResponse(w, http.StatusOK, response)
}

// LivenessCheck 存活检查接口
// GET /api/live
func (h *HealthHandler) LivenessCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "只支持GET方法")
		return
	}

	response := map[string]any{
		"status":    "alive",
		"timestamp": time.Now().Format(time.RFC3339),
		"service":   "storage-service",
	}

	writeJSONResponse(w, http.StatusOK, response)
}