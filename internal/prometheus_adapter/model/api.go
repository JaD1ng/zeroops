package model

import "time"

// ===== API 响应结构体 =====

// MetricListResponse 指标列表响应（对应 GET /v1/metrics）
type MetricListResponse struct {
	Metrics []string `json:"metrics"`
}

// MetricQueryResponse 指标查询响应（对应 GET /v1/metrics/:service/:metric）
type MetricQueryResponse struct {
	Service string            `json:"service"`
	Version string            `json:"version,omitempty"`
	Metric  string            `json:"metric"`
	Data    []MetricDataPoint `json:"data"`
}

// MetricDataPoint 指标数据点
type MetricDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}
