package database

import (
	"database/sql"
	"fmt"
)

// InstanceRepo 实例数据访问层
type InstanceRepo struct {
	db *sql.DB
}

// NewInstanceRepo 创建实例仓库
func NewInstanceRepo(db *Database) *InstanceRepo {
	return &InstanceRepo{
		db: db.GetDB(),
	}
}

// GetInstanceIPByInstanceID 根据实例ID获取实例IP地址
func (r *InstanceRepo) GetInstanceIPByInstanceID(instanceID string) (string, error) {
	query := `SELECT ip_address FROM instances WHERE id = $1`

	var ipAddress string
	err := r.db.QueryRow(query, instanceID).Scan(&ipAddress)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("instance with ID %s not found", instanceID)
		}
		return "", fmt.Errorf("failed to query instance IP: %w", err)
	}

	return ipAddress, nil
}

// GetInstancePortByInstanceID 根据实例ID获取实例端口号
func (r *InstanceRepo) GetInstancePortByInstanceID(instanceID string) (int, error) {
	query := `SELECT port FROM instances WHERE id = $1`

	var port int
	err := r.db.QueryRow(query, instanceID).Scan(&port)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("instance with ID %s not found", instanceID)
		}
		return 0, fmt.Errorf("failed to query instance port: %w", err)
	}

	return port, nil
}
