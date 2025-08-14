package repository

import (
	"context"
	"encoding/json"
	"metadata/internal/model"
	"shared/database"
)

// MetadataRepository 元数据存储接口
type MetadataRepository interface {
	Save(ctx context.Context, entry *model.MetadataEntry) error
	Get(ctx context.Context, key string) (*model.MetadataEntry, error)
	Update(ctx context.Context, entry *model.MetadataEntry) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, limit, offset int) ([]*model.MetadataEntry, error)
	Exists(ctx context.Context, key string) (bool, error)
	GetStats(ctx context.Context) (map[string]any, error)
	Search(ctx context.Context, query string, limit int) ([]*model.MetadataEntry, error)
}

type metadataRepository struct {
	db    database.SQLDatabase
	cache database.RedisCache
}

func NewMetadataRepository(db database.SQLDatabase, cache database.RedisCache) MetadataRepository {
	return &metadataRepository{
		db:    db,
		cache: cache,
	}
}

func (r *metadataRepository) Save(ctx context.Context, entry *model.MetadataEntry) error {
	query := `
		INSERT INTO metadata (id, key, size, content_type, md5_hash, storage_nodes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (key) DO UPDATE SET
			size = EXCLUDED.size,
			content_type = EXCLUDED.content_type,
			md5_hash = EXCLUDED.md5_hash,
			storage_nodes = EXCLUDED.storage_nodes,
			updated_at = EXCLUDED.updated_at
	`

	// 将storage_nodes转换为JSON
	storageNodesJSON, err := r.serializeStorageNodes(entry.StorageNodes)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, query,
		entry.ID,
		entry.Key,
		entry.Size,
		entry.ContentType,
		entry.MD5Hash,
		storageNodesJSON,
		entry.CreatedAt,
		entry.UpdatedAt,
	)

	if err != nil {
		return err
	}

	return r.setCache(ctx, entry.Key, entry)
}

func (r *metadataRepository) Get(ctx context.Context, key string) (*model.MetadataEntry, error) {
	if entry := r.getCache(ctx, key); entry != nil {
		return entry, nil
	}

	query := `
		SELECT id, key, size, content_type, md5_hash, storage_nodes, created_at, updated_at
		FROM metadata
		WHERE key = $1
	`

	entry := &model.MetadataEntry{}
	var storageNodesJSON string

	err := r.db.QueryRow(ctx, query, key).Scan(
		&entry.ID,
		&entry.Key,
		&entry.Size,
		&entry.ContentType,
		&entry.MD5Hash,
		&storageNodesJSON,
		&entry.CreatedAt,
		&entry.UpdatedAt,
	)

	if err != nil {
		return nil, model.ErrMetadataNotFound
	}

	// 反序列化storage_nodes
	entry.StorageNodes, err = r.deserializeStorageNodes(storageNodesJSON)
	if err != nil {
		return nil, err
	}

	r.setCache(ctx, key, entry)
	return entry, nil
}

func (r *metadataRepository) Update(ctx context.Context, entry *model.MetadataEntry) error {
	query := `
		UPDATE metadata
		SET size = $2, content_type = $3, md5_hash = $4, storage_nodes = $5, updated_at = $6
		WHERE key = $1
	`

	storageNodesJSON, err := r.serializeStorageNodes(entry.StorageNodes)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, query,
		entry.Key,
		entry.Size,
		entry.ContentType,
		entry.MD5Hash,
		storageNodesJSON,
		entry.UpdatedAt,
	)

	if err != nil {
		return err
	}

	return r.setCache(ctx, entry.Key, entry)
}

func (r *metadataRepository) Delete(ctx context.Context, key string) error {
	query := `DELETE FROM metadata WHERE key = $1`

	_, err := r.db.Exec(ctx, query, key)
	if err != nil {
		return err
	}

	return r.deleteCache(ctx, key)
}

