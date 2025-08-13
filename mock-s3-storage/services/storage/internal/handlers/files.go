package handlers

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"storage-service/internal/services"
	"shared/utils"
)

// FilesHandler 文件操作处理器
type FilesHandler struct {
	storageService services.StorageService
}

// NewFilesHandler 创建文件处理器
func NewFilesHandler(storageService services.StorageService) *FilesHandler {
	return &FilesHandler{
		storageService: storageService,
	}
}

// UploadFile 处理文件上传请求
// POST /api/files/upload
func (h *FilesHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	// 检查请求方法
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "只支持POST方法")
		return
	}

	// 解析multipart表单
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB限制
		writeErrorResponse(w, http.StatusBadRequest, "PARSE_FORM_ERROR", fmt.Sprintf("解析表单失败: %v", err))
		return
	}

	// 获取上传的文件
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "GET_FILE_ERROR", fmt.Sprintf("获取文件失败: %v", err))
		return
	}
	defer file.Close()

	// 检查文件类型（只允许文本文件）
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = utils.GetFileMimeType(header.Filename)
	}
	if !utils.IsTextFile(contentType) {
		writeErrorResponse(w, http.StatusBadRequest, "INVALID_FILE_TYPE", "只支持文本文件上传")
		return
	}

	// 生成文件ID
	fileID := utils.GenerateFileID()

	// 使用原始文件名
	fileName := header.Filename

	// 上传文件到存储服务
	fileInfo, err := h.storageService.UploadFile(r.Context(), fileID, fileName, contentType, file)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "UPLOAD_ERROR", fmt.Sprintf("上传文件失败: %v", err))
		return
	}

	// 返回成功响应
	writeSuccessResponse(w, "文件上传成功", fileInfo)
}

// DownloadFile 处理文件下载请求
// GET /api/files/download/{fileID}
func (h *FilesHandler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	// 检查请求方法
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "只支持GET方法")
		return
	}

	// 从URL路径中提取文件ID
	fileID := extractFileIDFromPath(r.URL.Path, 4) // /api/files/download/{fileID}
	if fileID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "INVALID_FILE_ID", "无效的文件ID")
		return
	}

	// 从存储服务下载文件
	reader, fileInfo, err := h.storageService.DownloadFile(r.Context(), fileID)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "DOWNLOAD_ERROR", fmt.Sprintf("下载文件失败: %v", err))
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
		// 注意：这里不能再写HTTP错误响应，因为可能已经开始写入文件内容
		return
	}
}

// DeleteFile 处理文件删除请求
// DELETE /api/files/{fileID}
func (h *FilesHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	// 检查请求方法
	if r.Method != http.MethodDelete {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "只支持DELETE方法")
		return
	}

	// 从URL路径中提取文件ID
	fileID := extractFileIDFromPath(r.URL.Path, 3) // /api/files/{fileID}
	if fileID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "INVALID_FILE_ID", "无效的文件ID")
		return
	}

	// 从存储服务删除文件
	err := h.storageService.DeleteFile(r.Context(), fileID)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "DELETE_ERROR", fmt.Sprintf("删除文件失败: %v", err))
		return
	}

	// 返回成功响应
	response := map[string]any{
		"success": true,
		"message": "文件删除成功",
		"file_id": fileID,
	}
	writeJSONResponse(w, http.StatusOK, response)
}

// GetFileInfo 获取文件信息
// GET /api/files/{fileID}/info
func (h *FilesHandler) GetFileInfo(w http.ResponseWriter, r *http.Request) {
	// 检查请求方法
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "只支持GET方法")
		return
	}

	// 从URL路径中提取文件ID
	fileID := extractFileIDFromPath(r.URL.Path, 4) // /api/files/{fileID}/info
	if fileID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "INVALID_FILE_ID", "无效的文件ID")
		return
	}
	// 去掉末尾的 "/info"
	fileID = strings.TrimSuffix(fileID, "/info")

	// 获取文件信息
	fileInfo, err := h.storageService.GetFileInfo(r.Context(), fileID)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "GET_INFO_ERROR", fmt.Sprintf("获取文件信息失败: %v", err))
		return
	}

	// 返回文件信息
	writeSuccessResponse(w, "获取文件信息成功", fileInfo)
}

// ListFiles 列出所有文件
// GET /api/files
func (h *FilesHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	// 检查请求方法
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "只支持GET方法")
		return
	}

	// 获取文件列表
	files, err := h.storageService.ListFiles(r.Context())
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "LIST_ERROR", fmt.Sprintf("获取文件列表失败: %v", err))
		return
	}

	// 返回文件列表
	response := map[string]any{
		"success": true,
		"message": "获取文件列表成功",
		"data":    files,
		"count":   len(files),
	}
	writeJSONResponse(w, http.StatusOK, response)
}

// extractFileIDFromPath 从路径中提取文件ID
func extractFileIDFromPath(path string, expectedParts int) string {
	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	if len(pathParts) < expectedParts {
		return ""
	}
	return pathParts[expectedParts-1]
}

