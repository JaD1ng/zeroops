package model

// DeployNewServiceParams 部署新服务参数
type DeployNewServiceParams struct {
	Service    string   `json:"service"`     // 必填，服务名称
	Version    string   `json:"version"`     // 必填，目标版本号
	Hosts      []string `json:"hosts"`       // 必填，主机名称列表
	PackageURL string   `json:"package_url"` // 必填，包下载URL
}

// DeployNewVersionParams 发布参数
type DeployNewVersionParams struct {
	Service    string   `json:"service"`     // 必填，服务名称
	Version    string   `json:"version"`     // 必填，目标版本号
	Instances  []string `json:"instances"`   // 必填，实例ID列表
	PackageURL string   `json:"package_url"` // 必填，包下载URL
}

// RollbackParams 回滚参数
type RollbackParams struct {
	Service       string   `json:"service"`        // 必填，服务名称
	TargetVersion string   `json:"target_version"` // 必填，目标版本号
	Instances     []string `json:"instances"`      // 必填，实例ID列表
	PackageURL    string   `json:"package_url"`    // 必填，包下载URL
}

// OperationResult 操作结果
type OperationResult struct {
	Service        string   `json:"service"`         // 服务名称
	Version        string   `json:"version"`         // 操作的目标版本
	Instances      []string `json:"instances"`       // 实际操作的实例ID列表
	TotalInstances int      `json:"total_instances"` // 操作的实例总数
}
