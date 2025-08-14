package handler

import (
	"net/http"
	"shared/httpserver"
)

func RegisterRoutes(server httpserver.Server, metadataHandler *MetadataHandler) {
	// Core metadata operations
	server.AddHandlerFunc("/metadata", handleMethodRouter(map[string]http.HandlerFunc{
		http.MethodPost: metadataHandler.SaveMetadata,  // 保存元数据（由上层服务调用）
		http.MethodGet:  metadataHandler.ListMetadata,  // 列出所有元数据
	}))
	
	server.AddHandlerFunc("/metadata/get", handleMethodRouter(map[string]http.HandlerFunc{
		http.MethodGet: metadataHandler.GetMetadata,    // 获取单个元数据
	}))
	
	server.AddHandlerFunc("/metadata/delete", handleMethodRouter(map[string]http.HandlerFunc{
		http.MethodDelete: metadataHandler.DeleteMetadata, // 删除元数据
	}))
	
	server.AddHandlerFunc("/metadata/update", handleMethodRouter(map[string]http.HandlerFunc{
		http.MethodPut: metadataHandler.UpdateMetadata,    // 更新元数据
	}))

	// Search and query operations
	server.AddHandlerFunc("/metadata/search", handleMethodRouter(map[string]http.HandlerFunc{
		http.MethodGet: metadataHandler.SearchMetadata,    // 搜索元数据
	}))
	
	server.AddHandlerFunc("/metadata/pattern", handleMethodRouter(map[string]http.HandlerFunc{
		http.MethodGet: metadataHandler.GetMetadataByPattern, // 模式匹配
	}))

	// Import/Export operations
	server.AddHandlerFunc("/metadata/export", handleMethodRouter(map[string]http.HandlerFunc{
		http.MethodGet: metadataHandler.ExportMetadata,    // 导出元数据
	}))
	
	server.AddHandlerFunc("/metadata/import", handleMethodRouter(map[string]http.HandlerFunc{
		http.MethodPost: metadataHandler.ImportMetadata,   // 导入元数据
	}))

	// Statistics
	server.AddHandlerFunc("/metadata/stats", handleMethodRouter(map[string]http.HandlerFunc{
		http.MethodGet: metadataHandler.GetStats,          // 获取统计信息
	}))

	// S3 compatibility endpoints
	server.AddHandlerFunc("/buckets", handleMethodRouter(map[string]http.HandlerFunc{
		http.MethodGet: metadataHandler.ListByBucket,      // 按bucket列出对象
	}))

	// Health check
	server.AddHandlerFunc("/health", handleMethodRouter(map[string]http.HandlerFunc{
		http.MethodGet: healthCheck,
	}))
}

func handleMethodRouter(methodHandlers map[string]http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if handler, ok := methodHandlers[r.Method]; ok {
			handler(w, r)
		} else {
			w.Header().Set("Allow", getAllowedMethods(methodHandlers))
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	}
}

func getAllowedMethods(methodHandlers map[string]http.HandlerFunc) string {
	methods := ""
	for method := range methodHandlers {
		if methods != "" {
			methods += ", "
		}
		methods += method
	}
	return methods
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy","service":"metadata"}`))
}
