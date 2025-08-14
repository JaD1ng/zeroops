package repository

import (
	"fmt"
	"storage-service/internal/service"
)

// StorageType 存储类型
type StorageType string

const (
	StorageTypePostgres StorageType = "postgres"
	StorageTypeS3       StorageType = "s3"
)

// StorageFactory 存储工厂
type StorageFactory struct{}

// NewStorageFactory 创建存储工厂
func NewStorageFactory() *StorageFactory {
	return &StorageFactory{}
}

// CreateStorage 创建存储实例
func (f *StorageFactory) CreateStorage(storageType StorageType, config map[string]string) (service.StorageService, error) {
	switch storageType {
	case StorageTypePostgres:
		connectionString, ok := config["connection_string"]
		if !ok {
			return nil, fmt.Errorf("PostgreSQL配置缺少connection_string")
		}
		tableName, ok := config["table_name"]
		if !ok {
			tableName = "files" // 默认表名
		}
		return NewPostgresStorage(connectionString, tableName)

	case StorageTypeS3:
		bucketName, ok := config["bucket_name"]
		if !ok {
			return nil, fmt.Errorf("S3配置缺少bucket_name")
		}
		region, ok := config["region"]
		if !ok {
			return nil, fmt.Errorf("S3配置缺少region")
		}
		return NewS3Storage(bucketName, region)

	default:
		return nil, fmt.Errorf("不支持的存储类型: %s", storageType)
	}
}

// CreatePostgresStorage 创建PostgreSQL存储实例（便捷方法）
func (f *StorageFactory) CreatePostgresStorage(connectionString string, tableName string) (service.StorageService, error) {
	return NewPostgresStorage(connectionString, tableName)
}

// CreateS3Storage 创建S3存储实例（便捷方法）
func (f *StorageFactory) CreateS3Storage(bucketName, region string) (service.StorageService, error) {
	return NewS3Storage(bucketName, region)
}
