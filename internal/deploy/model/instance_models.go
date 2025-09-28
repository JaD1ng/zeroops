package model

// InstanceInfo 实例信息
type InstanceInfo struct {
	InstanceID     string `json:"instance_id"`     // 实例唯一标识符
	ServiceName    string `json:"service_name"`    // 所属服务名称
	ServiceVersion string `json:"service_version"` // 当前运行的版本号
	Status         string `json:"status"`          // 实例运行状态 - 'active'运行中；'pending'发布中；'error'出现故障
}

// VersionInfo 版本信息
type VersionInfo struct {
	Version string `json:"version"` // 版本号
	Status  string `json:"status"`  // 版本状态 - 'active'当前运行版本；'stable'稳定版本；'deprecated'已废弃版本
}

// Instance 实例数据库模型
type Instance struct {
	ID             string `json:"id"`              // 实例唯一标识符（必填）
	ServiceName    string `json:"service_name"`    // 服务名称
	ServiceVersion string `json:"service_version"` // 服务版本
	HostID         string `json:"host_id"`         // 主机ID
	HostIPAddress  string `json:"host_ip_address"` // 主机IP地址
	IPAddress      string `json:"ip_address"`      // 实例IP地址
	Port           int    `json:"port"`            // 实例端口
	Status         string `json:"status"`          // 实例状态
	IsStopped      bool   `json:"is_stopped"`      // 是否已停止
}

// VersionHistory 版本历史数据库模型
type VersionHistory struct {
	ID             int    `json:"id"`              // 自增主键
	InstanceID     string `json:"instance_id"`     // 实例ID
	ServiceName    string `json:"service_name"`    // 服务名称
	ServiceVersion string `json:"service_version"` // 服务版本
	Status         string `json:"status"`          // 版本状态
}
