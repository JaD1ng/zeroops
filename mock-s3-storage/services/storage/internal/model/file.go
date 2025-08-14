package model

import "time"

// FileInfo 文件信息模型
type FileInfo struct {
	ID          string    `json:"id"`
	FileName    string    `json:"file_name"`
	FileSize    int64     `json:"file_size"`
	ContentType string    `json:"content_type"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// FileUploadRequest 文件上传请求
type FileUploadRequest struct {
	FileID      string
	FileName    string
	ContentType string
	Reader      interface{} // io.Reader
}

// FileDownloadResponse 文件下载响应
type FileDownloadResponse struct {
	FileInfo *FileInfo
	Reader   interface{} // io.Reader
}

// FileListResponse 文件列表响应
type FileListResponse struct {
	Files []*FileInfo `json:"files"`
	Count int         `json:"count"`
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Service   string    `json:"service"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// APIResponse 通用API响应
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
