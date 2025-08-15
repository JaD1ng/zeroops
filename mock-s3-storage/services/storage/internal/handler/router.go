package handler

import (
	"net/http"
	"strings"
)

// Router 路由处理器
type Router struct {
	fileHandler    *FileHandler
	faultHandler   *FaultHandler
	metricsHandler http.Handler
}

// NewRouter 创建路由处理器
func NewRouter(fileHandler *FileHandler, faultHandler *FaultHandler) *Router {
	return &Router{
		fileHandler:  fileHandler,
		faultHandler: faultHandler,
	}
}

// SetMetricsHandler 设置指标处理器
func (r *Router) SetMetricsHandler(handler http.Handler) {
	r.metricsHandler = handler
}

// ServeHTTP 实现http.Handler接口
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// 设置CORS头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	// 处理预检请求
	if req.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 路由分发
	path := req.URL.Path
	method := req.Method

	switch {
	// Prometheus指标端点
	case path == "/metrics" && method == "GET":
		if r.metricsHandler != nil {
			r.metricsHandler.ServeHTTP(w, req)
		} else {
			http.NotFound(w, req)
		}

	// 健康检查
	case path == "/api/health" && method == "GET":
		r.fileHandler.HealthCheck(w, req)

	// 文件上传
	case path == "/api/files/upload" && method == "POST":
		r.fileHandler.UploadFile(w, req)

	// 文件下载
	case strings.HasPrefix(path, "/api/files/download/") && method == "GET":
		r.fileHandler.DownloadFile(w, req)

	// 文件删除
	case strings.HasPrefix(path, "/api/files/") && strings.Count(path, "/") == 3 && method == "DELETE":
		r.fileHandler.DeleteFile(w, req)

	// 获取文件信息
	case strings.HasSuffix(path, "/info") && method == "GET":
		r.fileHandler.GetFileInfo(w, req)

	// 文件列表
	case path == "/api/files" && method == "GET":
		r.fileHandler.ListFiles(w, req)

	// ------------------------------------------
	// 故障启动
	case strings.HasPrefix(path, "/fault/start/") && method == http.MethodPost:
		r.faultHandler.StartFault(w, req)

	// 故障停止
	case strings.HasPrefix(path, "/fault/stop/") && method == http.MethodPost:
		r.faultHandler.StopFault(w, req)

	// 故障状态
	case strings.HasPrefix(path, "/fault/status/") && method == http.MethodGet:
		r.faultHandler.GetFaultStatus(w, req)

	// 故障列表
	case path == "/fault/list" && method == http.MethodGet:
		r.faultHandler.ListFaults(w, req)

	// 默认路由
	default:
		http.NotFound(w, req)
	}
}
