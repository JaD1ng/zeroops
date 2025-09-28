package database

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/qiniu/zeroops/internal/deploy/model"
)

// HostRepo 主机数据访问层
type HostRepo struct {
	db *sql.DB
}

// NewHostRepo 创建主机仓库
func NewHostRepo(db *Database) *HostRepo {
	return &HostRepo{
		db: db.GetDB(),
	}
}

// GetAvailableHostInfos 获取所有可用的主机信息列表
func (r *HostRepo) GetAvailableHostInfos() ([]*model.HostInfo, error) {
	query := `
		SELECT id, name, ip_address, is_stopped
		FROM hosts 
		WHERE is_stopped = false
		ORDER BY id
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query available hosts: %w", err)
	}
	defer rows.Close()

	var hostInfos []*model.HostInfo
	for rows.Next() {
		hostInfo := new(model.HostInfo)
		var id int
		err := rows.Scan(&id, &hostInfo.HostName, &hostInfo.HostIPAddress, &hostInfo.IsStopped)
		if err != nil {
			return nil, fmt.Errorf("failed to scan host: %w", err)
		}
		hostInfo.HostID = strconv.Itoa(id)
		hostInfos = append(hostInfos, hostInfo)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating hosts: %w", err)
	}

	return hostInfos, nil
}
