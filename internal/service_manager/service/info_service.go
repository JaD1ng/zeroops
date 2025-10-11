package service

import (
	"context"
	"fmt"
	"time"

	promModel "github.com/prometheus/common/model"
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
			DeployID:                fmt.Sprintf("%s-%s", serviceName, version), // 使用 service-version 组合
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
	// 检查 Prometheus 客户端是否可用
	if s.prometheusClient == nil {
		log.Warn().Msg("Prometheus client not available, returning mock data")
		return s.getMockMetricTimeSeries(serviceName, metricName, query), nil
	}

	// 解析时间参数
	start, err := time.Parse(time.RFC3339, query.Start)
	if err != nil {
		return nil, fmt.Errorf("invalid start time: %w", err)
	}
	end, err := time.Parse(time.RFC3339, query.End)
	if err != nil {
		return nil, fmt.Errorf("invalid end time: %w", err)
	}

	// 解析步长，默认为 15s
	step := 15 * time.Second
	if query.Granule != "" {
		step, err = time.ParseDuration(query.Granule)
		if err != nil {
			log.Warn().Err(err).Str("granule", query.Granule).Msg("Invalid granule, using default 15s")
		}
	}

	// 构建 PromQL 查询
	promQuery := buildPromQLQuery(metricName, serviceName, query.Version)

	log.Info().
		Str("query", promQuery).
		Str("start", start.String()).
		Str("end", end.String()).
		Str("step", step.String()).
		Msg("Querying Prometheus for time series data")

	// 使用 Prometheus 客户端的 QueryRangeRaw 方法获取完整的时序数据
	result, warnings, err := s.prometheusClient.QueryRangeRaw(ctx, promQuery, start, end, step)

	if err != nil {
		log.Error().Err(err).Msg("Failed to query Prometheus")
		return nil, fmt.Errorf("failed to query prometheus: %w", err)
	}

	if len(warnings) > 0 {
		log.Warn().Interface("warnings", warnings).Msg("Prometheus query returned warnings")
	}

	// 转换结果为我们的格式
	return convertToPrometheusQueryRangeResponse(result, metricName), nil
}

// getMockMetricTimeSeries 返回模拟数据（当 Prometheus 不可用时）
func (s *Service) getMockMetricTimeSeries(serviceName, metricName string, query *model.MetricTimeSeriesQuery) *model.PrometheusQueryRangeResponse {
	return &model.PrometheusQueryRangeResponse{
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
			},
		},
	}
}

// buildPromQLQuery 构建 PromQL 查询语句
func buildPromQLQuery(metricName, serviceName, version string) string {
	query := metricName + `{service_name="` + serviceName + `"`
	if version != "" {
		query += `,service_version="` + version + `"`
	}
	query += "}"
	return query
}

// convertToPrometheusQueryRangeResponse 转换 Prometheus 结果为响应格式
func convertToPrometheusQueryRangeResponse(result promModel.Value, metricName string) *model.PrometheusQueryRangeResponse {
	matrix, ok := result.(promModel.Matrix)
	if !ok {
		log.Warn().Msgf("Unexpected result type: %T", result)
		return &model.PrometheusQueryRangeResponse{
			Status: "success",
			Data: model.PrometheusQueryRangeData{
				ResultType: "matrix",
				Result:     []model.PrometheusTimeSeries{},
			},
		}
	}

	var timeSeries []model.PrometheusTimeSeries
	for _, sampleStream := range matrix {
		metrics := make(map[string]string)
		for k, v := range sampleStream.Metric {
			metrics[string(k)] = string(v)
		}

		var values [][]any
		for _, pair := range sampleStream.Values {
			values = append(values, []any{
				float64(pair.Timestamp) / 1000, // 转换为秒
				fmt.Sprintf("%f", pair.Value),
			})
		}

		timeSeries = append(timeSeries, model.PrometheusTimeSeries{
			Metric: metrics,
			Values: values,
		})
	}

	return &model.PrometheusQueryRangeResponse{
		Status: "success",
		Data: model.PrometheusQueryRangeData{
			ResultType: "matrix",
			Result:     timeSeries,
		},
	}
}

