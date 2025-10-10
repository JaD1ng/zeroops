package model

import "time"

// ServiceDeployTask 服务部署任务信息
type ServiceDeployTask struct {
	Service     string      `json:"service" db:"service"`          // varchar(255) - 服务名称（联合主键）
	Version     string      `json:"version" db:"version"`          // varchar(255) - 版本号（联合主键）
	StartTime   *time.Time  `json:"startTime" db:"start_time"`     // time - 开始时间
	EndTime     *time.Time  `json:"endTime" db:"end_time"`         // time - 结束时间
	TargetRatio float64     `json:"targetRatio" db:"target_ratio"` // double(指导值) - 目标比例
	Instances   []string    `json:"instances" db:"instances"`      // array(真实发布的节点列表) - 实例列表
	DeployState DeployState `json:"deployState" db:"deploy_state"` // 部署状态
}
