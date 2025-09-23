package service

import (
	"fmt"
	"math/rand"
	"net"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/qiniu/zeroops/internal/deploy/model"

	"github.com/qiniu/zeroops/internal/deploy/config"
	"github.com/qiniu/zeroops/internal/deploy/database"
)

var (
	dbInstance   *database.Database
	instanceRepo *database.InstanceRepo
	hostRepo     *database.HostRepo
	dbOnce       sync.Once
	dbErr        error
)

// initDatabase 初始化数据库连接（单例模式）
func initDatabase() (*database.Database, error) {
	dbOnce.Do(func() {
		cfg, err := config.LoadConfig("internal/deploy/config.yaml")
		if err != nil {
			dbErr = fmt.Errorf("failed to load config: %w", err)
			return
		}

		dbInstance, dbErr = database.NewDatabase(&cfg.Database)
		if dbErr != nil {
			dbErr = fmt.Errorf("failed to initialize database: %w", dbErr)
			return
		}

		// 初始化实例仓库
		instanceRepo = database.NewInstanceRepo(dbInstance)

		// 初始化主机仓库
		hostRepo = database.NewHostRepo(dbInstance)
	})

	return dbInstance, dbErr
}

// ValidatePackageURL 验证是否能通过URL找到包
func ValidatePackageURL(packageURL string) error {
	// TODO: 实现包URL验证逻辑
	return nil
}

// GetServiceInstanceInfos 根据服务名和版本获取实例信息列表，用于内部批量操作
func GetServiceInstanceInfos(serviceName string, version ...string) ([]*model.InstanceInfo, error) {
	// 参数验证
	if serviceName == "" {
		return nil, fmt.Errorf("serviceName cannot be empty")
	}

	// 获取数据库连接
	_, err := initDatabase()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database connection: %w", err)
	}

	// 根据是否有版本参数选择不同的查询方法
	if len(version) > 0 && version[0] != "" {
		// 有版本参数，按服务名和版本查询
		instances, err := instanceRepo.GetInstanceInfosByServiceNameAndVersion(serviceName, version[0])
		if err != nil {
			return nil, fmt.Errorf("failed to get instances by service name and version: %w", err)
		}
		return instances, nil
	} else {
		// 无版本参数，只按服务名查询
		instances, err := instanceRepo.GetInstanceInfosByServiceName(serviceName)
		if err != nil {
			return nil, fmt.Errorf("failed to get instances by service name: %w", err)
		}
		return instances, nil
	}
}

// GetInstanceIP 根据实例ID获取实例的IP地址
func GetInstanceIP(instanceID string) (string, error) {
	// 参数验证
	if instanceID == "" {
		return "", fmt.Errorf("instanceID cannot be empty")
	}

	// 验证instanceID是否为有效的数字（因为数据库中id是SERIAL类型）
	if _, err := strconv.Atoi(instanceID); err != nil {
		return "", fmt.Errorf("invalid instanceID format: %s, must be a number", instanceID)
	}

	// 获取数据库连接
	_, err := initDatabase()
	if err != nil {
		return "", fmt.Errorf("failed to initialize database connection: %w", err)
	}

	// 查询实例IP地址
	ipAddress, err := instanceRepo.GetInstanceIPByInstanceID(instanceID)
	if err != nil {
		return "", fmt.Errorf("failed to get instance IP for ID %s: %w", instanceID, err)
	}

	return ipAddress, nil
}

// GetInstancePort 根据实例ID获取实例的端口号
func GetInstancePort(instanceID string) (int, error) {
	// 参数验证
	if instanceID == "" {
		return 0, fmt.Errorf("instanceID cannot be empty")
	}

	// 验证instanceID是否为有效的数字（因为数据库中id是SERIAL类型）
	if _, err := strconv.Atoi(instanceID); err != nil {
		return 0, fmt.Errorf("invalid instanceID format: %s, must be a number", instanceID)
	}

	// 获取数据库连接
	_, err := initDatabase()
	if err != nil {
		return 0, fmt.Errorf("failed to initialize database connection: %w", err)
	}

	// 查询实例端口号
	port, err := instanceRepo.GetInstancePortByInstanceID(instanceID)
	if err != nil {
		return 0, fmt.Errorf("failed to get instance port for ID %s: %w", instanceID, err)
	}

	return port, nil
}

// CheckInstanceHealth 检查单个实例是否有响应，用于发布前验证目标实例的可用性
func CheckInstanceHealth(instanceIP string, instancePort int) (bool, error) {
	// 参数验证
	if instanceIP == "" {
		return false, fmt.Errorf("instanceIP cannot be empty")
	}
	if instancePort <= 0 || instancePort > 65535 {
		return false, fmt.Errorf("invalid port number: %d", instancePort)
	}

	// 尝试连接实例的IP和端口
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", instanceIP, instancePort), 5*time.Second)
	if err != nil {
		return false, nil // 连接失败，实例不健康
	}
	defer conn.Close()

	return true, nil // 连接成功，实例健康
}

func GetAvailableHostInfos() ([]*model.HostInfo, error) {
	// 获取数据库连接
	_, err := initDatabase()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database connection: %w", err)
	}

	// 查询可用主机信息列表
	hostInfos, err := hostRepo.GetAvailableHostInfos()
	if err != nil {
		return nil, fmt.Errorf("failed to get available host infos: %w", err)
	}

	return hostInfos, nil
}

// CheckHostHealth 判断主机运行状态
func CheckHostHealth(hostIpAddress string) (bool, error) {
	// 参数验证
	if hostIpAddress == "" {
		return false, fmt.Errorf("hostIpAddress cannot be empty")
	}

	// 验证IP地址格式
	if net.ParseIP(hostIpAddress) == nil {
		return false, fmt.Errorf("invalid IP address format: %s", hostIpAddress)
	}

	// 使用ping命令检查主机是否可达
	cmd := exec.Command("ping", "-c", "1", "-W", "3", hostIpAddress)
	err := cmd.Run()
	if err != nil {
		return false, nil // ping失败，主机不可达
	}

	return true, nil // ping成功，主机健康
}

// SelectHostForNewInstance 为新实例选择合适的主机
func SelectHostForNewInstance(availableHosts []*model.HostInfo, service string, version string) (*model.HostInfo, error) {
	// 参数验证
	if len(availableHosts) == 0 {
		return nil, fmt.Errorf("no available hosts")
	}

	// 随机选择一个主机
	randomIndex := rand.Intn(len(availableHosts))
	selectedHost := availableHosts[randomIndex]

	return selectedHost, nil
}

// GenerateInstanceID 根据服务名生成实例ID
func GenerateInstanceID(serviceName string) (string, error) {
	// 参数验证
	if serviceName == "" {
		return "", fmt.Errorf("serviceName cannot be empty")
	}

	// 生成时间戳（Unix时间戳）
	timestamp := time.Now().Unix()

	// 生成6位随机字符串
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	const randomLength = 6
	randomBytes := make([]byte, randomLength)
	for i := range randomBytes {
		randomBytes[i] = charset[rand.Intn(len(charset))]
	}
	randomString := string(randomBytes)

	// 组合生成实例ID：serviceName-timestamp-randomString
	instanceID := fmt.Sprintf("%s-%d-%s", serviceName, timestamp, randomString)

	return instanceID, nil
}

// GenerateInstanceIP 生成实例IP地址
func GenerateInstanceIP() (string, error) {
	// TODO: 实现实例IP生成逻辑
	return "", nil
}

// GenerateInstance 创建实例
func GenerateInstance(instanceID string, instanceIP string) error {
	// TODO: 实现实例创建逻辑
	return nil
}