// GetServiceMetricStats 获取服务指标统计
func (s *Service) GetServiceMetricStats(ctx context.Context, serviceName string) (*model.ServiceMetricStatsResponse, error) {
	// 检查 Prometheus 客户端是否可用
	if s.prometheusClient == nil {
		log.Warn().Msg("Prometheus client not available, returning mock data")
		return s.getMockMetricStats(ctx, serviceName)
	}

	// 获取服务的活跃版本
	instances, err := s.db.GetServiceInstances(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	// 按版本分组
	versionMap := make(map[string]bool)
	for _, instance := range instances {
		versionMap[instance.Version] = true
	}

	// 如果没有版本，返回空数据
	if len(versionMap) == 0 {
		return &model.ServiceMetricStatsResponse{
			Summary: model.MetricSummary{
				Metrics: []model.MetricItem{
					{Name: "latency", Value: 0},
					{Name: "traffic", Value: 0},
					{Name: "errorRatio", Value: 0},
					{Name: "saturation", Value: 0},
				},
			},
			Items: []model.VersionMetricStats{},
		}, nil
	}

	// 构建版本指标统计
	var items []model.VersionMetricStats
	var totalLatency, totalTraffic, totalErrorRatio, totalSaturation float64
	versionCount := 0

	for version := range versionMap {
		// 从 Prometheus 获取各个版本的指标
		metrics, err := s.getMetricsFromPrometheus(ctx, serviceName, version)
		if err != nil {
			log.Warn().Err(err).Str("version", version).Msg("Failed to get metrics from Prometheus, using defaults")
			// 使用默认值
			metrics = []model.MetricItem{
				{Name: "latency", Value: 0},
				{Name: "traffic", Value: 0},
				{Name: "errorRatio", Value: 0},
				{Name: "saturation", Value: 0},
			}
		}

		items = append(items, model.VersionMetricStats{
			Version: version,
			Metrics: metrics,
		})

		// 累计聚合数据
		for _, m := range metrics {
			switch m.Name {
			case "latency":
				totalLatency += m.Value
			case "traffic":
				totalTraffic += m.Value
			case "errorRatio":
				totalErrorRatio += m.Value
			case "saturation":
				totalSaturation += m.Value
			}
		}
		versionCount++
	}

	// 计算聚合值（平均值或总和，根据指标类型）
	summary := model.MetricSummary{
		Metrics: []model.MetricItem{
			{Name: "latency", Value: totalLatency / float64(versionCount)},       // 延迟取平均
			{Name: "traffic", Value: totalTraffic},                               // 流量取总和
			{Name: "errorRatio", Value: totalErrorRatio / float64(versionCount)}, // 错误率取平均
			{Name: "saturation", Value: totalSaturation / float64(versionCount)}, // 饱和度取平均
		},
	}

	response := &model.ServiceMetricStatsResponse{
		Summary: summary,
		Items:   items,
	}

	return response, nil
}

// getMetricsFromPrometheus 从 Prometheus 获取指标数据
func (s *Service) getMetricsFromPrometheus(ctx context.Context, serviceName, version string) ([]model.MetricItem, error) {
	now := time.Now()

	// 定义要查询的指标
	metricQueries := map[string]string{
		// 1. latency: 使用 http_latency_seconds histogram 的 P95 延迟，转换为毫秒
		"latency": fmt.Sprintf(`histogram_quantile(0.95, sum(rate(http_latency_seconds_bucket{service_name="%s",service_version="%s"}[5m])) by (le)) * 1000`, serviceName, version),

		// 2. traffic: 使用 system_network_qps_per_second
		"traffic": fmt.Sprintf(`avg(system_network_qps_per_second{service_name="%s",service_version="%s"})`, serviceName, version),

		// 3. errorRatio: 5xx 错误率百分比
		"errorRatio": fmt.Sprintf(`(sum(rate(http_latency_seconds_count{service_name="%s",service_version="%s",http_status_code=~"5.."}[5m])) / sum(rate(http_latency_seconds_count{service_name="%s",service_version="%s"}[5m]))) * 100 or vector(0)`, serviceName, version, serviceName, version),

		// 4. saturation: CPU 使用率百分比
		"saturation": fmt.Sprintf(`avg(system_cpu_usage_percent{service_name="%s",service_version="%s"})`, serviceName, version),
	}

	metrics := make([]model.MetricItem, 0, 4)

	for name, query := range metricQueries {
		result, _, err := s.prometheusClient.QueryRangeRaw(ctx, query, now.Add(-5*time.Minute), now, 30*time.Second)
		if err != nil {
			log.Warn().Err(err).Str("metric", name).Str("query", query).Msg("Failed to query metric")
			metrics = append(metrics, model.MetricItem{Name: name, Value: 0})
			continue
		}

		// 提取最新值
		value := extractLatestValue(result)
		metrics = append(metrics, model.MetricItem{
			Name:  name,
			Value: value,
		})
	}

	return metrics, nil
}

// extractLatestValue 从 Prometheus 结果中提取最新值
func extractLatestValue(result promModel.Value) float64 {
	switch v := result.(type) {
	case promModel.Matrix:
		if len(v) > 0 && len(v[0].Values) > 0 {
			// 取最后一个值
			lastValue := v[0].Values[len(v[0].Values)-1]
			return float64(lastValue.Value)
		}
	case promModel.Vector:
		if len(v) > 0 {
			return float64(v[0].Value)
		}
	case *promModel.Scalar:
		return float64(v.Value)
	}
	return 0
}

// getMockMetricStats 返回模拟数据（当 Prometheus 不可用时）
func (s *Service) getMockMetricStats(ctx context.Context, serviceName string) (*model.ServiceMetricStatsResponse, error) {
	// 获取服务的活跃版本
	instances, err := s.db.GetServiceInstances(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	// 按版本分组
	versionMap := make(map[string]bool)
	for _, instance := range instances {
		versionMap[instance.Version] = true
	}

	// 构建版本指标统计（模拟数据）
	var items []model.VersionMetricStats
	var totalLatency, totalTraffic, totalErrorRatio, totalSaturation float64
	versionCount := 0

	for version := range versionMap {
		latency := 10.0 + float64(versionCount)*2.0
		traffic := 1000.0 + float64(versionCount)*100.0
		errorRatio := 5.0 + float64(versionCount)*2.0
		saturation := 50.0 + float64(versionCount)*5.0

		items = append(items, model.VersionMetricStats{
			Version: version,
			Metrics: []model.MetricItem{
				{Name: "latency", Value: latency},
				{Name: "traffic", Value: traffic},
				{Name: "errorRatio", Value: errorRatio},
				{Name: "saturation", Value: saturation},
			},
		})

		totalLatency += latency
		totalTraffic += traffic
		totalErrorRatio += errorRatio
		totalSaturation += saturation
		versionCount++
	}

	summary := model.MetricSummary{
		Metrics: []model.MetricItem{
			{Name: "latency", Value: totalLatency / float64(versionCount)},
			{Name: "traffic", Value: totalTraffic},
			{Name: "errorRatio", Value: totalErrorRatio / float64(versionCount)},
			{Name: "saturation", Value: totalSaturation / float64(versionCount)},
		},
	}

	if versionCount == 0 {
		summary = model.MetricSummary{
			Metrics: []model.MetricItem{
				{Name: "latency", Value: 0},
				{Name: "traffic", Value: 0},
				{Name: "errorRatio", Value: 0},
				{Name: "saturation", Value: 0},
			},
		}
		items = []model.VersionMetricStats{}
	}

	return &model.ServiceMetricStatsResponse{
		Summary: summary,
		Items:   items,
	}, nil
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
