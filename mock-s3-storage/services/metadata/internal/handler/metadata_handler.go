package handler

import (
	"encoding/json"
	"metadata/internal/model"
	"metadata/internal/service"
	"net/http"
	"shared/faults"
	logs "shared/telemetry/logger"
	"shared/telemetry/metrics"
	"strconv"
	"strings"
)

type MetadataHandler struct {
	metadataService service.MetadataService
	logger          logs.Logger
	metrics         metrics.Metrics
}

func NewMetadataHandler(metadataService service.MetadataService, logger logs.Logger, metrics metrics.Metrics) *MetadataHandler {
	return &MetadataHandler{
		metadataService: metadataService,
		logger:          logger,
		metrics:         metrics,
	}
}

// SaveMetadataRequest 保存元数据请求
type SaveMetadataRequest struct {
	ID           string   `json:"id"`
	Key          string   `json:"key"`
	Size         int64    `json:"size"`
	ContentType  string   `json:"content_type"`
	MD5Hash      string   `json:"md5_hash"`
	StorageNodes []string `json:"storage_nodes"`
}

// UpdateMetadataRequest 更新元数据请求
type UpdateMetadataRequest struct {
	ContentType  string   `json:"content_type,omitempty"`
	StorageNodes []string `json:"storage_nodes,omitempty"`
}

// SaveMetadata 保存元数据（由上层文件上传服务调用）
func (h *MetadataHandler) SaveMetadata(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h.metrics.IncCounter("metadata_save_requests")

	var req SaveMetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error(ctx, "解析保存元数据请求失败", err, nil)
		h.writeErrorResponse(w, faults.NewError(faults.ErrorTypeBadRequest, "INVALID_REQUEST", "Invalid JSON request body"))
		return
	}

	entry := model.NewMetadataEntry(req.ID, req.Key, req.Size, req.ContentType, req.MD5Hash, req.StorageNodes)
	
	err := h.metadataService.SaveMetadata(ctx, entry)
	if err != nil {
		h.writeErrorResponse(w, err)
		return
	}

	h.writeJSONResponse(w, http.StatusCreated, map[string]any{
		"success": true,
		"message": "Metadata saved successfully",
		"key":     req.Key,
	})
}

// GetMetadata 获取元数据
func (h *MetadataHandler) GetMetadata(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := r.URL.Query().Get("key")
	
	if key == "" {
		h.writeErrorResponse(w, faults.NewError(faults.ErrorTypeBadRequest, "MISSING_KEY", "Key parameter is required"))
		return
	}

	entry, err := h.metadataService.GetMetadata(ctx, key)
	if err != nil {
		h.writeErrorResponse(w, err)
		return
	}

	h.writeJSONResponse(w, http.StatusOK, entry)
}

