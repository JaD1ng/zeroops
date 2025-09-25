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
		instanceInfo := new(model.InstanceInfo)
		var id int
		err := rows.Scan(&id, &instanceInfo.ServiceVersion, &instanceInfo.Status)
		if err != nil {
			return nil, fmt.Errorf("failed to scan instance: %w", err)
		}
		instanceInfo.InstanceID = strconv.Itoa(id)
		instanceInfo.ServiceName = serviceName // 直接使用参数值
		instanceInfos = append(instanceInfos, instanceInfo)
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
		instanceInfo := new(model.InstanceInfo)
		var id int
		err := rows.Scan(&id, &instanceInfo.Status)
		if err != nil {
			return nil, fmt.Errorf("failed to scan instance: %w", err)
		}
		instanceInfo.InstanceID = strconv.Itoa(id)
		instanceInfo.ServiceName = serviceName       // 直接使用参数值
		instanceInfo.ServiceVersion = serviceVersion // 直接使用参数值
		instanceInfos = append(instanceInfos, instanceInfo)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating instances: %w", err)
	}

	return instanceInfos, nil
}

// GetVersionHistoryByInstanceID 根据实例ID获取版本历史记录
func (r *InstanceRepo) GetVersionHistoryByInstanceID(instanceID string) ([]*model.VersionInfo, error) {
	// 参数验证
	if instanceID == "" {
		return nil, fmt.Errorf("instanceID cannot be empty")
	}

	query := `
		SELECT service_version, status
		FROM version_histories 
		WHERE instance_id = $1
		ORDER BY id DESC
	`

	rows, err := r.db.Query(query, instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to query version history for instance %s: %w", instanceID, err)
	}
	defer rows.Close()

	var versionInfos []*model.VersionInfo
	for rows.Next() {
		// 创建新地址，防止后续代码逻辑发生变化出现地址复用问题
		versionInfo := new(model.VersionInfo)
		err := rows.Scan(&versionInfo.Version, &versionInfo.Status)
		if err != nil {
			return nil, fmt.Errorf("failed to scan version history: %w", err)
		}
		versionInfos = append(versionInfos, versionInfo)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating version history: %w", err)
	}

	return versionInfos, nil
}

// CreateInstance 创建新实例记录
func (r *InstanceRepo) CreateInstance(instance *model.Instance) error {
	query := `
		INSERT INTO instances (service_name, service_version, host_id, host_ip_address, ip_address, port, status, is_stopped)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`

	var id int
	err := r.db.QueryRow(query,
		instance.ServiceName,
		instance.ServiceVersion,
		instance.HostID,
		instance.HostIPAddress,
		instance.IPAddress,
		instance.Port,
		instance.Status,
		instance.IsStopped,
	).Scan(&id)

	if err != nil {
		return fmt.Errorf("failed to create instance: %w", err)
	}

	instance.ID = id
	return nil
}

// CreateVersionHistory 创建版本历史记录
func (r *InstanceRepo) CreateVersionHistory(versionHistory *model.VersionHistory) error {
	query := `
		INSERT INTO version_histories (instance_id, service_name, service_version, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`

	var id int
	err := r.db.QueryRow(query,
		versionHistory.InstanceID,
		versionHistory.ServiceName,
		versionHistory.ServiceVersion,
		versionHistory.Status,
	).Scan(&id)

	if err != nil {
		return fmt.Errorf("failed to create version history: %w", err)
	}

	versionHistory.ID = id
	return nil
}

// UpdateInstanceVersion 更新实例版本信息
func (r *InstanceRepo) UpdateInstanceVersion(instanceID, serviceName, serviceVersion string) error {
	// 更新instances表中的版本信息
	updateInstanceQuery := `
		UPDATE instances 
		SET service_version = $1 
		WHERE id = $2 AND service_name = $3
	`

	result, err := r.db.Exec(updateInstanceQuery, serviceVersion, instanceID, serviceName)
	if err != nil {
		return fmt.Errorf("failed to update instance version: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("instance %s not found or not updated", instanceID)
	}

	return nil
}

// CreateVersionHistoryForUpdate 为版本更新创建版本历史记录
func (r *InstanceRepo) CreateVersionHistoryForUpdate(instanceID, serviceName, serviceVersion string) error {
	// 创建新的版本历史记录
	versionHistory := &model.VersionHistory{
		InstanceID:     instanceID,
		ServiceName:    serviceName,
		ServiceVersion: serviceVersion,
		Status:         "active", // 新版本默认为active
	}

	return r.CreateVersionHistory(versionHistory)
}

// UpdateVersionStatus 更新版本状态
func (r *InstanceRepo) UpdateVersionStatus(instanceID, serviceName, serviceVersion, newStatus string) error {
	query := `
		UPDATE version_histories 
		SET status = $1 
		WHERE instance_id = $2 AND service_name = $3 AND service_version = $4
	`

	result, err := r.db.Exec(query, newStatus, instanceID, serviceName, serviceVersion)
	if err != nil {
		return fmt.Errorf("failed to update version status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("version history not found for instance %s, service %s, version %s", instanceID, serviceName, serviceVersion)
	}

	return nil
}

// GetCurrentActiveVersion 获取当前活跃版本
func (r *InstanceRepo) GetCurrentActiveVersion(instanceID, serviceName string) (string, error) {
	query := `
		SELECT service_version 
		FROM version_histories 
		WHERE instance_id = $1 AND service_name = $2 AND status = 'active'
		ORDER BY id DESC 
		LIMIT 1
	`

	var currentVersion string
	err := r.db.QueryRow(query, instanceID, serviceName).Scan(&currentVersion)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("no active version found for instance %s, service %s", instanceID, serviceName)
		}
		return "", fmt.Errorf("failed to get current active version: %w", err)
	}

	return currentVersion, nil
}

// CreateInstanceVersionHistory 创建实例版本历史记录
func (r *InstanceRepo) CreateInstanceVersionHistory(instanceID, serviceName, serviceVersion, status string) error {
	versionHistory := &model.VersionHistory{
		InstanceID:     instanceID,
		ServiceName:    serviceName,
		ServiceVersion: serviceVersion,
		Status:         status,
	}

	return r.CreateVersionHistory(versionHistory)
}

// GetExistingInstancePorts 查询指定服务在指定主机上已存在的实例端口列表
func (r *InstanceRepo) GetExistingInstancePorts(serviceName, instanceIP string) ([]int, error) {
	// 参数验证
	if serviceName == "" {
		return nil, fmt.Errorf("serviceName cannot be empty")
	}
	if instanceIP == "" {
		return nil, fmt.Errorf("instanceIP cannot be empty")
	}

	query := `
		SELECT port 
		FROM instances 
		WHERE service_name = $1 AND ip_address = $2 AND status = 'active'
		ORDER BY port ASC
	`

	rows, err := r.db.Query(query, serviceName, instanceIP)
	if err != nil {
		return nil, fmt.Errorf("failed to query existing instance ports for service %s on host %s: %w", serviceName, instanceIP, err)
	}
	defer rows.Close()

	var ports []int
	for rows.Next() {
		var port int
		if err := rows.Scan(&port); err != nil {
			return nil, fmt.Errorf("failed to scan port: %w", err)
		}
		ports = append(ports, port)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating existing ports: %w", err)
	}

	return ports, nil
}
