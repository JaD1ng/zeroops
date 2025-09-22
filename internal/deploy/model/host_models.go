package model

// HostInfo 主机信息
type HostInfo struct {
	HostName string `json:"host_name"` // 主机名称
	HostIP   string `json:"host_ip"`   // 主机IP地址
}
