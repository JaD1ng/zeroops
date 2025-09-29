package service

import (
	"context"
	"time"

	"github.com/qiniu/zeroops/internal/service_manager/model"
	"github.com/rs/zerolog/log"
)

// ===== 部署管理业务方法 =====

// CreateDeployment 创建发布任务
func (s *Service) CreateDeployment(ctx context.Context, req *model.CreateDeploymentRequest) (*model.Deployment, error) {
	// 检查服务是否存在
	service, err := s.db.GetServiceByName(ctx, req.Service)
	if err != nil {
		return nil, err
	}
	if service == nil {
		return nil, ErrServiceNotFound
	}

	// 检查发布冲突
	conflict, err := s.db.CheckDeploymentConflict(ctx, req.Service, req.Version)
	if err != nil {
		return nil, err
	}
	if conflict {
		return nil, ErrDeploymentConflict
	}

	// 创建发布任务记录
	err = s.db.CreateDeployment(ctx, req)
	if err != nil {
		return nil, err
	}

	log.Info().
		Str("service", req.Service).
		Str("version", req.Version).
		Msg("deployment created successfully")

	// 异步执行实际部署
	go s.executeActualDeployment(ctx, req)

	// 返回创建的部署信息
	deployment := &model.Deployment{
		Service:      req.Service,
		Version:      req.Version,
		Status:       model.StatusDeploying,
		ScheduleTime: req.ScheduleTime,
	}

	return deployment, nil
}

// executeActualDeployment 执行实际部署操作
func (s *Service) executeActualDeployment(ctx context.Context, req *model.CreateDeploymentRequest) {
	// 捕获panic，防止goroutine崩溃
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Str("service", req.Service).Str("version", req.Version).Msg("deployment execution panic")
			s.db.UpdateDeploymentStatus(ctx, req.Service, req.Version, model.StatusError)
		}
	}()

	// 1. 更新状态为 deploying
	if err := s.db.UpdateDeploymentStatus(ctx, req.Service, req.Version, model.StatusDeploying); err != nil {
		log.Error().Err(err).Str("service", req.Service).Str("version", req.Version).Msg("failed to update deployment status to deploying")
		return
	}

	// 2. 判断是新服务部署还是版本升级
	isNewService, err := s.deployAdapter.IsNewServiceDeployment(req.Service)
	if err != nil {
		log.Error().Err(err).Str("service", req.Service).Str("version", req.Version).Msg("failed to determine deployment type")
		s.db.UpdateDeploymentStatus(ctx, req.Service, req.Version, model.StatusError)
		return
	}

	var result interface{}

	if isNewService {
		// 新服务部署
		params, err := s.deployAdapter.BuildDeployNewServiceParams(req)
		if err != nil {
			log.Error().Err(err).Str("service", req.Service).Str("version", req.Version).Msg("failed to build deploy new service params")
			s.db.UpdateDeploymentStatus(ctx, req.Service, req.Version, model.StatusError)
			return
		}

		// 验证包URL
		if err := s.deployAdapter.ValidatePackageURL(params.PackageURL); err != nil {
			log.Error().Err(err).Str("service", req.Service).Str("version", req.Version).Str("packageURL", params.PackageURL).Msg("package validation failed")
			s.db.UpdateDeploymentStatus(ctx, req.Service, req.Version, model.StatusError)
			return
		}

		result, err = s.deployService.DeployNewService(params)
		if err != nil {
			log.Error().Err(err).Str("service", req.Service).Str("version", req.Version).Msg("deploy new service failed")
			s.db.UpdateDeploymentStatus(ctx, req.Service, req.Version, model.StatusError)
			return
		}
	} else {
		// 版本升级
		params, err := s.deployAdapter.BuildDeployNewVersionParams(req)
		if err != nil {
			log.Error().Err(err).Str("service", req.Service).Str("version", req.Version).Msg("failed to build deploy new version params")
			s.db.UpdateDeploymentStatus(ctx, req.Service, req.Version, model.StatusError)
			return
		}

		// 验证包URL
		if err := s.deployAdapter.ValidatePackageURL(params.PackageURL); err != nil {
			log.Error().Err(err).Str("service", req.Service).Str("version", req.Version).Str("packageURL", params.PackageURL).Msg("package validation failed")
			s.db.UpdateDeploymentStatus(ctx, req.Service, req.Version, model.StatusError)
			return
		}

		result, err = s.deployService.DeployNewVersion(params)
		if err != nil {
			log.Error().Err(err).Str("service", req.Service).Str("version", req.Version).Msg("deploy new version failed")
			s.db.UpdateDeploymentStatus(ctx, req.Service, req.Version, model.StatusError)
			return
		}
	}

	// 3. 部署成功，更新状态
	if err := s.db.UpdateDeploymentStatus(ctx, req.Service, req.Version, model.StatusCompleted); err != nil {
		log.Error().Err(err).Str("service", req.Service).Str("version", req.Version).Msg("failed to update deployment status to completed")
		return
	}

	// 4. 更新完成时间
	if err := s.db.UpdateDeploymentFinishTime(ctx, req.Service, req.Version, time.Now()); err != nil {
		log.Error().Err(err).Str("service", req.Service).Str("version", req.Version).Msg("failed to update deployment finish time")
	}

	log.Info().
		Str("service", req.Service).
		Str("version", req.Version).
		Interface("result", result).
		Msg("deployment executed successfully")
}

