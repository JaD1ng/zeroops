package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// Config 结构体，映射 config.yaml 配置
// 包含 Prometheus 区域映射和 Superset 相关配置
// 可根据需要扩展
type Config struct {
	Global struct {
		DefaultFilePath string `yaml:"default_file_path"`
	} `yaml:"global"`
	Prometheus struct {
		Regions  map[string]string `yaml:"regions"`
		Port     int               `yaml:"port"`
		Endpoint string            `yaml:"endpoint"`
	} `yaml:"prometheus"`
	Superset struct {
		BaseURL  string `yaml:"base_url"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		Port     int    `yaml:"port"`
		Endpoint string `yaml:"endpoint"`
	} `yaml:"superset"`
	ElasticSearch struct {
		Port     int    `yaml:"port"`
		Endpoint string `yaml:"endpoint"`
	} `yaml:"elasticsearch"`
}

// GlobalConfig 全局变量，保存配置内容
var GlobalConfig Config

// projectRoot 项目根目录路径
var projectRoot string

// init 初始化项目根目录
func init() {
	// 获取当前可执行文件所在目录
	execPath, err := os.Executable()
	if err != nil {
		// 如果获取可执行文件路径失败，使用当前工作目录
		if workDir, err := os.Getwd(); err == nil {
			projectRoot = workDir
		}
		return
	}

	// 获取可执行文件所在目录
	execDir := filepath.Dir(execPath)

	// 如果是开发环境（go run），可执行文件在临时目录，需要特殊处理
	if filepath.Base(execPath) == "go" || filepath.Base(execPath) == "main" {
		// 开发环境，使用当前工作目录
		if workDir, err := os.Getwd(); err == nil {
			projectRoot = workDir
		}
	} else {
		// 生产环境，使用可执行文件所在目录
		projectRoot = execDir
	}
}

// GetProjectRoot 获取项目根目录
func GetProjectRoot() string {
	return projectRoot
}

// LoadConfig 读取 config.yaml 并解析到 Config 结构体
func LoadConfig(path string) error {
	// 使用 os.ReadFile 读取文件内容，io.ReadFile 在 Go 1.16+ 已被移至 os 包
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}
	// 解析 YAML 数据到全局配置结构体
	if err := yaml.Unmarshal(data, &GlobalConfig); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}
	return nil
}

// GetDefaultFilePath 获取默认文件保存路径
// 如果路径不存在则自动创建
// 返回值：
// - 默认文件路径的绝对路径
// - 错误信息
func GetDefaultFilePath() (string, error) {
	// 获取配置的默认路径
	configPath := GlobalConfig.Global.DefaultFilePath
	if configPath == "" {
		// 如果配置为空，使用项目根目录下的dataset
		configPath = "testdata"
	}

	// 使用项目根目录作为基准
	basePath := GetProjectRoot()
	if basePath == "" {
		// 如果项目根目录为空，使用当前工作目录
		var err error
		basePath, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("获取项目根目录失败: %w", err)
		}
	}

	// 构建完整路径
	var fullPath string
	if filepath.IsAbs(configPath) {
		// 如果是绝对路径，直接使用
		fullPath = configPath
	} else {
		// 如果是相对路径，相对于项目根目录
		fullPath = filepath.Join(basePath, configPath)
	}

	// 确保目录存在
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败 %s: %w", fullPath, err)
	}

	return fullPath, nil
}

// GetFilePath 根据文件名获取完整的文件路径
// 参数：
// - filename: 文件名
// 返回值：
// - 完整的文件路径
// - 错误信息
func GetFilePath(filename string) (string, error) {
	basePath, err := GetDefaultFilePath()
	if err != nil {
		return "", err
	}

	// 组合完整路径
	fullPath := filepath.Join(basePath, filename)

	return fullPath, nil
}
