package model

// InstanceInfo 实例信息
type InstanceInfo struct {
	InstanceID     string `json:"instance_id"`  // 实例唯一标识符
	ServiceName    string `json:"service_name"` // 所属服务名称
	ServiceVersion string `json:"version"`      // 当前运行的版本号
	Status         string `json:"status"`       // 实例运行状态 - 'active'运行中；'pending'发布中；'error'出现故障
}

// VersionInfo 版本信息
type VersionInfo struct {
	Version string `json:"version"` // 版本号
	Status  string `json:"status"`  // 版本状态 - 'active'当前运行版本；'stable'稳定版本；'deprecated'已废弃版本
}
