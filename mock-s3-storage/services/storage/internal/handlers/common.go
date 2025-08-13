package handlers

import (
	"encoding/json"
	"net/http"
)

// writeJSONResponse 写入JSON响应
func writeJSONResponse(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// writeSuccessResponse 写入成功响应
func writeSuccessResponse(w http.ResponseWriter, message string, data any) {
	response := map[string]any{
		"success": true,
		"message": message,
		"data":    data,
	}
	writeJSONResponse(w, http.StatusOK, response)
}

// writeErrorResponse 写入错误响应
func writeErrorResponse(w http.ResponseWriter, statusCode int, errorCode, message string) {
	response := map[string]any{
		"success":    false,
		"error":      message,
		"error_code": errorCode,
	}
	writeJSONResponse(w, statusCode, response)
}

// Router 创建路由处理器
func NewRouter(filesHandler *FilesHandler, healthHandler *HealthHandler) http.Handler {
	mux := http.NewServeMux()

	// 健康检查路由
	mux.HandleFunc("/api/health", healthHandler.HealthCheck)
	mux.HandleFunc("/api/ready", healthHandler.ReadinessCheck)
	mux.HandleFunc("/api/live", healthHandler.LivenessCheck)

	// 文件操作路由
	mux.HandleFunc("/api/files/upload", filesHandler.UploadFile)
	mux.HandleFunc("/api/files/download/", filesHandler.DownloadFile)
	mux.HandleFunc("/api/files", handleFilesRoute(filesHandler))

	return mux
}

// handleFilesRoute 处理文件相关的多种路由
func handleFilesRoute(handler *FilesHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		switch {
		case path == "/api/files":
			// GET /api/files - 列出所有文件
			handler.ListFiles(w, r)
		case matchPattern(path, "/api/files/*/info"):
			// GET /api/files/{fileID}/info - 获取文件信息
			handler.GetFileInfo(w, r)
		case matchPattern(path, "/api/files/*") && r.Method == http.MethodDelete:
			// DELETE /api/files/{fileID} - 删除文件
			handler.DeleteFile(w, r)
		default:
			writeErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "路由不存在")
		}
	}
}

// matchPattern 简单的路径模式匹配
func matchPattern(path, pattern string) bool {
	// 简化的实现，实际项目中可能需要更复杂的路由匹配
	// 这里只匹配 /api/files/* 和 /api/files/*/info 模式
	if pattern == "/api/files/*" {
		return len(path) > len("/api/files/") && path[:len("/api/files/")] == "/api/files/"
	}
	if pattern == "/api/files/*/info" {
		return len(path) > len("/api/files/") && 
			   path[:len("/api/files/")] == "/api/files/" &&
			   path[len(path)-5:] == "/info"
	}
	return false
}