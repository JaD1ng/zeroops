package database

import (
	"database/sql"
	"fmt"
	"sync"

	_ "github.com/lib/pq" // PostgreSQL驱动
	"github.com/qiniu/zeroops/internal/deploy/config"
)

// Database 数据库连接管理器
type Database struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewDatabase 创建新的数据库连接
func NewDatabase(cfg *config.DatabaseConfig) (*Database, error) {
	dsn := cfg.GetDSN()

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// 测试连接
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	database := &Database{
		db: db,
	}

	return database, nil
}

// GetDB 获取数据库连接（供repo使用）
func (d *Database) GetDB() *sql.DB {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db
}

// Close 关闭数据库连接
func (d *Database) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// Ping 测试数据库连接
func (d *Database) Ping() error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.Ping()
}
