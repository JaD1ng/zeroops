package database

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/qiniu/zeroops/internal/deploy/model"
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

// GetInstancesByServiceName 根据服务名获取实例信息列表
func (r *InstanceRepo) GetInstanceInfosByServiceName(serviceName string) ([]*model.InstanceInfo, error) {
	query := `
		SELECT id, service_version, status
		FROM instances 
		WHERE service_name = $1
		ORDER BY id
	`

	rows, err := r.db.Query(query, serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to query instances by service name: %w", err)
	}
	defer rows.Close()

	var instanceInfos []*model.InstanceInfo
	for rows.Next() {
		var instanceInfo model.InstanceInfo
		var id int
		err := rows.Scan(&id, &instanceInfo.ServiceVersion, &instanceInfo.Status)
		if err != nil {
			return nil, fmt.Errorf("failed to scan instance: %w", err)
		}
		instanceInfo.InstanceID = strconv.Itoa(id)
		instanceInfo.ServiceName = serviceName // 直接使用参数值
		instanceInfos = append(instanceInfos, &instanceInfo)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating instances: %w", err)
	}

	return instanceInfos, nil
}

// GetInstancesByServiceNameAndVersion 根据服务名和版本获取实例信息列表
func (r *InstanceRepo) GetInstanceInfosByServiceNameAndVersion(serviceName, serviceVersion string) ([]*model.InstanceInfo, error) {
	query := `
		SELECT id, status
		FROM instances 
		WHERE service_name = $1 AND service_version = $2
		ORDER BY id
	`

	rows, err := r.db.Query(query, serviceName, serviceVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to query instances by service name and version: %w", err)
	}
	defer rows.Close()

	var instanceInfos []*model.InstanceInfo
	for rows.Next() {
		var instanceInfo model.InstanceInfo
		var id int
		err := rows.Scan(&id, &instanceInfo.Status)
		if err != nil {
			return nil, fmt.Errorf("failed to scan instance: %w", err)
		}
		instanceInfo.InstanceID = strconv.Itoa(id)
		instanceInfo.ServiceName = serviceName       // 直接使用参数值
		instanceInfo.ServiceVersion = serviceVersion // 直接使用参数值
		instanceInfos = append(instanceInfos, &instanceInfo)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating instances: %w", err)
	}

	return instanceInfos, nil
}
