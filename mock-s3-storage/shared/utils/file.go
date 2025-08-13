package utils

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// EnsureDir 确保目录存在，如果不存在则创建
func EnsureDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}

// SafeFileName 生成安全的文件名
func SafeFileName(name string) string {
	// 替换不安全的字符
	unsafe := []string{"\\", "/", ":", "*", "?", "\"", "<", ">", "|"}
	safe := name
	for _, char := range unsafe {
		safe = strings.ReplaceAll(safe, char, "_")
	}
	return safe
}

// GetFileExtension 获取文件扩展名
func GetFileExtension(filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		return ".txt" // 默认扩展名
	}
	return ext
}

// CalculateFileHash 计算文件内容的MD5哈希
func CalculateFileHash(content []byte) string {
	hash := md5.Sum(content)
	return fmt.Sprintf("%x", hash)
}

// GetFileMimeType 根据扩展名获取MIME类型
func GetFileMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	mimeTypes := map[string]string{
		".txt":  "text/plain",
		".json": "application/json",
		".xml":  "application/xml",
		".html": "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".yaml": "application/x-yaml",
		".yml":  "application/x-yaml",
		".toml": "application/x-toml",
		".csv":  "text/csv",
		".md":   "text/markdown",
	}
	
	if mimeType, ok := mimeTypes[ext]; ok {
		return mimeType
	}
	return "text/plain" // 默认类型
}

// IsTextFile 检查是否为文本文件类型
func IsTextFile(contentType string) bool {
	textTypes := []string{
		"text/plain",
		"text/html",
		"text/css",
		"text/javascript",
		"text/markdown",
		"text/csv",
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

// FileInfo 文件信息结构
type FileInfo struct {
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	ModTime      time.Time `json:"mod_time"`
	IsDir        bool      `json:"is_dir"`
	ContentType  string    `json:"content_type"`
	Hash         string    `json:"hash,omitempty"`
}

// GetFileInfo 获取文件信息
func GetFileInfo(path string) (*FileInfo, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	
	info := &FileInfo{
		Name:    stat.Name(),
		Size:    stat.Size(),
		ModTime: stat.ModTime(),
		IsDir:   stat.IsDir(),
	}
	
	if !info.IsDir {
		info.ContentType = GetFileMimeType(info.Name)
	}
	
	return info, nil
}

// CopyFile 复制文件
func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	
	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()
	
	_, err = io.Copy(destFile, sourceFile)
	return err
}