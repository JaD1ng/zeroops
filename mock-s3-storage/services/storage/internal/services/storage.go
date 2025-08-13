package services

import (
	"context"
	"io"
)

// FileInfo 文件信息结构体
type FileInfo struct {
	ID          string `json:"id"`
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	ContentType string `json:"content_type"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// StorageService 存储服务接口
type StorageService interface {
	// UploadFile 上传文件
	UploadFile(ctx context.Context, fileID, fileName, contentType string, reader io.Reader) (*FileInfo, error)

	// DownloadFile 下载文件
	DownloadFile(ctx context.Context, fileID string) (io.Reader, *FileInfo, error)

	// DeleteFile 删除文件
	DeleteFile(ctx context.Context, fileID string) error

	// GetFileInfo 获取文件信息
	GetFileInfo(ctx context.Context, fileID string) (*FileInfo, error)

	// ListFiles 列出所有文件
	ListFiles(ctx context.Context) ([]*FileInfo, error)

	// Close 关闭存储连接
	Close() error
}