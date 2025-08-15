package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"shared/middleware"
	"strings"
	"time"

	"github.com/google/uuid"
	"storage-service/internal/service"
)

// FileHandler 文件处理器
type FileHandler struct {
	storageService service.StorageService
}

// NewFileHandler 创建文件处理器
func NewFileHandler(storageService service.StorageService) *FileHandler {
	return &FileHandler{
		storageService: storageService,
	}
}

// UploadFile 处理文件上传请求
// POST /api/files/upload
func (h *FileHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 获取request_id（测试用）
	requestID := middleware.GetRequestID(r.Context())
	log.Printf("处理文件上传请求，request_id: %s", requestID)

	// 检查请求方法
	if r.Method != http.MethodPost {
		http.Error(w, "只支持POST方法", http.StatusMethodNotAllowed)
		return
	}

	// 解析multipart表单
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB限制
		http.Error(w, fmt.Sprintf("解析表单失败: %v", err), http.StatusBadRequest)
		return
	}

	// 获取上传的文件
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, fmt.Sprintf("获取文件失败: %v", err), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 检查文件类型（只允许文本文件）
	contentType := header.Header.Get("Content-Type")
	if !isTextFile(contentType) {
		http.Error(w, "只支持文本文件上传", http.StatusBadRequest)
		return
	}

	// 生成文件ID
	fileID := uuid.New().String()

	// 获取文件扩展名
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".txt" // 默认扩展名
	}

	// 生成新的文件名
	fileName := fileID + ext

	// 上传文件到存储服务
	fileInfo, err := h.storageService.UploadFile(r.Context(), fileID, fileName, contentType, file)
	if err != nil {
		http.Error(w, fmt.Sprintf("上传文件失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 返回成功响应
	response := map[string]interface{}{
		"success": true,
		"message": "文件上传成功",
		"data":    fileInfo,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// DownloadFile 处理文件下载请求
// GET /api/files/download/{fileID}
func (h *FileHandler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	// 检查请求方法
	if r.Method != http.MethodGet {
		http.Error(w, "只支持GET方法", http.StatusMethodNotAllowed)
		return
	}

	// 从URL路径中提取文件ID
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "无效的文件ID", http.StatusBadRequest)
		return
	}
	fileID := pathParts[len(pathParts)-1]

	// 从存储服务下载文件
	reader, fileInfo, err := h.storageService.DownloadFile(r.Context(), fileID)
	if err != nil {
		http.Error(w, fmt.Sprintf("下载文件失败: %v", err), http.StatusNotFound)
		return
	}
	defer reader.(io.ReadCloser).Close()

	// 设置响应头
	w.Header().Set("Content-Type", fileInfo.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileInfo.FileName))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.FileSize))

	// 写入文件内容
	_, err = io.Copy(w, reader)
	if err != nil {
		http.Error(w, fmt.Sprintf("写入响应失败: %v", err), http.StatusInternalServerError)
		return
	}
}

// DeleteFile 处理文件删除请求
// DELETE /api/files/{fileID}
func (h *FileHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 检查请求方法
	if r.Method != http.MethodDelete {
		http.Error(w, "只支持DELETE方法", http.StatusMethodNotAllowed)
		return
	}

	// 从URL路径中提取文件ID
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 {
		http.Error(w, "无效的文件ID", http.StatusBadRequest)
		return
	}
	fileID := pathParts[len(pathParts)-1]

	// 从存储服务删除文件
	err := h.storageService.DeleteFile(r.Context(), fileID)
	if err != nil {
		http.Error(w, fmt.Sprintf("删除文件失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 返回成功响应
	response := map[string]interface{}{
		"success": true,
		"message": "文件删除成功",
		"fileID":  fileID,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// GetFileInfo 获取文件信息
// GET /api/files/{fileID}/info
func (h *FileHandler) GetFileInfo(w http.ResponseWriter, r *http.Request) {
	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 检查请求方法
	if r.Method != http.MethodGet {
		http.Error(w, "只支持GET方法", http.StatusMethodNotAllowed)
		return
	}

	// 从URL路径中提取文件ID
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "无效的文件ID", http.StatusBadRequest)
		return
	}
	fileID := pathParts[len(pathParts)-2] // 倒数第二个是fileID，最后一个是"info"

	// 获取文件信息
	fileInfo, err := h.storageService.GetFileInfo(r.Context(), fileID)
	if err != nil {
		http.Error(w, fmt.Sprintf("获取文件信息失败: %v", err), http.StatusNotFound)
		return
	}

	// 返回文件信息
	response := map[string]interface{}{
		"success": true,
		"data":    fileInfo,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// ListFiles 列出所有文件
// GET /api/files
func (h *FileHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 检查请求方法
	if r.Method != http.MethodGet {
		http.Error(w, "只支持GET方法", http.StatusMethodNotAllowed)
		return
	}

	// 获取文件列表
	files, err := h.storageService.ListFiles(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("获取文件列表失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 返回文件列表
	response := map[string]interface{}{
		"success": true,
		"data":    files,
		"count":   len(files),
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// HealthCheck 健康检查接口
// GET /api/health
func (h *FileHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"service":   "file-storage-service",
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// isTextFile 检查是否为文本文件
func isTextFile(contentType string) bool {
	textTypes := []string{
		"text/plain",
		"text/html",
		"text/css",
		"text/javascript",
		"application/json",
		"application/xml",
		"application/javascript",
		"application/x-yaml",
		"application/x-toml",
		"application/x-csv",
	}

	for _, textType := range textTypes {
		if strings.HasPrefix(contentType, textType) {
			return true
		}
	}
	return false
}
