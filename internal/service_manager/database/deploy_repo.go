package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"time"

	"github.com/qiniu/zeroops/internal/service_manager/model"
)

// CreateDeployment 创建发布任务
func (d *Database) CreateDeployment(ctx context.Context, req *model.CreateDeploymentRequest) error {
	// 根据是否有计划时间决定初始状态
	var initialStatus model.DeployState
	if req.ScheduleTime == nil {
		initialStatus = model.StatusDeploying // 立即发布
	} else {
		initialStatus = model.StatusUnrelease // 计划发布
	}

	query := `INSERT INTO deploy_tasks (service, version, start_time, end_time, target_ratio, instances, deploy_state)
	          VALUES ($1, $2, $3, $4, $5, $6, $7)`

	// 默认实例为空数组
	instances := []string{}
	instancesJSON, _ := json.Marshal(instances)

	_, err := d.ExecContext(ctx, query, req.Service, req.Version, req.ScheduleTime, nil, 0.0, string(instancesJSON), initialStatus)
	return err
}

// UpdateDeploymentStatus 更新部署任务状态
func (d *Database) UpdateDeploymentStatus(ctx context.Context, service, version string, status model.DeployState) error {
	query := `UPDATE deploy_tasks SET deploy_state = $1 WHERE service = $2 AND version = $3`
	_, err := d.ExecContext(ctx, query, status, service, version)
	return err
}

// UpdateDeploymentFinishTime 更新部署任务完成时间
func (d *Database) UpdateDeploymentFinishTime(ctx context.Context, service, version string, finishTime time.Time) error {
	query := `UPDATE deploy_tasks SET end_time = $1 WHERE service = $2 AND version = $3`
	_, err := d.ExecContext(ctx, query, finishTime, service, version)
	return err
}

// GetDeploymentByServiceAndVersion 根据服务名和版本获取发布任务详情
func (d *Database) GetDeploymentByServiceAndVersion(ctx context.Context, service, version string) (*model.Deployment, error) {
	query := `SELECT service, version, start_time, end_time, target_ratio, instances, deploy_state
	          FROM deploy_tasks WHERE service = $1 AND version = $2`
	row := d.QueryRowContext(ctx, query, service, version)

	var task model.ServiceDeployTask
	var instancesJSON string
	if err := row.Scan(&task.Service, &task.Version, &task.StartTime, &task.EndTime, &task.TargetRatio,
		&instancesJSON, &task.DeployState); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	// 解析实例JSON数组
	if instancesJSON != "" {
		if err := json.Unmarshal([]byte(instancesJSON), &task.Instances); err != nil {
			return nil, err
		}
	}

	deployment := &model.Deployment{
		Service:      task.Service,
		Version:      task.Version,
		Status:       task.DeployState,
		ScheduleTime: task.StartTime,
		FinishTime:   task.EndTime,
	}

	return deployment, nil
}

// GetDeployments 获取发布任务列表
func (d *Database) GetDeployments(ctx context.Context, query *model.DeploymentQuery) ([]model.Deployment, error) {
	sql := `SELECT service, version, start_time, end_time, target_ratio, instances, deploy_state
	        FROM deploy_tasks WHERE 1=1`
	args := []any{}

	if query.Type != "" {
		sql += " AND deploy_state = $" + strconv.Itoa(len(args)+1)
		args = append(args, query.Type)
	}

	if query.Service != "" {
		sql += " AND service = $" + strconv.Itoa(len(args)+1)
		args = append(args, query.Service)
	}

	sql += " ORDER BY start_time DESC"

	if query.Limit > 0 {
		sql += " LIMIT $" + strconv.Itoa(len(args)+1)
		args = append(args, query.Limit)
	}

	rows, err := d.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deployments []model.Deployment
	for rows.Next() {
		var task model.ServiceDeployTask
		var instancesJSON string
		if err := rows.Scan(&task.Service, &task.Version, &task.StartTime, &task.EndTime, &task.TargetRatio,
			&instancesJSON, &task.DeployState); err != nil {
			return nil, err
		}

		// 解析实例JSON数组
		if instancesJSON != "" {
			if err := json.Unmarshal([]byte(instancesJSON), &task.Instances); err != nil {
				return nil, err
			}
		}

		deployment := model.Deployment{
			Service:      task.Service,
			Version:      task.Version,
			Status:       task.DeployState,
			ScheduleTime: task.StartTime,
			FinishTime:   task.EndTime,
		}

		deployments = append(deployments, deployment)
	}

	return deployments, rows.Err()
}

// UpdateDeployment 修改未开始的发布任务
func (d *Database) UpdateDeployment(ctx context.Context, service, version string, req *model.UpdateDeploymentRequest) error {
	sql := `UPDATE deploy_tasks SET `
	args := []any{}
	updates := []string{}
	paramIndex := 1

	if req.ScheduleTime != nil {
		updates = append(updates, "start_time = $"+strconv.Itoa(paramIndex))
		args = append(args, req.ScheduleTime)
		paramIndex++
	}

	if len(updates) == 0 {
		return nil
	}

	sql += updates[0]
	for i := 1; i < len(updates); i++ {
		sql += ", " + updates[i]
	}

	sql += " WHERE service = $" + strconv.Itoa(paramIndex) + " AND version = $" + strconv.Itoa(paramIndex+1) + " AND deploy_state = $" + strconv.Itoa(paramIndex+2)
	args = append(args, service, version, model.StatusUnrelease)

	_, err := d.ExecContext(ctx, sql, args...)
	return err
}

// DeleteDeployment 删除未开始的发布任务
func (d *Database) DeleteDeployment(ctx context.Context, service, version string) error {
	query := `DELETE FROM deploy_tasks WHERE service = $1 AND version = $2 AND deploy_state = $3`
	_, err := d.ExecContext(ctx, query, service, version, model.StatusUnrelease)
	return err
}

// PauseDeployment 暂停正在灰度的发布任务
func (d *Database) PauseDeployment(ctx context.Context, service, version string) error {
	query := `UPDATE deploy_tasks SET deploy_state = $1 WHERE service = $2 AND version = $3 AND deploy_state = $4`
	_, err := d.ExecContext(ctx, query, model.StatusStop, service, version, model.StatusDeploying)
	return err
}

// ContinueDeployment 继续发布
func (d *Database) ContinueDeployment(ctx context.Context, service, version string) error {
	query := `UPDATE deploy_tasks SET deploy_state = $1 WHERE service = $2 AND version = $3 AND deploy_state = $4`
	_, err := d.ExecContext(ctx, query, model.StatusDeploying, service, version, model.StatusStop)
	return err
}

// RollbackDeployment 回滚发布任务
func (d *Database) RollbackDeployment(ctx context.Context, service, version string) error {
	query := `UPDATE deploy_tasks SET deploy_state = $1 WHERE service = $2 AND version = $3`
	_, err := d.ExecContext(ctx, query, model.StatusRollback, service, version)
	return err
}

// CheckDeploymentConflict 检查发布冲突
func (d *Database) CheckDeploymentConflict(ctx context.Context, service, version string) (bool, error) {
	// 检查是否已存在相同服务和版本的部署任务
	query := `SELECT COUNT(*) FROM deploy_tasks WHERE service = $1 AND version = $2 AND deploy_state IN ($3, $4, $5)`
	var count int
	err := d.QueryRowContext(ctx, query, service, version, model.StatusDeploying, model.StatusUnrelease, model.StatusStop).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
