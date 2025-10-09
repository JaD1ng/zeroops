package model

// HealthState 健康状态枚举
type HealthState string

const (
	HealthStateNormal  HealthState = "Normal"
	HealthStateWarning HealthState = "Warning"
	HealthStateError   HealthState = "Error"
)

// DeployState 发布状态枚举
type DeployState string

const (
	StatusUnrelease DeployState = "unrelease" // 未发布/待发布
	StatusDeploying DeployState = "deploying" // 正在发布
	StatusStop      DeployState = "stop"      // 暂停发布
	StatusRollback  DeployState = "rollback"  // 已回滚
	StatusCompleted DeployState = "completed" // 发布完成
	StatusError     DeployState = "error"     // 发布失败
)
