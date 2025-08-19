package logger

import (
	"fmt"
	"net"
	"os"
)

// GetHostID 获取主机ID
// 优先使用环境变量 HOST_ID，如果没有则使用主机名
func GetHostID() string {
	// 首先尝试从环境变量获取
	if hostID := os.Getenv("HOST_ID"); hostID != "" {
		return hostID
	}

	// 如果没有环境变量，使用主机名
	hostname, err := os.Hostname()
	if err != nil {
		// 如果获取主机名失败，使用默认值
		return "unknown-host"
	}

	// 获取本机IP地址作为后缀，确保唯一性
	localIP := getLocalIP()
	if localIP != "" {
		return fmt.Sprintf("%s-%s", hostname, localIP)
	}

	return hostname
}

// getLocalIP 获取本机IP地址
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	return ""
}
