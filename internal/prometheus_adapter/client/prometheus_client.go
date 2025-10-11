package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	promModel "github.com/prometheus/common/model"
	"github.com/qiniu/zeroops/internal/prometheus_adapter/model"
	"github.com/rs/zerolog/log"
)

// PrometheusClient Prometheus 客户端
type PrometheusClient struct {
	api        v1.API
	httpClient *http.Client
	baseURL    string
}

// NewPrometheusClient 创建新的 Prometheus 客户端
func NewPrometheusClient(address string) (*PrometheusClient, error) {
	client, err := api.NewClient(api.Config{
		Address: address,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus client: %w", err)
	}

	return &PrometheusClient{
		api:        v1.NewAPI(client),
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    address,
	}, nil
}

// QueryRange 执行范围查询
func (c *PrometheusClient) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) ([]model.MetricDataPoint, error) {
	r := v1.Range{
		Start: start,
		End:   end,
		Step:  step,
	}

	result, warnings, err := c.api.QueryRange(ctx, query, r)
	if err != nil {
		return nil, fmt.Errorf("failed to query prometheus: %w", err)
	}

	if len(warnings) > 0 {
		// 记录警告但不返回错误
		fmt.Printf("Prometheus warnings: %v\n", warnings)
	}

	// 转换结果为我们的数据格式
	matrix, ok := result.(promModel.Matrix)
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}

	var dataPoints []model.MetricDataPoint
	for _, sample := range matrix {
		for _, pair := range sample.Values {
			dataPoints = append(dataPoints, model.MetricDataPoint{
				Timestamp: pair.Timestamp.Time(),
				Value:     float64(pair.Value),
			})
		}
	}

	return dataPoints, nil
}

// GetAvailableMetrics 获取所有可用的指标名称
func (c *PrometheusClient) GetAvailableMetrics(ctx context.Context) ([]string, error) {
	// 查询所有指标名称
	result, warnings, err := c.api.LabelValues(ctx, "__name__", nil, time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	if len(warnings) > 0 {
		fmt.Printf("Prometheus warnings: %v\n", warnings)
	}

	// 转换为字符串数组，过滤相关的指标
	metrics := make([]string, 0)
	for _, m := range result {
		metricName := string(m)
		metrics = append(metrics, metricName)
	}

	return metrics, nil
}

// CheckMetricExists 检查指标是否存在
func (c *PrometheusClient) CheckMetricExists(ctx context.Context, metric string) (bool, error) {
	// 查询指标是否存在
	query := fmt.Sprintf(`{__name__="%s"}`, metric)
	result, _, err := c.api.Query(ctx, query, time.Now())
	if err != nil {
		return false, fmt.Errorf("failed to check metric existence: %w", err)
	}

	// 如果有结果，说明指标存在
	switch v := result.(type) {
	case promModel.Vector:
		return len(v) > 0, nil
	case promModel.Matrix:
		return len(v) > 0, nil
	default:
		return false, nil
	}
}

// CheckServiceExists 检查服务是否存在
func (c *PrometheusClient) CheckServiceExists(ctx context.Context, service string) (bool, error) {
	// 查询服务是否存在
	query := fmt.Sprintf(`{service_name="%s"}`, service)
	result, _, err := c.api.Query(ctx, query, time.Now())
	if err != nil {
		return false, fmt.Errorf("failed to check service existence: %w", err)
	}

	// 如果有结果，说明服务存在
	switch v := result.(type) {
	case promModel.Vector:
		return len(v) > 0, nil
	case promModel.Matrix:
		return len(v) > 0, nil
	default:
		return false, nil
	}
}

// BuildQuery 构建 PromQL 查询
func BuildQuery(service, metric, version string) string {
	// 基础查询
	query := fmt.Sprintf(`%s{service_name="%s"`, metric, service)

	// 如果指定了版本，添加版本过滤
	if version != "" {
		query += fmt.Sprintf(`,service_version="%s"`, version)
	}

	query += "}"
	return query
}

// QueryRangeRaw 执行范围查询并返回原始结果（用于需要完整 Prometheus 响应的场景）
func (c *PrometheusClient) QueryRangeRaw(ctx context.Context, query string, start, end time.Time, step time.Duration) (promModel.Value, v1.Warnings, error) {
	r := v1.Range{
		Start: start,
		End:   end,
		Step:  step,
	}
	return c.api.QueryRange(ctx, query, r)
}

// GetAlerts 获取 Prometheus 当前的告警
func (c *PrometheusClient) GetAlerts(ctx context.Context) (*model.PrometheusAlertsResponse, error) {
	url := fmt.Sprintf("%s/api/v1/alerts", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query alerts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("prometheus returned status %d: %s", resp.StatusCode, string(body))
	}

	var alertsResp model.PrometheusAlertsResponse
	if err := json.NewDecoder(resp.Body).Decode(&alertsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Debug().
		Int("alert_count", len(alertsResp.Data.Alerts)).
		Msg("Retrieved alerts from Prometheus")

	return &alertsResp, nil
}
