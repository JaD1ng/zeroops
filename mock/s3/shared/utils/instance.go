package utils

import (
	"crypto/rand"
	"fmt"
	"os"
	"strings"
	"sync"
)

var (
	cachedInstanceID string
	instanceIDMutex  sync.RWMutex
)

// GetInstanceID 获取实例ID，优先级：
// 1. 环境变量 INSTANCE_ID
// 2. 自动生成：{service_name}-{short_uuid}
// 3. 后备方案：hostname
func GetInstanceID(serviceName string) string {
	instanceIDMutex.RLock()
	if cachedInstanceID != "" {
		instanceIDMutex.RUnlock()
		return cachedInstanceID
	}
	instanceIDMutex.RUnlock()

	instanceIDMutex.Lock()
	defer instanceIDMutex.Unlock()

	// 双重检查，避免重复计算
	if cachedInstanceID != "" {
		return cachedInstanceID
	}

	// 1. 优先使用环境变量
	if instanceID := os.Getenv("INSTANCE_ID"); instanceID != "" {
		cachedInstanceID = instanceID
		return cachedInstanceID
	}

	// 2. 自动生成基于服务名的实例ID
	if serviceName != "" {
		if generatedID := generateInstanceID(serviceName); generatedID != "" {
			cachedInstanceID = generatedID
			return cachedInstanceID
		}
	}

	// 3. 后备方案：使用hostname
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		cachedInstanceID = hostname
		return cachedInstanceID
	}

	// 4. 最后的后备方案
	cachedInstanceID = "unknown-instance"
	return cachedInstanceID
}

// generateInstanceID 生成格式为 {service_name}-{short_uuid} 的实例ID
func generateInstanceID(serviceName string) string {
	// 清理服务名：移除常见后缀，转换为小写
	cleanServiceName := cleanServiceName(serviceName)

	// 生成8位短UUID
	shortUUID := generateShortUUID()
	if shortUUID == "" {
		return ""
	}

	return fmt.Sprintf("%s-%s", cleanServiceName, shortUUID)
}

// cleanServiceName 清理服务名
func cleanServiceName(serviceName string) string {
	name := strings.ToLower(serviceName)

	// 移除常见后缀
	suffixes := []string{"-service", "_service", "service"}
	for _, suffix := range suffixes {
		if strings.HasSuffix(name, suffix) {
			name = strings.TrimSuffix(name, suffix)
			break
		}
	}

	// 替换特殊字符为连字符
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, " ", "-")

	return name
}

// generateShortUUID 生成8位短UUID
func generateShortUUID() string {
	bytes := make([]byte, 4) // 4字节 = 8位十六进制字符
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return fmt.Sprintf("%x", bytes)
}

// ResetInstanceID 重置缓存的实例ID（主要用于测试）
func ResetInstanceID() {
	instanceIDMutex.Lock()
	defer instanceIDMutex.Unlock()
	cachedInstanceID = ""
}
