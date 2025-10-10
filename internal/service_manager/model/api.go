package model

import "time"

// ===== 服务基础信息结构体 =====

// ServiceItem API响应用的服务信息（对应/v1/services接口items格式）
type ServiceItem struct {
	Name        string      `json:"name"`        // 服务名称
	DeployState DeployState `json:"deployState"` // 发布状态
	Health      HealthState `json:"health"`      // 健康状态：Normal/Warning/Error
	Deps        []string    `json:"deps"`        // 依赖关系（直接使用Service.Deps）
}

// ServicesResponse 服务列表API响应（对应/v1/services接口）
type ServicesResponse struct {
	Items    []ServiceItem       `json:"items"`
	Relation map[string][]string `json:"relation"` // 树形关系描述，有向无环图
}

// ActiveVersionItem 活跃版本项目
type ActiveVersionItem struct {
	Version                 string      `json:"version"`                 // v1.0.1
	DeployID                string      `json:"deployID"`                // 1001
	StartTime               time.Time   `json:"startTime"`               // 开始时间
	EstimatedCompletionTime time.Time   `json:"estimatedCompletionTime"` // 预估完成时间
	Instances               int         `json:"instances"`               // 实例个数
	Health                  HealthState `json:"health"`                  // 健康状态：Normal/Warning/Error
}

// PrometheusQueryRangeResponse Prometheus query_range接口响应格式
type PrometheusQueryRangeResponse struct {
	Status string                   `json:"status"`
	Data   PrometheusQueryRangeData `json:"data"`
}

// PrometheusQueryRangeData Prometheus响应数据
type PrometheusQueryRangeData struct {
	ResultType string                 `json:"resultType"`
	Result     []PrometheusTimeSeries `json:"result"`
}

// PrometheusTimeSeries Prometheus时序数据
type PrometheusTimeSeries struct {
	Metric map[string]string `json:"metric"`
	Values [][]any           `json:"values"` // [timestamp, value]数组
}

// MetricTimeSeriesQuery 时序指标查询参数
type MetricTimeSeriesQuery struct {
	Service string `form:"service" binding:"required"`
	Name    string `form:"name" binding:"required"`
	Version string `form:"version,omitempty"`
	Start   string `form:"start" binding:"required"` // RFC3339格式时间
	End     string `form:"end" binding:"required"`   // RFC3339格式时间
	Granule string `form:"granule,omitempty"`        // 1m/5m/1h等
}

// ===== 部署任务操作结构体 =====

// Deployment API响应用的发布任务
type Deployment struct {
	Service      string      `json:"service"`
	Version      string      `json:"version"`
	Status       DeployState `json:"status"`
	ScheduleTime *time.Time  `json:"scheduleTime,omitempty"`
	FinishTime   *time.Time  `json:"finishTime,omitempty"`
}

// CreateDeploymentRequest 创建发布任务请求
type CreateDeploymentRequest struct {
	Service       string     `json:"service" binding:"required"`
	Version       string     `json:"version" binding:"required"`
	ScheduleTime  *time.Time `json:"scheduleTime,omitempty"`  // 可选参数，不填为立即发布
	InstanceCount int        `json:"instanceCount,omitempty"` // 可选参数，新服务部署时的实例数量
	PackageURL    string     `json:"packageUrl,omitempty"`    // 可选参数，包下载URL，不填则自动构建
}

// UpdateDeploymentRequest 修改发布任务请求
type UpdateDeploymentRequest struct {
	Version      string     `json:"version,omitempty"`
	ScheduleTime *time.Time `json:"scheduleTime,omitempty"` // 新的计划发布时间
}

// RollbackDeploymentRequest 回滚发布任务请求
type RollbackDeploymentRequest struct {
	TargetVersion string `json:"targetVersion" binding:"required"` // 回滚目标版本
	PackageURL    string `json:"packageUrl,omitempty"`             // 可选参数，回滚包URL，不填则自动构建
}

// DeploymentQuery 发布任务查询参数
type DeploymentQuery struct {
	Type    DeployState `form:"type"`    // deploying/stop/rollback/completed
	Service string      `form:"service"` // 服务名称过滤
	Start   string      `form:"start"`   // 分页起始
	Limit   int         `form:"limit"`   // 分页大小
}
