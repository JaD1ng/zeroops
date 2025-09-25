package model

// AlertRule 告警规则表 - 定义告警规则模板
type AlertRule struct {
	Name        string `json:"name" gorm:"type:varchar(255);primaryKey"`  // 主键，告警规则名称
	Description string `json:"description" gorm:"type:text"`              // 可读标题，可拼接渲染为可读的 title
	Expr        string `json:"expr" gorm:"type:text;not null"`            // 左侧业务指标表达式，如 sum(apitime) by (service, version)
	Op          string `json:"op" gorm:"type:varchar(4);not null"`        // 阈值比较方式（>, <, =, !=）
	Severity    string `json:"severity" gorm:"type:varchar(32);not null"` // 告警等级，通常进入告警的 labels.severity
	WatchTime   int    `json:"watch_time"`                                // 持续时长（秒），映射 Prometheus rule 的 for 字段
}

// AlertRuleMeta 告警规则元信息表 - 存储服务级别的告警配置
// 用于将告警规则模板实例化为具体的服务告警
type AlertRuleMeta struct {
	AlertName string  `json:"alert_name" gorm:"type:varchar(255);index"` // 关联 alert_rules.name
	Labels    string  `json:"labels" gorm:"type:jsonb"`                  // 适用标签，如 {"service":"s3","version":"v1"}，为空表示全局
	Threshold float64 `json:"threshold"`                                 // 阈值（会被渲染成特定规则的 threshold metric 数值）
}

// AlertmanagerAlert 符合 Alertmanager API v2 的告警格式
type AlertmanagerAlert struct {
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations,omitempty"`
	StartsAt     string            `json:"startsAt,omitempty"` // RFC3339 格式
	EndsAt       string            `json:"endsAt,omitempty"`   // RFC3339 格式
	GeneratorURL string            `json:"generatorURL,omitempty"`
}
