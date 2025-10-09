package model

import "time"

// ServiceState 服务状态信息
type ServiceState struct {
	Service       string      `json:"service" db:"service"`               // varchar(255) - 联合PK
	Version       string      `json:"version" db:"version"`               // varchar(255) - 联合PK
	ReportAt      time.Time   `json:"reportAt" db:"report_at"`            // time - 报告时间
	ResolvedAt    *time.Time  `json:"resolvedAt" db:"resolved_at"`        // time - 解决时间
	HealthState   HealthState `json:"healthState" db:"health_state"`      // 健康状态
	CorrelationID string      `json:"alertIssueIds" db:"alert_issue_ids"` // varchar - 关联ID
}