// GetDeploymentByServiceAndVersion 根据服务名和版本获取发布任务详情
func (s *Service) GetDeploymentByServiceAndVersion(ctx context.Context, service, version string) (*model.Deployment, error) {
	deployment, err := s.db.GetDeploymentByServiceAndVersion(ctx, service, version)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, ErrDeploymentNotFound
	}
	return deployment, nil
}

// GetDeployments 获取发布任务列表
func (s *Service) GetDeployments(ctx context.Context, query *model.DeploymentQuery) ([]model.Deployment, error) {
	return s.db.GetDeployments(ctx, query)
}

// UpdateDeployment 修改发布任务
func (s *Service) UpdateDeployment(ctx context.Context, service, version string, req *model.UpdateDeploymentRequest) error {
	// 检查部署任务是否存在
	deployment, err := s.db.GetDeploymentByServiceAndVersion(ctx, service, version)
	if err != nil {
		return err
	}
	if deployment == nil {
		return ErrDeploymentNotFound
	}

	// 只有unrelease状态的任务可以修改
	if deployment.Status != model.StatusUnrelease {
		return ErrInvalidDeployState
	}

	err = s.db.UpdateDeployment(ctx, service, version, req)
	if err != nil {
		return err
	}

	log.Info().
		Str("service", service).
		Str("version", version).
		Msg("deployment updated successfully")

	return nil
}

// DeleteDeployment 删除发布任务
func (s *Service) DeleteDeployment(ctx context.Context, service, version string) error {
	// 检查部署任务是否存在
	deployment, err := s.db.GetDeploymentByServiceAndVersion(ctx, service, version)
	if err != nil {
		return err
	}
	if deployment == nil {
		return ErrDeploymentNotFound
	}

	// 只有未开始的任务可以删除
	if deployment.Status != model.StatusUnrelease {
		return ErrInvalidDeployState
	}

	err = s.db.DeleteDeployment(ctx, service, version)
	if err != nil {
		return err
	}

	log.Info().
		Str("service", service).
		Str("version", version).
		Msg("deployment deleted successfully")

	return nil
}

// PauseDeployment 暂停发布任务
func (s *Service) PauseDeployment(ctx context.Context, service, version string) error {
	// 检查部署任务是否存在且为正在部署状态
	deployment, err := s.db.GetDeploymentByServiceAndVersion(ctx, service, version)
	if err != nil {
		return err
	}
	if deployment == nil {
		return ErrDeploymentNotFound
	}
	if deployment.Status != model.StatusDeploying {
		return ErrInvalidDeployState
	}

	err = s.db.PauseDeployment(ctx, service, version)
	if err != nil {
		return err
	}

	log.Info().
		Str("service", service).
		Str("version", version).
		Msg("deployment paused successfully")

	return nil
}

// ContinueDeployment 继续发布任务
func (s *Service) ContinueDeployment(ctx context.Context, service, version string) error {
	// 检查部署任务是否存在且为暂停状态
	deployment, err := s.db.GetDeploymentByServiceAndVersion(ctx, service, version)
	if err != nil {
		return err
	}
	if deployment == nil {
		return ErrDeploymentNotFound
	}
	if deployment.Status != model.StatusStop {
		return ErrInvalidDeployState
	}

	err = s.db.ContinueDeployment(ctx, service, version)
	if err != nil {
		return err
	}

	log.Info().
		Str("service", service).
		Str("version", version).
		Msg("deployment continued successfully")

	return nil
}

// RollbackDeployment 回滚发布任务
func (s *Service) RollbackDeployment(ctx context.Context, service, version string, req *model.RollbackDeploymentRequest) error {
	// 检查部署任务是否存在
	deployment, err := s.db.GetDeploymentByServiceAndVersion(ctx, service, version)
	if err != nil {
		return err
	}
	if deployment == nil {
		return ErrDeploymentNotFound
	}

	// 只有正在部署或暂停的任务可以回滚
	if deployment.Status != model.StatusDeploying && deployment.Status != model.StatusStop {
		return ErrInvalidDeployState
	}

	// 异步执行实际回滚
	go s.executeActualRollback(ctx, deployment, req)

	// 更新数据库状态
	err = s.db.RollbackDeployment(ctx, service, version)
	if err != nil {
		return err
	}

	log.Info().
		Str("service", service).
		Str("version", version).
		Str("targetVersion", req.TargetVersion).
		Msg("deployment rollback initiated successfully")

	return nil
}

// executeActualRollback 执行实际回滚操作
func (s *Service) executeActualRollback(_ context.Context, deployment *model.Deployment, req *model.RollbackDeploymentRequest) {
	// 捕获panic，防止goroutine崩溃
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Str("service", deployment.Service).Str("version", deployment.Version).Msg("rollback execution panic")
		}
	}()

	// 1. 构建回滚参数
	params, err := s.deployAdapter.BuildRollbackParams(deployment, req)
	if err != nil {
		log.Error().Err(err).Str("service", deployment.Service).Str("version", deployment.Version).Msg("failed to build rollback params")
		return
	}

	// 2. 验证回滚包URL
	if err := s.deployAdapter.ValidatePackageURL(params.PackageURL); err != nil {
		log.Error().Err(err).Str("service", deployment.Service).Str("version", deployment.Version).Str("packageURL", params.PackageURL).Msg("rollback package validation failed")
		return
	}

	// 3. 执行回滚
	result, err := s.deployService.ExecuteRollback(params)
	if err != nil {
		log.Error().Err(err).Str("service", deployment.Service).Str("version", deployment.Version).Msg("execute rollback failed")
		return
	}

	log.Info().
		Str("service", deployment.Service).
		Str("version", deployment.Version).
		Interface("result", result).
		Msg("rollback executed successfully")
}
