package service

import (
	"context"
	"time"

	"github.com/qiniu/zeroops/internal/prometheus_adapter/client"
	"github.com/qiniu/zeroops/internal/prometheus_adapter/model"
	"github.com/rs/zerolog/log"
)

// MetricService 指标服务
type MetricService struct {
	promClient *client.PrometheusClient
}

// NewMetricService 创建指标服务
func NewMetricService(promClient *client.PrometheusClient) *MetricService {
	return &MetricService{
		promClient: promClient,
	}
}

// GetAvailableMetrics 获取可用的指标列表
func (s *MetricService) GetAvailableMetrics(ctx context.Context) (*model.MetricListResponse, error) {
	// 从 Prometheus 动态获取指标列表
	metrics, err := s.promClient.GetAvailableMetrics(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to get available metrics from prometheus")
		return nil, &model.PrometheusError{Message: err.Error()}
	}

	return &model.MetricListResponse{
		Metrics: metrics,
	}, nil
}

// QueryMetric 查询指标数据
func (s *MetricService) QueryMetric(ctx context.Context, service, metric, version string, start, end time.Time, step time.Duration) (*model.MetricQueryResponse, error) {
	// 动态验证服务是否存在
	serviceExists, err := s.promClient.CheckServiceExists(ctx, service)
	if err != nil {
		log.Error().Err(err).Str("service", service).Msg("failed to check service existence")
		return nil, &model.PrometheusError{Message: err.Error()}
	}
	if !serviceExists {
		return nil, &model.ServiceNotFoundError{Service: service}
	}

	// 动态验证指标是否存在
	metricExists, err := s.promClient.CheckMetricExists(ctx, metric)
	if err != nil {
		log.Error().Err(err).Str("metric", metric).Msg("failed to check metric existence")
		return nil, &model.PrometheusError{Message: err.Error()}
	}
	if !metricExists {
		return nil, &model.MetricNotFoundError{Metric: metric}
	}

	// 构建 PromQL 查询
	query := client.BuildQuery(service, metric, version)
	log.Debug().Str("query", query).Msg("executing prometheus query")

	// 执行查询
	dataPoints, err := s.promClient.QueryRange(ctx, query, start, end, step)
	if err != nil {
		log.Error().Err(err).Str("query", query).Msg("failed to query prometheus")
		return nil, &model.PrometheusError{Message: err.Error()}
	}

	// 构建响应
	response := &model.MetricQueryResponse{
		Service: service,
		Version: version,
		Metric:  metric,
		Data:    dataPoints,
	}

	return response, nil
}
