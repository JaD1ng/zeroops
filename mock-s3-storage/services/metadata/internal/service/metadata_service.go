package service

import (
	"context"
	"encoding/json"
	"metadata/internal/model"
	"metadata/internal/repository"
	"shared/faults"
	logs "shared/telemetry/logger"
	"shared/telemetry/metrics"
	"strings"
	"time"
)

// MetadataService 元数据服务
type MetadataService interface {
	// 保存元数据（由上层文件上传服务调用）
	SaveMetadata(ctx context.Context, entry *model.MetadataEntry) error

	// 获取元数据
	GetMetadata(ctx context.Context, key string) (*model.MetadataEntry, error)

	// 删除元数据
	DeleteMetadata(ctx context.Context, key string) error

	// 列出元数据（分页）
	ListMetadata(ctx context.Context, limit, offset int) ([]*model.MetadataEntry, error)

	// 更新元数据
	UpdateMetadata(ctx context.Context, key string, updates map[string]any) error

	// 获取统计信息
	GetStats(ctx context.Context) (map[string]any, error)

	// 搜索元数据
	SearchMetadata(ctx context.Context, query string, limit int) ([]*model.MetadataEntry, error)

	// 根据模式获取元数据
	GetMetadataByPattern(ctx context.Context, pattern string) ([]*model.MetadataEntry, error)

	// 导出元数据到JSON
	ExportMetadata(ctx context.Context, keys []string) ([]byte, error)

	// 从JSON导入元数据
	ImportMetadata(ctx context.Context, data []byte) error

	// 验证元数据
	ValidateMetadata(entry *model.MetadataEntry) error
}

type metadataService struct {
	repo     repository.MetadataRepository
	faultMgr *faults.FaultManager
	logger   logs.Logger
	metrics  metrics.Metrics
}

func NewMetadataService(
	repo repository.MetadataRepository,
	faultMgr *faults.FaultManager,
	logger logs.Logger,
	metrics metrics.Metrics,
) MetadataService {
	return &metadataService{
		repo:     repo,
		faultMgr: faultMgr,
		logger:   logger,
		metrics:  metrics,
	}
}

func (s *metadataService) SaveMetadata(ctx context.Context, entry *model.MetadataEntry) error {
	start := time.Now()
	defer func() {
		s.metrics.ObserveHistogram("metadata_save_duration", time.Since(start).Seconds(), "operation", "save")
	}()

	// 故障注入
	if s.faultMgr.ShouldInject(0.05) {
		s.logger.Warn(ctx, "故障注入：模拟元数据保存错误", map[string]any{
			"key":        entry.Key,
			"fault_type": "save_error",
		})
		return faults.NewError(faults.ErrorTypeInternal, "FAULT_INJECTION", "Simulated metadata save error")
	}

	// 验证元数据
	if err := s.ValidateMetadata(entry); err != nil {
		s.metrics.IncCounter("metadata_save_errors", "error", "validation")
		return err
	}

	// 保存到存储
	err := s.repo.Save(ctx, entry)
	if err != nil {
		s.logger.Error(ctx, "保存元数据失败", err, map[string]any{"key": entry.Key})
		s.metrics.IncCounter("metadata_save_errors", "error", "database")
		return err
	}

	s.logger.Info(ctx, "成功保存元数据", map[string]any{
		"key":  entry.Key,
		"size": entry.Size,
	})
	s.metrics.IncCounter("metadata_saved")
	return nil
}

func (s *metadataService) GetMetadata(ctx context.Context, key string) (*model.MetadataEntry, error) {
	start := time.Now()
	defer func() {
		s.metrics.ObserveHistogram("metadata_get_duration", time.Since(start).Seconds(), "operation", "get")
	}()

	entry, err := s.repo.Get(ctx, key)
	if err != nil {
		s.metrics.IncCounter("metadata_get_errors", "error", "not_found")
		return nil, model.ErrMetadataNotFound
	}

	s.metrics.IncCounter("metadata_get_success")
	return entry, nil
}

func (s *metadataService) DeleteMetadata(ctx context.Context, key string) error {
	start := time.Now()
	defer func() {
		s.metrics.ObserveHistogram("metadata_delete_duration", time.Since(start).Seconds(), "operation", "delete")
	}()

	// TODO: 需要先获取存储节点信息，然后发送到消息队列通知storage服务删除实际对象
	// 1. 获取元数据中的storage_nodes信息
	// 2. 发送删除消息到消息队列 (topic: object.delete)
	// 3. 消息内容应包含: key, storage_nodes
	// 4. storage服务收到消息后删除对应节点上的实际文件
	// 5. 只有在确认存储删除成功后，才删除元数据记录

	err := s.repo.Delete(ctx, key)
	if err != nil {
		s.logger.Error(ctx, "删除元数据失败", err, map[string]any{"key": key})
		s.metrics.IncCounter("metadata_delete_errors", "error", "database")
		return err
	}

	s.logger.Info(ctx, "成功删除元数据", map[string]any{"key": key})
	s.metrics.IncCounter("metadata_deleted")
	return nil
}