// DeleteMetadata 删除元数据
func (h *MetadataHandler) DeleteMetadata(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := r.URL.Query().Get("key")
	
	if key == "" {
		h.writeErrorResponse(w, faults.NewError(faults.ErrorTypeBadRequest, "MISSING_KEY", "Key parameter is required"))
		return
	}

	err := h.metadataService.DeleteMetadata(ctx, key)
	if err != nil {
		h.writeErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListMetadata 列出元数据
func (h *MetadataHandler) ListMetadata(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	limit := 100
	offset := 0
	
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	entries, err := h.metadataService.ListMetadata(ctx, limit, offset)
	if err != nil {
		h.writeErrorResponse(w, err)
		return
	}

	response := map[string]any{
		"entries": entries,
		"limit":   limit,
		"offset":  offset,
		"count":   len(entries),
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// UpdateMetadata 更新元数据
func (h *MetadataHandler) UpdateMetadata(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := r.URL.Query().Get("key")
	
	if key == "" {
		h.writeErrorResponse(w, faults.NewError(faults.ErrorTypeBadRequest, "MISSING_KEY", "Key parameter is required"))
		return
	}

	var req UpdateMetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error(ctx, "解析更新元数据请求失败", err, nil)
		h.writeErrorResponse(w, faults.NewError(faults.ErrorTypeBadRequest, "INVALID_REQUEST", "Invalid JSON request body"))
		return
	}

	updates := make(map[string]any)
	if req.ContentType != "" {
		updates["content_type"] = req.ContentType
	}
	if len(req.StorageNodes) > 0 {
		updates["storage_nodes"] = req.StorageNodes
	}

	err := h.metadataService.UpdateMetadata(ctx, key, updates)
	if err != nil {
		h.writeErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetStats 获取统计信息
func (h *MetadataHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stats, err := h.metadataService.GetStats(ctx)
	if err != nil {
		h.writeErrorResponse(w, err)
		return
	}

	h.writeJSONResponse(w, http.StatusOK, stats)
}

// SearchMetadata 搜索元数据
func (h *MetadataHandler) SearchMetadata(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query().Get("q")
	
	if query == "" {
		h.writeErrorResponse(w, faults.NewError(faults.ErrorTypeBadRequest, "MISSING_QUERY", "Query parameter 'q' is required"))
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}

	entries, err := h.metadataService.SearchMetadata(ctx, query, limit)
	if err != nil {
		h.writeErrorResponse(w, err)
		return
	}

	response := map[string]any{
		"query":   query,
		"results": entries,
		"total":   len(entries),
		"limit":   limit,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// GetMetadataByPattern 根据模式获取元数据
func (h *MetadataHandler) GetMetadataByPattern(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pattern := r.URL.Query().Get("pattern")
	
	if pattern == "" {
		h.writeErrorResponse(w, faults.NewError(faults.ErrorTypeBadRequest, "MISSING_PATTERN", "Pattern parameter is required"))
		return
	}

	entries, err := h.metadataService.GetMetadataByPattern(ctx, pattern)
	if err != nil {
		h.writeErrorResponse(w, err)
		return
	}

	response := map[string]any{
		"pattern": pattern,
		"results": entries,
		"total":   len(entries),
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// ExportMetadata 导出元数据
func (h *MetadataHandler) ExportMetadata(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	keysParam := r.URL.Query().Get("keys")
	var keys []string
	if keysParam != "" {
		keys = strings.Split(keysParam, ",")
	}

	data, err := h.metadataService.ExportMetadata(ctx, keys)
	if err != nil {
		h.writeErrorResponse(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=metadata-export.json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// ImportMetadata 导入元数据
func (h *MetadataHandler) ImportMetadata(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 读取请求体
	data := make([]byte, r.ContentLength)
	_, err := r.Body.Read(data)
	if err != nil {
		h.writeErrorResponse(w, faults.NewError(faults.ErrorTypeBadRequest, "INVALID_DATA", "Failed to read request body"))
		return
	}

	err = h.metadataService.ImportMetadata(ctx, data)
	if err != nil {
		h.writeErrorResponse(w, err)
		return
	}

	h.writeJSONResponse(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Metadata imported successfully",
	})
}

// ListByBucket 按bucket列出对象（兼容性接口）
func (h *MetadataHandler) ListByBucket(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := r.URL.Query().Get("bucket")
	prefix := r.URL.Query().Get("prefix")
	
	if bucket == "" {
		h.writeErrorResponse(w, faults.NewError(faults.ErrorTypeBadRequest, "MISSING_BUCKET", "Bucket parameter is required"))
		return
	}

	// 构建搜索模式：bucket/prefix*
	pattern := bucket + "/"
	if prefix != "" {
		pattern += prefix + "*"
	} else {
		pattern += "*"
	}

	entries, err := h.metadataService.GetMetadataByPattern(ctx, pattern)
	if err != nil {
		h.writeErrorResponse(w, err)
		return
	}

	// 转换为S3兼容格式
	var objects []map[string]any
	for _, entry := range entries {
		objectKey := entry.ExtractObjectKey()
		objects = append(objects, map[string]any{
			"Key":          objectKey,
			"Size":         entry.Size,
			"LastModified": entry.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
			"ETag":         `"` + entry.MD5Hash + `"`,
			"StorageClass": "STANDARD",
		})
	}

	response := map[string]any{
		"Name":           bucket,
		"Prefix":         prefix,
		"Objects":        objects,
		"TotalCount":     len(objects),
		"IsTruncated":    false,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *MetadataHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func (h *MetadataHandler) writeErrorResponse(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*faults.AppError); ok {
		response := map[string]any{
			"error": map[string]any{
				"type":    appErr.Type,
				"code":    appErr.Code,
				"message": appErr.Message,
			},
		}
		h.writeJSONResponse(w, appErr.HTTPStatusCode(), response)
		h.metrics.IncCounter("http_errors", "code", strconv.Itoa(appErr.HTTPStatusCode()))
	} else {
		response := map[string]any{
			"error": map[string]any{
				"type":    "INTERNAL_ERROR",
				"code":    "UNKNOWN",
				"message": err.Error(),
			},
		}
		h.writeJSONResponse(w, http.StatusInternalServerError, response)
		h.metrics.IncCounter("http_errors", "code", "500")
	}
}