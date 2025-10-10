package service

import (
	"context"

	"github.com/qiniu/zeroops/internal/service_manager/model"
	"github.com/rs/zerolog/log"
)

// ===== 服务信息查询方法 =====

// GetServicesResponse 获取服务列表响应
func (s *Service) GetServicesResponse(ctx context.Context) (*model.ServicesResponse, error) {
	services, err := s.db.GetServices(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]model.ServiceItem, len(services))
	relation := make(map[string][]string)

	for i, service := range services {
		// 获取服务状态来确定健康状态
		state, err := s.db.GetServiceState(ctx, service.Name)
		if err != nil {
			log.Error().Err(err).Str("service", service.Name).Msg("failed to get service state")
		}

		// 默认为正常状态，因为正常状态的服务不会存储在service_state表中
		health := model.HealthStateNormal
		if state != nil {
			health = state.HealthState
		}

		// 默认设置为已完成部署状态
		deployState := model.StatusCompleted

		items[i] = model.ServiceItem{
			Name:        service.Name,
			DeployState: deployState,
			Health:      health,
			Deps:        service.Deps,
		}

		// 构建依赖关系图
		if len(service.Deps) > 0 {
			relation[service.Name] = service.Deps
		}
	}

	return &model.ServicesResponse{
		Items:    items,
		Relation: relation,
	}, nil
}

// GetServiceActiveVersions 获取服务活跃版本
func (s *Service) GetServiceActiveVersions(ctx context.Context, serviceName string) ([]model.ActiveVersionItem, error) {
	instances, err := s.db.GetServiceInstances(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	// 按版本分组统计实例
	versionMap := make(map[string][]model.ServiceInstance)
	for _, instance := range instances {
		versionMap[instance.Version] = append(versionMap[instance.Version], instance)
	}

	var activeVersions []model.ActiveVersionItem
	for version, versionInstances := range versionMap {
		// 获取服务状态
		state, err := s.db.GetServiceStateAndVersion(ctx, serviceName, version)
		if err != nil {
			log.Error().Err(err).Str("service", serviceName).Msg("failed to get service state")
		}

		// 默认为正常状态，因为正常状态的服务不会存储在service_state表中
		health := model.HealthStateNormal
		reportAt := &model.ServiceState{}
		if state != nil {
			health = state.HealthState
			reportAt = state
		}

		activeVersion := model.ActiveVersionItem{
			Version:                 version,
			DeployID:                "1001", // TODO:临时值，实际需要从部署任务中获取
			StartTime:               reportAt.ReportAt,
			EstimatedCompletionTime: reportAt.ReportAt,
			Instances:               len(versionInstances),
			Health:                  health,
		}

		activeVersions = append(activeVersions, activeVersion)
	}

	return activeVersions, nil
}

// GetServiceAvailableVersions 获取可用服务版本
func (s *Service) GetServiceAvailableVersions(ctx context.Context, serviceName, versionType string) ([]model.ServiceVersion, error) {
	// 获取所有版本
	versions, err := s.db.GetServiceVersions(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	// TODO:根据类型过滤（这里简化处理，实际需要根据业务需求过滤）
	if versionType == "unrelease" {
		// 返回未发布的版本，这里简化返回所有版本
		return versions, nil
	}

	return versions, nil
}

// GetServiceMetricTimeSeries 获取服务时序指标数据
func (s *Service) GetServiceMetricTimeSeries(ctx context.Context, serviceName, metricName string, query *model.MetricTimeSeriesQuery) (*model.PrometheusQueryRangeResponse, error) {
	// TODO:这里应该调用实际的Prometheus或其他监控系统API
	// 现在返回模拟数据

	response := &model.PrometheusQueryRangeResponse{
		Status: "success",
		Data: model.PrometheusQueryRangeData{
			ResultType: "matrix",
			Result: []model.PrometheusTimeSeries{
				{
					Metric: map[string]string{
						"__name__": metricName,
						"service":  serviceName,
						"instance": "instance-1",
						"version":  query.Version,
					},
					Values: [][]any{
						{1435781430.781, "1.2"},
						{1435781445.781, "1.5"},
						{1435781460.781, "1.1"},
					},
				},
				{
					Metric: map[string]string{
						"__name__": metricName,
						"service":  serviceName,
						"instance": "instance-2",
						"version":  query.Version,
					},
					Values: [][]any{
						{1435781430.781, "0.8"},
						{1435781445.781, "0.9"},
						{1435781460.781, "1.0"},
					},
				},
			},
		},
	}

	return response, nil
}

// ===== 服务管理CRUD方法 =====

// CreateService 创建服务
func (s *Service) CreateService(ctx context.Context, service *model.Service) error {
	return s.db.CreateService(ctx, service)
}

// UpdateService 更新服务信息
func (s *Service) UpdateService(ctx context.Context, service *model.Service) error {
	return s.db.UpdateService(ctx, service)
}

// DeleteService 删除服务
func (s *Service) DeleteService(ctx context.Context, name string) error {
	return s.db.DeleteService(ctx, name)
}
