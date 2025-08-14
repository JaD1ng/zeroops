package repository

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"
	"time"

	"storage-service/internal/model"

	_ "github.com/lib/pq"
)

// PostgresStorage PostgreSQL存储实现
type PostgresStorage struct {
	db        *sql.DB
	tableName string
}

// NewPostgresStorage 创建PostgreSQL存储实例
func NewPostgresStorage(connectionString string, tableName string) (*PostgresStorage, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("连接PostgreSQL失败: %w", err)
	}

	// 测试连接
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("PostgreSQL连接测试失败: %w", err)
	}

	// 创建文件表
	if err := createFileTable(db, tableName); err != nil {
		return nil, fmt.Errorf("创建文件表失败: %w", err)
	}

	return &PostgresStorage{db: db, tableName: tableName}, nil
}

// createFileTable 创建文件存储表
func createFileTable(db *sql.DB, tableName string) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id VARCHAR(255) PRIMARY KEY,
			file_name VARCHAR(255) NOT NULL,
			file_size BIGINT NOT NULL,
			content_type VARCHAR(100) NOT NULL,
			file_content TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE INDEX IF NOT EXISTS idx_%s_created_at ON %s(created_at);
		CREATE INDEX IF NOT EXISTS idx_%s_file_name ON %s(file_name);
	`, tableName, tableName, tableName, tableName, tableName)

	_, err := db.Exec(query)
	return err
}

// UploadFile 上传文件到PostgreSQL
func (p *PostgresStorage) UploadFile(ctx context.Context, fileID, fileName, contentType string, reader io.Reader) (*model.FileInfo, error) {
	// 读取文件内容
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("读取文件内容失败: %w", err)
	}

	// 检查文件大小（限制为1MB，适合文本文件）
	if len(content) > 1024*1024 {
		return nil, fmt.Errorf("文件大小超过限制(1MB)")
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	// 插入或更新文件记录
	query := fmt.Sprintf(`
		INSERT INTO %s (id, file_name, file_size, content_type, file_content, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		ON CONFLICT (id) DO UPDATE SET
			file_name = EXCLUDED.file_name,
			file_size = EXCLUDED.file_size,
			content_type = EXCLUDED.content_type,
			file_content = EXCLUDED.file_content,
			updated_at = EXCLUDED.updated_at
	`, p.tableName)

	_, err = p.db.ExecContext(ctx, query, fileID, fileName, int64(len(content)), contentType, string(content), now)
	if err != nil {
		return nil, fmt.Errorf("保存文件到数据库失败: %w", err)
	}

	return &model.FileInfo{
		ID:          fileID,
		FileName:    fileName,
		FileSize:    int64(len(content)),
		ContentType: contentType,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

// DownloadFile 从PostgreSQL下载文件
func (p *PostgresStorage) DownloadFile(ctx context.Context, fileID string) (io.Reader, *model.FileInfo, error) {
	query := fmt.Sprintf(`
		SELECT id, file_name, file_size, content_type, file_content, created_at, updated_at
		FROM %s WHERE id = $1
	`, p.tableName)

	var fileInfo model.FileInfo
	var content string

	err := p.db.QueryRowContext(ctx, query, fileID).Scan(
		&fileInfo.ID,
		&fileInfo.FileName,
		&fileInfo.FileSize,
		&fileInfo.ContentType,
		&content,
		&fileInfo.CreatedAt,
		&fileInfo.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, fmt.Errorf("文件不存在: %s", fileID)
		}
		return nil, nil, fmt.Errorf("查询文件失败: %w", err)
	}

	reader := io.NopCloser(io.NewSectionReader(strings.NewReader(content), 0, int64(len(content))))
	return reader, &fileInfo, nil
}

// DeleteFile 从PostgreSQL删除文件
func (p *PostgresStorage) DeleteFile(ctx context.Context, fileID string) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, p.tableName)

	result, err := p.db.ExecContext(ctx, query, fileID)
	if err != nil {
		return fmt.Errorf("删除文件失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取删除行数失败: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("文件不存在: %s", fileID)
	}

	return nil
}

// GetFileInfo 获取文件信息
func (p *PostgresStorage) GetFileInfo(ctx context.Context, fileID string) (*model.FileInfo, error) {
	query := fmt.Sprintf(`
		SELECT id, file_name, file_size, content_type, created_at, updated_at
		FROM %s WHERE id = $1
	`, p.tableName)

	var fileInfo model.FileInfo
	err := p.db.QueryRowContext(ctx, query, fileID).Scan(
		&fileInfo.ID,
		&fileInfo.FileName,
		&fileInfo.FileSize,
		&fileInfo.ContentType,
		&fileInfo.CreatedAt,
		&fileInfo.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("文件不存在: %s", fileID)
		}
		return nil, fmt.Errorf("查询文件信息失败: %w", err)
	}

	return &fileInfo, nil
}

// ListFiles 列出所有文件
func (p *PostgresStorage) ListFiles(ctx context.Context) ([]*model.FileInfo, error) {
	query := fmt.Sprintf(`
		SELECT id, file_name, file_size, content_type, created_at, updated_at
		FROM %s ORDER BY created_at DESC
	`, p.tableName)

	rows, err := p.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询文件列表失败: %w", err)
	}
	defer rows.Close()

	var files []*model.FileInfo
	for rows.Next() {
		var fileInfo model.FileInfo
		err := rows.Scan(
			&fileInfo.ID,
			&fileInfo.FileName,
			&fileInfo.FileSize,
			&fileInfo.ContentType,
			&fileInfo.CreatedAt,
			&fileInfo.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("读取文件信息失败: %w", err)
		}
		files = append(files, &fileInfo)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历文件列表失败: %w", err)
	}

	return files, nil
}

// Close 关闭数据库连接
func (p *PostgresStorage) Close() error {
	return p.db.Close()
}
