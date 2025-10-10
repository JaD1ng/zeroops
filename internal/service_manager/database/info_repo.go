package database

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/qiniu/zeroops/internal/service_manager/model"
)

// GetServices 获取所有服务列表
func (d *Database) GetServices(ctx context.Context) ([]model.Service, error) {
	query := `SELECT name, deps FROM services`
	rows, err := d.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []model.Service
	for rows.Next() {
		var service model.Service
		var depsJSON string
		if err := rows.Scan(&service.Name, &depsJSON); err != nil {
			return nil, err
		}

		if depsJSON != "" {
			if err := json.Unmarshal([]byte(depsJSON), &service.Deps); err != nil {
				return nil, err
			}
		}

		services = append(services, service)
	}

	return services, rows.Err()
}

// GetServiceByName 根据名称获取服务信息
func (d *Database) GetServiceByName(ctx context.Context, name string) (*model.Service, error) {
	query := `SELECT name, deps FROM services WHERE name = $1`
	row := d.QueryRowContext(ctx, query, name)

	var service model.Service
	var depsJSON string
	if err := row.Scan(&service.Name, &depsJSON); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if depsJSON != "" {
		if err := json.Unmarshal([]byte(depsJSON), &service.Deps); err != nil {
			return nil, err
		}
	}

	return &service, nil
}

// CreateService 创建服务
func (d *Database) CreateService(ctx context.Context, service *model.Service) error {
	depsJSON, err := json.Marshal(service.Deps)
	if err != nil {
		return err
	}

	query := `INSERT INTO services (name, deps) VALUES ($1, $2)`
	_, err = d.ExecContext(ctx, query, service.Name, string(depsJSON))
	return err
}

// UpdateService 更新服务信息
func (d *Database) UpdateService(ctx context.Context, service *model.Service) error {
	depsJSON, err := json.Marshal(service.Deps)
	if err != nil {
		return err
	}

	query := `UPDATE services SET deps = $1 WHERE name = $2`
	_, err = d.ExecContext(ctx, query, string(depsJSON), service.Name)
	return err
}

// DeleteService 删除服务
func (d *Database) DeleteService(ctx context.Context, name string) error {
	query := `DELETE FROM services WHERE name = $1`
	_, err := d.ExecContext(ctx, query, name)
	return err
}

// ===== 服务版本操作 =====

// GetServiceVersions 获取服务版本列表
func (d *Database) GetServiceVersions(ctx context.Context, serviceName string) ([]model.ServiceVersion, error) {
	query := `SELECT version, service, create_time FROM service_versions WHERE service = $1 ORDER BY create_time DESC`
	rows, err := d.QueryContext(ctx, query, serviceName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []model.ServiceVersion
	for rows.Next() {
		var version model.ServiceVersion
		if err := rows.Scan(&version.Version, &version.Service, &version.CreateTime); err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}

	return versions, rows.Err()
}

// CreateServiceVersion 创建服务版本
func (d *Database) CreateServiceVersion(ctx context.Context, version *model.ServiceVersion) error {
	query := `INSERT INTO service_versions (version, service, create_time) VALUES ($1, $2, $3)`
	_, err := d.ExecContext(ctx, query, version.Version, version.Service, version.CreateTime)
	return err
}

// ===== 服务实例操作 =====

// GetServiceInstances 获取服务实例列表
func (d *Database) GetServiceInstances(ctx context.Context, serviceName string) ([]model.ServiceInstance, error) {
	query := `SELECT id, service, version FROM service_instances WHERE service = $1`
	rows, err := d.QueryContext(ctx, query, serviceName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instances []model.ServiceInstance
	for rows.Next() {
		var instance model.ServiceInstance
		if err := rows.Scan(&instance.ID, &instance.Service, &instance.Version); err != nil {
			return nil, err
		}
		instances = append(instances, instance)
	}

	return instances, rows.Err()
}

// CreateServiceInstance 创建服务实例
func (d *Database) CreateServiceInstance(ctx context.Context, instance *model.ServiceInstance) error {
	query := `INSERT INTO service_instances (id, service, version) VALUES ($1, $2, $3)`
	_, err := d.ExecContext(ctx, query, instance.ID, instance.Service, instance.Version)
	return err
}

// ===== 服务状态操作 =====

// GetServiceState 获取服务状态
func (d *Database) GetServiceState(ctx context.Context, serviceName string) (*model.ServiceState, error) {
	query := `SELECT service, version, report_at, resolved_at, health_state, alert_issue_ids
	          FROM service_states WHERE service = $1 ORDER BY report_at DESC LIMIT 1`
	row := d.QueryRowContext(ctx, query, serviceName)

	var state model.ServiceState
	if err := row.Scan(&state.Service, &state.Version, &state.ReportAt,
		&state.ResolvedAt, &state.HealthState, &state.CorrelationID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &state, nil
}

func (d *Database) GetServiceStateAndVersion(ctx context.Context, serviceName string, version string) (*model.ServiceState, error) {
	query := `SELECT service, version, report_at, resolved_at, health_state, correlation_id
	          FROM service_states WHERE service = $1 AND version = $2 ORDER BY report_at DESC LIMIT 1`
	row := d.QueryRowContext(ctx, query, serviceName, version)

	var state model.ServiceState
	if err := row.Scan(&state.Service, &state.Version, &state.ReportAt,
		&state.ResolvedAt, &state.HealthState, &state.CorrelationID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &state, nil
}