func (s *metadataService) ListMetadata(ctx context.Context, limit, offset int) ([]*model.MetadataEntry, error) {
	start := time.Now()
	defer func() {
		s.metrics.ObserveHistogram("metadata_list_duration", time.Since(start).Seconds(), "operation", "list")
	}()

	entries, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		s.logger.Error(ctx, "获取元数据列表失败", err, map[string]any{
			"limit":  limit,
			"offset": offset,
		})
		s.metrics.IncCounter("metadata_list_errors", "error", "database")
		return nil, err
	}

	s.metrics.IncCounter("metadata_list_success")
	return entries, nil
}

func (s *metadataService) UpdateMetadata(ctx context.Context, key string, updates map[string]any) error {
	// 首先获取现有元数据
	entry, err := s.GetMetadata(ctx, key)
	if err != nil {
		return err
	}

	// 更新字段
	if contentType, ok := updates["content_type"].(string); ok {
		entry.ContentType = contentType
	}
	if storageNodes, ok := updates["storage_nodes"].([]string); ok {
		entry.StorageNodes = storageNodes
	}

	entry.UpdatedAt = time.Now()

	err = s.repo.Update(ctx, entry)
	if err != nil {
		s.logger.Error(ctx, "更新元数据失败", err, map[string]any{"key": key})
		return err
	}

	s.logger.Info(ctx, "成功更新元数据", map[string]any{"key": key})
	return nil
}

func (s *metadataService) GetStats(ctx context.Context) (map[string]any, error) {
	stats, err := s.repo.GetStats(ctx)
	if err != nil {
		s.logger.Error(ctx, "获取统计信息失败", err, nil)
		return nil, err
	}

	return stats, nil
}

func (s *metadataService) SearchMetadata(ctx context.Context, query string, limit int) ([]*model.MetadataEntry, error) {
	entries, err := s.repo.Search(ctx, query, limit)
	if err != nil {
		s.logger.Error(ctx, "搜索元数据失败", err, map[string]any{"query": query})
		return nil, err
	}

	return entries, nil
}

func (s *metadataService) GetMetadataByPattern(ctx context.Context, pattern string) ([]*model.MetadataEntry, error) {
	// 简单的通配符匹配实现
	allEntries, err := s.ListMetadata(ctx, 1000, 0)
	if err != nil {
		return nil, err
	}

	var matchedEntries []*model.MetadataEntry
	for _, entry := range allEntries {
		if s.matchPattern(entry.Key, pattern) {
			matchedEntries = append(matchedEntries, entry)
		}
	}

	return matchedEntries, nil
}

func (s *metadataService) ExportMetadata(ctx context.Context, keys []string) ([]byte, error) {
	var entries []*model.MetadataEntry

	if len(keys) == 0 {
		// 导出所有元数据
		allEntries, err := s.ListMetadata(ctx, 1000, 0) // 限制1000条
		if err != nil {
			return nil, err
		}
		entries = allEntries
	} else {
		// 导出指定的元数据
		for _, key := range keys {
			entry, err := s.GetMetadata(ctx, key)
			if err != nil {
				continue // 跳过不存在的key
			}
			entries = append(entries, entry)
		}
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		s.logger.Error(ctx, "序列化元数据失败", err, nil)
		return nil, err
	}

	return data, nil
}

func (s *metadataService) ImportMetadata(ctx context.Context, data []byte) error {
	var entries []*model.MetadataEntry

	err := json.Unmarshal(data, &entries)
	if err != nil {
		return faults.NewError(faults.ErrorTypeBadRequest, "INVALID_JSON", "Failed to unmarshal metadata")
	}

	successCount := 0
	for _, entry := range entries {
		err := s.ValidateMetadata(entry)
		if err != nil {
			s.logger.Warn(ctx, "无效的元数据条目", map[string]any{
				"key":   entry.Key,
				"error": err.Error(),
			})
			continue
		}

		err = s.repo.Save(ctx, entry)
		if err != nil {
			s.logger.Warn(ctx, "导入元数据失败", map[string]any{
				"key":   entry.Key,
				"error": err.Error(),
			})
			continue
		}

		successCount++
	}

	s.logger.Info(ctx, "元数据导入完成", map[string]any{
		"total":   len(entries),
		"success": successCount,
	})
	return nil
}

func (s *metadataService) ValidateMetadata(entry *model.MetadataEntry) error {
	if entry.Key == "" {
		return model.ErrInvalidKey
	}

	if entry.Size < 0 {
		return model.ErrInvalidMetadata
	}

	if len(entry.StorageNodes) == 0 {
		return model.ErrInvalidMetadata
	}

	// 验证MD5格式
	if entry.MD5Hash != "" && len(entry.MD5Hash) != 32 {
		return model.ErrInvalidMetadata
	}

	return nil
}

// matchPattern 简单的通配符匹配
func (s *metadataService) matchPattern(text, pattern string) bool {
	if pattern == "*" {
		return true
	}

	if strings.Contains(pattern, "*") {
		// 简单的前缀/后缀匹配
		if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
			// *something*
			middle := pattern[1 : len(pattern)-1]
			return strings.Contains(text, middle)
		} else if strings.HasPrefix(pattern, "*") {
			// *suffix
			suffix := pattern[1:]
			return strings.HasSuffix(text, suffix)
		} else if strings.HasSuffix(pattern, "*") {
			// prefix*
			prefix := pattern[:len(pattern)-1]
			return strings.HasPrefix(text, prefix)
		}
	}

	return text == pattern
}
