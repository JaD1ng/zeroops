package models

import (
	"time"
)

// MetricAnomalyRule 指标异常注入规则
type MetricAnomalyRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Service     string `json:"service"`            // 目标服务
	Instance    string `json:"instance,omitempty"` // 目标实例，可选
	MetricName  string `json:"metric_name"`        // 目标指标名称
	AnomalyType string `json:"anomaly_type"`
	Enabled     bool   `json:"enabled"`

	// 异常参数
	TargetValue float64       `json:"target_value"` // 目标异常值
	Duration    time.Duration `json:"duration"`     // 持续时间

	// 时间控制
	StartTime   *time.Time `json:"start_time,omitempty"`
	MaxTriggers int        `json:"max_triggers"`
	Triggered   int        `json:"triggered"`
	CreatedAt   time.Time  `json:"created_at"`
}

// 指标异常类型
const (
	// 监控指标异常
	AnomalyCPUSpike     = "cpu_spike"     // cpu异常
	AnomalyMemoryLeak   = "memory_leak"   // 内存泄露
	AnomalyDiskFull     = "disk_full"     // 磁盘容量异常
	AnomalyNetworkFlood = "network_flood" // 网络异常
	AnomalyMachineDown  = "machine_down"  // 机器宕机
)

// 预定义指标名称
const (
	MetricCPUUsage      = "system_cpu_usage_percent"
	MetricMemoryUsage   = "system_memory_usage_percent"
	MetricDiskUsage     = "system_disk_usage_percent"
	MetricNetworkQPS    = "system_network_qps"
	MetricMachineStatus = "system_machine_online_status"
)

// 使用示例：
//
// CPU峰值异常规则:
// {
//   "id": "cpu_spike_001",
//   "name": "cpu异常",
//   "service": "storage-service",
//   "metric_name": "system_cpu_usage_percent",
//   "anomaly_type": "cpu_spike",
//   "target_value": 95.0,
//   "duration": 300
// }
//
// 内存泄露异常规则:
// {
//   "id": "mem_leak_001",
//   "name": "内存泄露",
//   "service": "metadata-service",
//   "metric_name": "system_memory_usage_percent",
//   "anomaly_type": "memory_leak",
//   "target_value": 92.5,
//   "duration": 600
// }
