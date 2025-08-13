package utils

import (
	"crypto/rand"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// GenerateUUID 生成UUID字符串
func GenerateUUID() string {
	return uuid.New().String()
}

// GenerateShortID 生成短ID（8位）
func GenerateShortID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")[:8]
}

// GenerateFileID 生成文件ID（去掉连字符的UUID）
func GenerateFileID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}

// IsValidUUID 验证UUID格式
func IsValidUUID(str string) bool {
	_, err := uuid.Parse(str)
	return err == nil
}

// GenerateRequestID 生成请求ID
func GenerateRequestID() string {
	return "req-" + GenerateShortID()
}

// GenerateSecureToken 生成安全令牌
func GenerateSecureToken(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("token length must be positive")
	}
	
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate secure token: %w", err)
	}
	
	// 转换为十六进制字符串
	return fmt.Sprintf("%x", bytes), nil
}