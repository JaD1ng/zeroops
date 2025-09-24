package model

import "time"

// ===== 指标相关 API =====

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

// ===== 告警规则相关 API =====

// CreateAlertRuleRequest 创建告警规则请求
type CreateAlertRuleRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description,omitempty"`
	Expr        string `json:"expr" binding:"required"`
	Op          string `json:"op" binding:"required,oneof=> < = !="`
	Severity    string `json:"severity" binding:"required"`

	// 元信息字段（可选）
	Labels    map[string]string `json:"labels,omitempty"`
	Threshold float64           `json:"threshold,omitempty"`
	WatchTime int               `json:"watch_time,omitempty"`
	MatchTime string            `json:"match_time,omitempty"`
}

// UpdateAlertRuleRequest 更新告警规则请求
type UpdateAlertRuleRequest struct {
	Description string `json:"description,omitempty"`
	Expr        string `json:"expr,omitempty"`
	Op          string `json:"op,omitempty" binding:"omitempty,oneof=> < = !="`
	Severity    string `json:"severity,omitempty"`
}

// CreateAlertRuleMetaRequest 创建告警规则元信息请求
type CreateAlertRuleMetaRequest struct {
	AlertName string            `json:"alert_name" binding:"required"`
	Labels    map[string]string `json:"labels" binding:"required"`
	Threshold float64           `json:"threshold" binding:"required"`
	WatchTime int               `json:"watch_time,omitempty"`
	MatchTime string            `json:"match_time,omitempty"`
}

// UpdateAlertRuleMetaRequest 更新告警规则元信息请求
type UpdateAlertRuleMetaRequest struct {
	AlertName string  `json:"alert_name" binding:"required"`
	Labels    string  `json:"labels" binding:"required"`
	Threshold float64 `json:"threshold"`
	WatchTime int     `json:"watch_time"`
}

// SyncRulesRequest 同步规则请求
// 从监控告警模块发送过来的完整规则列表
type SyncRulesRequest struct {
	Rules     []AlertRule     `json:"rules"`      // 告警规则列表
	RuleMetas []AlertRuleMeta `json:"rule_metas"` // 规则元信息列表
}
