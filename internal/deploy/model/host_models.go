package model

// HostInfo 主机信息
type HostInfo struct {
	HostID        string `json:"host_id"`         // 主机ID
	HostName      string `json:"host_name"`       // 主机名称
	HostIPAddress string `json:"host_ip_address"` // 主机IP地址
	IsStopped     bool   `json:"is_stopped"`      // 主机启动/停止判断
}
