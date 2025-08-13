package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"shared/faults"
	"shared/utils"
	"strings"
	"time"
)

// FilesystemStorageService 文件系统存储服务实现
type FilesystemStorageService struct {
	baseDir         string
	metadataDir     string
	maxFileSize     int64
	injectionEngine faults.InjectionEngine
}

// NewFilesystemStorageService 创建文件系统存储服务
func NewFilesystemStorageService(baseDir string, maxFileSize int64, injectionEngine faults.InjectionEngine) *FilesystemStorageService {
	return &FilesystemStorageService{
		baseDir:         baseDir,
		metadataDir:     filepath.Join(baseDir, ".metadata"),
		maxFileSize:     maxFileSize,
		injectionEngine: injectionEngine,
	}
}

// Initialize 初始化存储目录
func (f *FilesystemStorageService) Initialize() error {
	// 创建基础目录
	if err := utils.EnsureDir(f.baseDir); err != nil {
		return &faults.AppError{
			Type:    faults.ErrorTypeStorage,
			Code:    "STORAGE_INIT_ERROR",
			Message: fmt.Sprintf("Failed to create base directory: %v", err),
			Cause:   err,
		}
	}

	// 创建元数据目录
	if err := utils.EnsureDir(f.metadataDir); err != nil {
		return &faults.AppError{
			Type:    faults.ErrorTypeStorage,
			Code:    "METADATA_INIT_ERROR",
			Message: fmt.Sprintf("Failed to create metadata directory: %v", err),
			Cause:   err,
		}
	}

	return nil
}

// UploadFile 上传文件到文件系统
func (f *FilesystemStorageService) UploadFile(ctx context.Context, fileID, fileName, contentType string, reader io.Reader) (*FileInfo, error) {
	// 检查故障注入
	if f.injectionEngine != nil && f.injectionEngine.ShouldInject(ctx, "storage-service", "/api/files/upload") {
		if rule := f.injectionEngine.GetInjectionRule("storage-service", "/api/files/upload"); rule != nil {
			switch rule.Type {
			case faults.InjectionTypeStorageError:
				return nil, f.injectionEngine.CreateError(rule)
			case faults.InjectionTypeGoroutineLeak:
				// 触发Goroutine泄漏（在服务层面）
				f.createLeakedGoroutine(rule)
				return nil, &faults.AppError{
					Type:    faults.ErrorTypeGoroutineLeak,
					Code:    "UPLOAD_GOROUTINE_LEAK",
					Message: "Goroutine leak injected during file upload",
				}
			}
		}
	}

	// 读取文件内容
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, &faults.AppError{
			Type:    faults.ErrorTypeInternal,
			Code:    "READ_FILE_ERROR",
			Message: fmt.Sprintf("Failed to read file content: %v", err),
			Cause:   err,
		}
	}

	// 检查文件大小
	if int64(len(content)) > f.maxFileSize {
		return nil, &faults.AppError{
			Type:    faults.ErrorTypeBadRequest,
			Code:    "FILE_TOO_LARGE",
			Message: fmt.Sprintf("File size %d exceeds maximum allowed size %d", len(content), f.maxFileSize),
		}
	}

	// 生成安全的文件名
	safeName := utils.SafeFileName(fileName)
	filePath := filepath.Join(f.baseDir, fileID+"_"+safeName)

	// 写入文件
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return nil, &faults.AppError{
			Type:    faults.ErrorTypeStorage,
			Code:    "WRITE_FILE_ERROR",
			Message: fmt.Sprintf("Failed to write file: %v", err),
			Cause:   err,
		}
	}

	// 创建文件信息
	now := time.Now().Format(time.RFC3339)
	fileInfo := &FileInfo{
		ID:          fileID,
		FileName:    fileName,
		FileSize:    int64(len(content)),
		ContentType: contentType,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// 写入元数据
	if err := f.saveMetadata(fileInfo); err != nil {
		// 如果元数据写入失败，删除已写入的文件
		os.Remove(filePath)
		return nil, &faults.AppError{
			Type:    faults.ErrorTypeStorage,
			Code:    "METADATA_WRITE_ERROR",
			Message: fmt.Sprintf("Failed to write metadata: %v", err),
			Cause:   err,
		}
	}

	return fileInfo, nil
}

