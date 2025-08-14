package service

import (
	"context"
	"io"
	"storage-service/internal/model"
)

// StorageService 存储服务接口
// 这个接口定义了存储服务的基本操作，支持多种存储后端
type StorageService interface {
	// UploadFile 上传文件
	// fileID: 文件唯一标识符
	// fileName: 文件名
	// contentType: 文件类型
	// reader: 文件内容读取器
	// 返回文件信息和错误
	UploadFile(ctx context.Context, fileID, fileName, contentType string, reader io.Reader) (*model.FileInfo, error)

	// DownloadFile 下载文件
	// fileID: 文件唯一标识符
	// 返回文件内容读取器和文件信息
	DownloadFile(ctx context.Context, fileID string) (io.Reader, *model.FileInfo, error)

	// DeleteFile 删除文件
	// fileID: 文件唯一标识符
	// 返回删除是否成功和错误
	DeleteFile(ctx context.Context, fileID string) error

	// GetFileInfo 获取文件信息
	// fileID: 文件唯一标识符
	// 返回文件信息
	GetFileInfo(ctx context.Context, fileID string) (*model.FileInfo, error)

	// ListFiles 列出所有文件
	// 返回文件信息列表
	ListFiles(ctx context.Context) ([]*model.FileInfo, error)

	// Close 关闭存储连接
	// 用于清理资源
	Close() error
}
