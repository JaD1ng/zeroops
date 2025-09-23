package model

// AlertRule 告警规则表 - 定义告警规则模板
type AlertRule struct {
	Name        string `json:"name" gorm:"type:varchar(255);primaryKey"`
	Description string `json:"description" gorm:"type:text"`
	Expr        string `json:"expr" gorm:"type:text;not null"`
	Op          string `json:"op" gorm:"type:enum('>', '<', '=', '!=');not null"`
	Severity    string `json:"severity" gorm:"type:varchar(50);not null"`
}

// AlertRuleMeta 告警规则元信息表 - 存储服务级别的告警配置
// 用于将告警规则模板实例化为具体的服务告警
type AlertRuleMeta struct {
	AlertName string  `json:"alert_name" gorm:"type:varchar(255);primaryKey"`
	Labels    string  `json:"labels" gorm:"type:text"`     // JSON格式的服务标签，如：{"service":"storage-service","version":"1.0.0"}
	Threshold float64 `json:"threshold"`                   // 告警阈值
	WatchTime int     `json:"watch_time"`                  // 持续时间（秒），对应Prometheus的for字段
	MatchTime string  `json:"match_time" gorm:"type:text"` // 时间范围表达式
}