func (r *metadataRepository) List(ctx context.Context, limit, offset int) ([]*model.MetadataEntry, error) {
	query := `
		SELECT id, key, size, content_type, md5_hash, storage_nodes, created_at, updated_at
		FROM metadata
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*model.MetadataEntry
	for rows.Next() {
		entry := &model.MetadataEntry{}
		var storageNodesJSON string

		err := rows.Scan(
			&entry.ID,
			&entry.Key,
			&entry.Size,
			&entry.ContentType,
			&entry.MD5Hash,
			&storageNodesJSON,
			&entry.CreatedAt,
			&entry.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		entry.StorageNodes, err = r.deserializeStorageNodes(storageNodesJSON)
		if err != nil {
			continue // 跳过损坏的记录
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

func (r *metadataRepository) Exists(ctx context.Context, key string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM metadata WHERE key = $1)`

	var exists bool
	err := r.db.QueryRow(ctx, query, key).Scan(&exists)
	return exists, err
}

func (r *metadataRepository) GetStats(ctx context.Context) (map[string]any, error) {
	stats := make(map[string]any)

	// 总文件数
	var totalFiles int64
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM metadata").Scan(&totalFiles)
	if err != nil {
		return nil, err
	}
	stats["total_files"] = totalFiles

	// 总大小
	var totalSize int64
	err = r.db.QueryRow(ctx, "SELECT COALESCE(SUM(size), 0) FROM metadata").Scan(&totalSize)
	if err != nil {
		return nil, err
	}
	stats["total_size"] = totalSize

	// 平均大小
	if totalFiles > 0 {
		stats["average_size"] = totalSize / totalFiles
	} else {
		stats["average_size"] = 0
	}

	// 按内容类型统计
	contentTypeStats := make(map[string]int)
	rows, err := r.db.Query(ctx, "SELECT content_type, COUNT(*) FROM metadata GROUP BY content_type")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var contentType string
			var count int
			if rows.Scan(&contentType, &count) == nil {
				contentTypeStats[contentType] = count
			}
		}
	}
	stats["content_types"] = contentTypeStats

	return stats, nil
}

func (r *metadataRepository) Search(ctx context.Context, query string, limit int) ([]*model.MetadataEntry, error) {
	searchSQL := `
		SELECT id, key, size, content_type, md5_hash, storage_nodes, created_at, updated_at
		FROM metadata 
		WHERE key ILIKE $1 OR content_type ILIKE $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	searchPattern := "%" + query + "%"
	rows, err := r.db.Query(ctx, searchSQL, searchPattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*model.MetadataEntry
	for rows.Next() {
		entry := &model.MetadataEntry{}
		var storageNodesJSON string

		err := rows.Scan(
			&entry.ID,
			&entry.Key,
			&entry.Size,
			&entry.ContentType,
			&entry.MD5Hash,
			&storageNodesJSON,
			&entry.CreatedAt,
			&entry.UpdatedAt,
		)

		if err != nil {
			continue
		}

		entry.StorageNodes, err = r.deserializeStorageNodes(storageNodesJSON)
		if err != nil {
			continue
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// 辅助方法
func (r *metadataRepository) serializeStorageNodes(nodes []string) (string, error) {
	data, err := json.Marshal(nodes)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (r *metadataRepository) deserializeStorageNodes(data string) ([]string, error) {
	if data == "" {
		return []string{}, nil
	}

	var nodes []string
	err := json.Unmarshal([]byte(data), &nodes)
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

func (r *metadataRepository) getCache(ctx context.Context, key string) *model.MetadataEntry {
	// TODO: 实现缓存获取逻辑
	return nil
}

func (r *metadataRepository) setCache(ctx context.Context, key string, entry *model.MetadataEntry) error {
	// TODO: 实现缓存设置逻辑
	return nil
}

func (r *metadataRepository) deleteCache(ctx context.Context, key string) error {
	// TODO: 实现缓存删除逻辑
	return nil
}