// DownloadFile 从文件系统下载文件
func (f *FilesystemStorageService) DownloadFile(ctx context.Context, fileID string) (io.Reader, *FileInfo, error) {
	// 检查故障注入
	if f.injectionEngine != nil && f.injectionEngine.ShouldInject(ctx, "storage-service", "/api/files/download") {
		if rule := f.injectionEngine.GetInjectionRule("storage-service", "/api/files/download"); rule != nil {
			if rule.Type == faults.InjectionTypeStorageError {
				return nil, nil, f.injectionEngine.CreateError(rule)
			}
		}
	}

	// 读取元数据
	fileInfo, err := f.loadMetadata(fileID)
	if err != nil {
		return nil, nil, &faults.AppError{
			Type:    faults.ErrorTypeObjectNotFound,
			Code:    "FILE_NOT_FOUND",
			Message: fmt.Sprintf("File not found: %s", fileID),
			Cause:   err,
		}
	}

	// 构建文件路径
	safeName := utils.SafeFileName(fileInfo.FileName)
	filePath := filepath.Join(f.baseDir, fileID+"_"+safeName)

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, nil, &faults.AppError{
			Type:    faults.ErrorTypeObjectNotFound,
			Code:    "FILE_NOT_FOUND",
			Message: fmt.Sprintf("Physical file not found: %s", fileID),
		}
	}

	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, &faults.AppError{
			Type:    faults.ErrorTypeStorage,
			Code:    "READ_FILE_ERROR",
			Message: fmt.Sprintf("Failed to open file: %v", err),
			Cause:   err,
		}
	}

	return file, fileInfo, nil
}

// DeleteFile 从文件系统删除文件
func (f *FilesystemStorageService) DeleteFile(ctx context.Context, fileID string) error {
	// 检查故障注入
	if f.injectionEngine != nil && f.injectionEngine.ShouldInject(ctx, "storage-service", "/api/files/delete") {
		if rule := f.injectionEngine.GetInjectionRule("storage-service", "/api/files/delete"); rule != nil {
			if rule.Type == faults.InjectionTypeStorageError {
				return f.injectionEngine.CreateError(rule)
			}
		}
	}

	// 读取元数据获取文件名
	fileInfo, err := f.loadMetadata(fileID)
	if err != nil {
		return &faults.AppError{
			Type:    faults.ErrorTypeObjectNotFound,
			Code:    "FILE_NOT_FOUND",
			Message: fmt.Sprintf("File not found: %s", fileID),
			Cause:   err,
		}
	}

	// 构建文件路径
	safeName := utils.SafeFileName(fileInfo.FileName)
	filePath := filepath.Join(f.baseDir, fileID+"_"+safeName)
	metadataPath := filepath.Join(f.metadataDir, fileID+".json")

	// 删除物理文件
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return &faults.AppError{
			Type:    faults.ErrorTypeStorage,
			Code:    "DELETE_FILE_ERROR",
			Message: fmt.Sprintf("Failed to delete file: %v", err),
			Cause:   err,
		}
	}

	// 删除元数据文件
	if err := os.Remove(metadataPath); err != nil && !os.IsNotExist(err) {
		return &faults.AppError{
			Type:    faults.ErrorTypeStorage,
			Code:    "DELETE_METADATA_ERROR",
			Message: fmt.Sprintf("Failed to delete metadata: %v", err),
			Cause:   err,
		}
	}

	return nil
}

// GetFileInfo 获取文件信息
func (f *FilesystemStorageService) GetFileInfo(ctx context.Context, fileID string) (*FileInfo, error) {
	// 检查故障注入
	if f.injectionEngine != nil && f.injectionEngine.ShouldInject(ctx, "storage-service", "/api/files/info") {
		if rule := f.injectionEngine.GetInjectionRule("storage-service", "/api/files/info"); rule != nil {
			if rule.Type == faults.InjectionTypeStorageError {
				return nil, f.injectionEngine.CreateError(rule)
			}
		}
	}

	fileInfo, err := f.loadMetadata(fileID)
	if err != nil {
		return nil, &faults.AppError{
			Type:    faults.ErrorTypeObjectNotFound,
			Code:    "FILE_NOT_FOUND",
			Message: fmt.Sprintf("File not found: %s", fileID),
			Cause:   err,
		}
	}

	return fileInfo, nil
}

