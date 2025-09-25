package model

import (
	"time"
)

// PrometheusAlert Prometheus 告警 API 响应结构
type PrometheusAlert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	State       string            `json:"state"` // pending, firing
	ActiveAt    time.Time         `json:"activeAt"`
	Value       string            `json:"value"` // 触发告警时的值
}

// PrometheusAlertsResponse Prometheus /api/v1/alerts 响应
type PrometheusAlertsResponse struct {
	Status string `json:"status"`
	Data   struct {
		Alerts []PrometheusAlert `json:"alerts"`
	} `json:"data"`
}

// AlertmanagerWebhookAlert 单个告警
type AlertmanagerWebhookAlert struct {
	Status       string            `json:"status"`       // "firing" or "resolved"
	Labels       map[string]string `json:"labels"`       // 包含 alertname, service, severity, idc, service_version 等
	Annotations  map[string]string `json:"annotations"`  // 包含 summary, description
	StartsAt     string            `json:"startsAt"`     // RFC3339 格式时间
	EndsAt       string            `json:"endsAt"`       // RFC3339 格式时间
	GeneratorURL string            `json:"generatorURL"` // Prometheus 查询链接
	Fingerprint  string            `json:"fingerprint"`  // 告警唯一标识
}

// AlertmanagerWebhookRequest 发送到监控告警模块的请求格式
type AlertmanagerWebhookRequest struct {
	Receiver     string                     `json:"receiver"` // "our-webhook"
	Status       string                     `json:"status"`   // "firing" or "resolved"
	Alerts       []AlertmanagerWebhookAlert `json:"alerts"`
	GroupLabels  map[string]string          `json:"groupLabels"`  // 分组标签
	CommonLabels map[string]string          `json:"commonLabels"` // 公共标签
	Version      string                     `json:"version"`      // "4"
}

// AlertWebhookResponse 告警推送响应
type AlertWebhookResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
