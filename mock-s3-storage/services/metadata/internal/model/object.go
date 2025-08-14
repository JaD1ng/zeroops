package model

import (
	"strings"
	"time"
)

// MetadataEntry 元数据条目 - 对齐原始服务设计
type MetadataEntry struct {
	ID           string    `json:"id" db:"id"`
	Key          string    `json:"key" db:"key"`                          // 完整的key，包含bucket前缀 (bucket/object-key)
	Size         int64     `json:"size" db:"size"`
	ContentType  string    `json:"content_type" db:"content_type"`
	MD5Hash      string    `json:"md5_hash" db:"md5_hash"`
	StorageNodes []string  `json:"storage_nodes" db:"storage_nodes"`      // 存储节点列表
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// NewMetadataEntry 创建新的元数据条目
func NewMetadataEntry(id, key string, size int64, contentType, md5Hash string, storageNodes []string) *MetadataEntry {
	now := time.Now()
	return &MetadataEntry{
		ID:           id,
		Key:          key,
		Size:         size,
		ContentType:  contentType,
		MD5Hash:      md5Hash,
		StorageNodes: storageNodes,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// UpdateStorageNodes 更新存储节点
func (e *MetadataEntry) UpdateStorageNodes(nodes []string) {
	e.StorageNodes = nodes
	e.UpdatedAt = time.Now()
}

// ExtractBucketName 从完整key中提取bucket名称
func (e *MetadataEntry) ExtractBucketName() string {
	if idx := strings.Index(e.Key, "/"); idx > 0 {
		return e.Key[:idx]
	}
	return ""
}

// ExtractObjectKey 从完整key中提取对象key
func (e *MetadataEntry) ExtractObjectKey() string {
	if idx := strings.Index(e.Key, "/"); idx > 0 && idx < len(e.Key)-1 {
		return e.Key[idx+1:]
	}
	return e.Key
}