// ListFiles 列出所有文件
func (f *FilesystemStorageService) ListFiles(ctx context.Context) ([]*FileInfo, error) {
	// 检查故障注入
	if f.injectionEngine != nil && f.injectionEngine.ShouldInject(ctx, "storage-service", "/api/files") {
		if rule := f.injectionEngine.GetInjectionRule("storage-service", "/api/files"); rule != nil {
			if rule.Type == faults.InjectionTypeStorageError {
				return nil, f.injectionEngine.CreateError(rule)
			}
		}
	}

	// 读取元数据目录
	entries, err := os.ReadDir(f.metadataDir)
	if err != nil {
		return nil, &faults.AppError{
			Type:    faults.ErrorTypeStorage,
			Code:    "LIST_FILES_ERROR",
			Message: fmt.Sprintf("Failed to read metadata directory: %v", err),
			Cause:   err,
		}
	}

	var files []*FileInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		fileID := strings.TrimSuffix(entry.Name(), ".json")
		fileInfo, err := f.loadMetadata(fileID)
		if err != nil {
			// 跳过损坏的元数据文件
			continue
		}

		files = append(files, fileInfo)
	}

	return files, nil
}

// Close 关闭存储服务（文件系统存储无需特殊关闭操作）
func (f *FilesystemStorageService) Close() error {
	return nil
}

// saveMetadata 保存文件元数据
func (f *FilesystemStorageService) saveMetadata(fileInfo *FileInfo) error {
	metadataPath := filepath.Join(f.metadataDir, fileInfo.ID+".json")
	
	data, err := json.MarshalIndent(fileInfo, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	return os.WriteFile(metadataPath, data, 0644)
}

// loadMetadata 加载文件元数据
func (f *FilesystemStorageService) loadMetadata(fileID string) (*FileInfo, error) {
	metadataPath := filepath.Join(f.metadataDir, fileID+".json")
	
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, err
	}

	var fileInfo FileInfo
	if err := json.Unmarshal(data, &fileInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &fileInfo, nil
}

// createLeakedGoroutine 创建泄漏的goroutine（用于故障注入）
func (f *FilesystemStorageService) createLeakedGoroutine(rule *faults.InjectionRule) {
	// 从配置中获取泄漏类型
	leakType, ok := rule.Config["leak_type"]
	if !ok {
		leakType = "file_processing" // 默认文件处理类型
	}
	
	leakCount, ok := rule.Config["leak_count"]
	if !ok {
		leakCount = 1.0 // 默认创建1个泄漏的goroutine
	}
	
	count := int(leakCount.(float64))
	
	// 创建与存储相关的泄漏goroutine
	for range count {
		switch leakType {
		case "file_processing":
			// 模拟文件处理过程中的goroutine泄漏
			go func() {
				for {
					// 模拟文件扫描，永不结束
					filepath.Walk(f.baseDir, func(path string, info os.FileInfo, err error) error {
						if err != nil {
							return nil // 忽略错误，继续扫描
						}
						time.Sleep(10 * time.Millisecond) // 模拟处理时间
						return nil
					})
					time.Sleep(100 * time.Millisecond)
				}
			}()
			
		case "metadata_sync":
			// 模拟元数据同步过程中的goroutine泄漏
			go func() {
				ticker := time.NewTicker(50 * time.Millisecond)
				// 注意：这里故意不调用ticker.Stop()，造成资源泄漏
				for range ticker.C {
					// 持续读取元数据目录，模拟同步过程
					os.ReadDir(f.metadataDir)
				}
			}()
			
		case "file_watcher":
			// 模拟文件监控goroutine泄漏
			go func() {
				for {
					// 模拟文件系统事件监听
					entries, _ := os.ReadDir(f.baseDir)
					for _, entry := range entries {
						// 模拟对每个文件的监控
						if !entry.IsDir() {
							filePath := filepath.Join(f.baseDir, entry.Name())
							os.Stat(filePath) // 检查文件状态
						}
					}
					time.Sleep(20 * time.Millisecond)
				}
			}()
			
		default:
			// 默认的阻塞goroutine
			go func() {
				ch := make(chan struct{})
				<-ch // 永远阻塞
			}()
		}
	}
}