package service

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/qiniu/zeroops/internal/deploy/model"
	"gopkg.in/yaml.v3"
)

// PingResponse ping响应结构
type PingResponse struct {
	WantPkg    bool `json:"pkg"`
	WantConfig bool `json:"config"`
}

// RunRet 运行结果结构
type RunRet struct {
	Output string `json:"output"`
	Stderr string `json:"stderr"`
	Error  string `json:"err"`
}

// DeployService 发布服务接口，负责发布和回滚操作的执行
type DeployService interface {
	// DeployNewService 在指定主机上部署新服务
	DeployNewService(params *model.DeployNewServiceParams) (*model.OperationResult, error)

	// DeployNewVersion 触发指定服务版本的发布操作
	DeployNewVersion(params *model.DeployNewVersionParams) (*model.OperationResult, error)

	// ExecuteRollback 对指定实例执行回滚操作，支持单实例或批量实例回滚
	ExecuteRollback(params *model.RollbackParams) (*model.OperationResult, error)
}

// floyDeployService 使用floy实现发布和回滚操作
type floyDeployService struct {
	privateKey    string
	rsaPrivateKey *rsa.PrivateKey
	port          string
}

// NewDeployService 创建DeployService实例
func NewDeployService() DeployService {
	privateKeyPEM := loadPrivateKeyFromConfig()
	if privateKeyPEM == "" {
		panic("deploy service initialization failed: private key not found in config")
	}

	// 解析RSA私钥
	rsaPrivateKey, err := parseRSAPrivateKey(privateKeyPEM)
	if err != nil {
		// 快速失败，防止服务在不正确的状态下运行
		panic(fmt.Sprintf("deploy service initialization failed: invalid RSA private key: %v", err))
	}

	return &floyDeployService{
		privateKey:    privateKeyPEM,
		rsaPrivateKey: rsaPrivateKey,
		port:          "9092", // 默认floy端口
	}
}

// loadPrivateKeyFromConfig 从配置文件加载私钥
func loadPrivateKeyFromConfig() string {
	configPath := filepath.Join("internal", "deploy", "config.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}

	var config model.Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return ""
	}

	// 清理私钥字符串，移除多余的空白字符
	privateKey := strings.TrimSpace(config.PrivateKey)
	return privateKey
}

// parseRSAPrivateKey 解析RSA私钥
func parseRSAPrivateKey(privateKeyPEM string) (*rsa.PrivateKey, error) {
	// 添加PEM头尾（如果不存在）
	if !strings.Contains(privateKeyPEM, "-----BEGIN") {
		privateKeyPEM = "-----BEGIN RSA PRIVATE KEY-----\n" + privateKeyPEM + "\n-----END RSA PRIVATE KEY-----"
	}

	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %v", err)
	}

	return privateKey, nil
}

// DeployNewService 实现新服务部署操作
func (f *floyDeployService) DeployNewService(params *model.DeployNewServiceParams) (*model.OperationResult, error) {
	// 1. 参数验证
	if err := f.validateDeployNewServiceParams(params); err != nil {
		return nil, err
	}

	// 2. 验证包URL
	if err := ValidatePackageURL(params.PackageURL); err != nil {
		return nil, err
	}

	// 3. 下载包文件
	packageFilePath, _, err := f.downloadPackage(params.PackageURL)
	if err != nil {
		return nil, err
	}
	// 确保在函数结束时清理临时文件
	defer os.Remove(packageFilePath)

	// 4. 获取服务基础端口（从包文件中读取）
	basePort, err := f.getServiceBasePort(packageFilePath, params.Service)
	if err != nil {
		return nil, fmt.Errorf("获取服务基础端口失败: %v", err)
	}

	// 5. 获取可用的主机列表
	availableHosts, err := GetAvailableHostInfos()
	if err != nil {
		return nil, err
	}

	// 6. 创建指定数量的新服务实例
	successfulInstances := []string{}
	for i := 0; i < params.TotalNum; i++ {
		// 6.1 选择合适的主机
		selectedHost, err := SelectHostForNewInstance(availableHosts, params.Service, params.Version)
		if err != nil {
			// 记录错误但继续处理其他实例
			fmt.Printf("为实例 %d 选择主机失败: %v\n", i+1, err)
			continue
		}
		// 6.2 获取主机ip
		hostIP := selectedHost.HostIPAddress

		// 6.3 检查主机健康状态
		healthy, err := CheckHostHealth(hostIP)
		if err != nil {
			// 记录错误但继续处理其他实例
			fmt.Printf("检查主机 %s 健康状态失败: %v\n", hostIP, err)
			continue
		}
		if !healthy {
			// 记录错误但继续处理其他实例
			fmt.Printf("主机 %s 健康检查失败\n", hostIP)
			continue
		}
		// 6.4 创建新的instance
		instanceID, err := GenerateInstanceID(params.Service)
		if err != nil {
			// 记录错误但继续处理其他实例
			fmt.Printf("创建实例 %s 失败: %v\n", params.Service, err)
			continue
		}
		instanceIP := hostIP

		// 6.5 获取实例端口
		instancePort, err := f.getNextAvailablePort(params.Service, instanceIP, basePort)
		if err != nil {
			fmt.Printf("获取实例端口失败: %v\n", err)
			continue
		}

		// 6.6 处理包文件（修改配置文件中的端口）
		modifiedPackagePath, newMd5sum, err := f.processPackageWithPort(packageFilePath, params.Service, instancePort)
		if err != nil {
			fmt.Printf("处理包文件失败: %v\n", err)
			continue
		}
		defer os.Remove(modifiedPackagePath) // 清理修改后的包文件

		// 6.7 为当前实例计算包含包内容和配置的fversion
		instanceFversion := f.calculateFversion(params.Service, "prod", params.Version, modifiedPackagePath)

		// 6.8 部署服务到新创建的实例（使用新的MD5值）
		if err := f.deployToSingleInstance(instanceIP, params.Service, params.Version, instanceFversion, modifiedPackagePath, newMd5sum); err != nil {
			// 记录错误但继续处理其他实例
			fmt.Printf("部署到实例 %s (%s) 失败: %v\n", hostIP, hostIP, err)
			continue
		}

		// 6.8 记录实例信息到数据库（包含端口）
		fmt.Printf("实例 %s 部署成功，IP: %s, 端口: %d\n", instanceID, instanceIP, instancePort)

		// 6.9 将实例信息添加到数据库
		_, err = f.createInstanceRecord(instanceID, params.Service, params.Version, selectedHost.HostID, hostIP, instanceIP, instancePort)
		if err != nil {
			fmt.Printf("创建实例记录失败: %v\n", err)
			// 继续处理，不因为数据库错误而中断部署流程
		} else {
			// 创建版本历史记录
			if _, err := f.createVersionHistoryRecord(instanceID, params.Service, params.Version); err != nil {
				fmt.Printf("创建版本历史记录失败: %v\n", err)
				// 继续处理，不因为数据库错误而中断部署流程
			}
		}

		successfulInstances = append(successfulInstances, instanceID)
	}

	// 6. 构造返回结果
	result := &model.OperationResult{
		Service:        params.Service,
		Version:        params.Version,
		Instances:      successfulInstances,
		TotalInstances: len(successfulInstances),
	}

	return result, nil
}

// DeployNewVersion 实现发布操作
func (f *floyDeployService) DeployNewVersion(params *model.DeployNewVersionParams) (*model.OperationResult, error) {
	// 1. 参数验证
	if err := f.validateDeployNewVersionParams(params); err != nil {
		return nil, err
	}

	// 2. 验证包URL
	if err := ValidatePackageURL(params.PackageURL); err != nil {
		return nil, err
	}

	// 3. 下载包文件
	packageFilePath, _, err := f.downloadPackage(params.PackageURL)
	if err != nil {
		return nil, fmt.Errorf("下载包文件失败: %v", err)
	}
	// 确保在函数结束时清理临时文件
	defer os.Remove(packageFilePath)

	// 4. 串行部署到各个实例
	successInstances := []string{}
	for _, instanceID := range params.Instances {

		// 4.1 获取实例IP和端口
		instanceIP, err := GetInstanceIP(instanceID)
		if err != nil {
			fmt.Printf("获取实例 %s IP失败: %v\n", instanceID, err)
			continue
		}

		instancePort, err := GetInstancePort(instanceID)
		if err != nil {
			fmt.Printf("获取实例 %s 端口失败: %v\n", instanceID, err)
			continue
		}

		// 4.2 检查实例健康状态
		healthy, err := CheckInstanceHealth(instanceIP, instancePort)
		if err != nil {
			// 记录错误但继续处理其他实例
			fmt.Printf("实例 %s 健康检查失败: %v\n", instanceID, err)
			continue
		}
		if !healthy {
			// 记录错误但继续处理其他实例
			fmt.Printf("实例 %s (IP: %s, Port: %d) 健康检查失败\n", instanceID, instanceIP, instancePort)
			continue
		}

		// 4.3 处理包文件（修改配置文件中的端口）
		modifiedPackagePath, newMd5sum, err := f.processPackageWithPort(packageFilePath, params.Service, instancePort)
		if err != nil {
			fmt.Printf("处理包文件失败: %v\n", err)
			continue
		}
		defer os.Remove(modifiedPackagePath) // 清理修改后的包文件

		// 4.4 为当前实例计算fversion
		instanceFversion := f.calculateFversion(params.Service, "prod", params.Version, modifiedPackagePath)

		// 4.5 部署到单个实例（使用修改后的包文件）
		if err := f.deployToSingleInstance(instanceIP, params.Service, params.Version, instanceFversion, modifiedPackagePath, newMd5sum); err != nil {
			// 记录错误但继续处理其他实例
			fmt.Printf("部署到实例 %s (%s) 失败: %v\n", instanceID, instanceIP, err)
			continue
		}

		// 4.6 更新实例版本信息到数据库
		if err := f.updateInstanceVersion(instanceID, params.Service, params.Version); err != nil {
			fmt.Printf("更新实例 %s 版本信息失败: %v\n", instanceID, err)
			// 继续处理，不因为数据库错误而中断部署流程
		} else {
			// 5.6 处理版本历史记录（DeployNewVersion逻辑）
			// 获取当前活跃版本并标记为deprecated
			if currentVersion, err := f.getCurrentActiveVersion(instanceID, params.Service); err == nil {
				if err := f.updateVersionStatus(instanceID, params.Service, currentVersion, "deprecated"); err != nil {
					fmt.Printf("更新实例 %s 当前版本状态失败: %v\n", instanceID, err)
				}
			}

			// 创建新版本的版本历史记录
			if err := f.createInstanceVersionHistory(instanceID, params.Service, params.Version, "active"); err != nil {
				fmt.Printf("创建实例 %s 版本历史记录失败: %v\n", instanceID, err)
			}
		}

		successInstances = append(successInstances, instanceID)
	}

	// 6. 构造返回结果
	result := &model.OperationResult{
		Service:        params.Service,
		Version:        params.Version,
		Instances:      successInstances,
		TotalInstances: len(successInstances),
	}

	return result, nil
}

// ExecuteRollback 实现回滚操作
func (f *floyDeployService) ExecuteRollback(params *model.RollbackParams) (*model.OperationResult, error) {
	// 1. 参数验证
	if err := f.validateRollbackParams(params); err != nil {
		return nil, err
	}

	// 2. 验证回滚包URL
	if err := ValidatePackageURL(params.PackageURL); err != nil {
		return nil, err
	}

	// 3. 下载回滚包文件
	packageFilePath, _, err := f.downloadPackage(params.PackageURL)
	if err != nil {
		return nil, fmt.Errorf("下载回滚包文件失败: %v", err)
	}
	// 确保在函数结束时清理临时文件
	defer os.Remove(packageFilePath)

	// 4. 串行回滚到各个实例（单实例容错）
	successInstances := []string{}
	for _, instanceID := range params.Instances {

		// 4.1 获取实例IP和端口
		instanceIP, err := GetInstanceIP(instanceID)
		if err != nil {
			fmt.Printf("获取实例 %s IP失败: %v\n", instanceID, err)
			continue
		}

		instancePort, err := GetInstancePort(instanceID)
		if err != nil {
			fmt.Printf("获取实例 %s 端口失败: %v\n", instanceID, err)
			continue
		}

		// 4.2 检查实例健康状态
		healthy, err := CheckInstanceHealth(instanceIP, instancePort)
		if err != nil {
			// 记录错误但继续处理其他实例
			fmt.Printf("实例 %s 健康检查失败: %v\n", instanceID, err)
			continue
		}
		if !healthy {
			// 记录错误但继续处理其他实例
			fmt.Printf("实例 %s (IP: %s, Port: %d) 健康检查失败\n", instanceID, instanceIP, instancePort)
			continue
		}

		// 4.3 处理包文件（修改配置文件中的端口）
		modifiedPackagePath, newMd5sum, err := f.processPackageWithPort(packageFilePath, params.Service, instancePort)
		if err != nil {
			fmt.Printf("处理包文件失败: %v\n", err)
			continue
		}
		defer os.Remove(modifiedPackagePath) // 清理修改后的包文件

		// 4.4 为当前实例计算fversion
		instanceFversion := f.calculateFversion(params.Service, "prod", params.TargetVersion, modifiedPackagePath)

		// 4.5 回滚到单个实例（使用修改后的包文件）
		if err := f.rollbackToSingleInstance(instanceIP, params.Service, params.TargetVersion, instanceFversion, modifiedPackagePath, newMd5sum); err != nil {
			// 记录错误但继续处理其他实例
			fmt.Printf("回滚到实例 %s (%s) 失败: %v\n", instanceID, instanceIP, err)
			continue
		}

		// 4.6 更新实例版本信息到数据库
		if err := f.updateInstanceVersion(instanceID, params.Service, params.TargetVersion); err != nil {
			fmt.Printf("更新实例 %s 版本信息失败: %v\n", instanceID, err)
			// 继续处理，不因为数据库错误而中断部署流程
		} else {
			// 5.6 处理版本历史记录（ExecuteRollback逻辑）
			// 获取当前活跃版本并标记为rollback
			if currentVersion, err := f.getCurrentActiveVersion(instanceID, params.Service); err == nil {
				if err := f.updateVersionStatus(instanceID, params.Service, currentVersion, "rollback"); err != nil {
					fmt.Printf("更新实例 %s 当前版本状态失败: %v\n", instanceID, err)
				}
			}

			// 将目标版本状态改为active
			if err := f.updateVersionStatus(instanceID, params.Service, params.TargetVersion, "active"); err != nil {
				fmt.Printf("更新实例 %s 目标版本状态失败: %v\n", instanceID, err)
			}
		}

		successInstances = append(successInstances, instanceID)
	}

	// 6. 构造返回结果
	result := &model.OperationResult{
		Service:        params.Service,
		Version:        params.TargetVersion,
		Instances:      successInstances,
		TotalInstances: len(successInstances),
	}

	return result, nil
}

// validateDeployNewServiceParams 验证新服务部署参数
func (f *floyDeployService) validateDeployNewServiceParams(params *model.DeployNewServiceParams) error {
	if params == nil {
		return fmt.Errorf("部署参数不能为空")
	}
	if params.Service == "" {
		return fmt.Errorf("服务名称不能为空")
	}
	if params.Version == "" {
		return fmt.Errorf("版本号不能为空")
	}
	if params.TotalNum <= 0 {
		return fmt.Errorf("新建实例数量必须大于0")
	}
	if params.PackageURL == "" {
		return fmt.Errorf("包URL不能为空")
	}
	return nil
}

// validateDeployNewVersionParams 验证发布参数
func (f *floyDeployService) validateDeployNewVersionParams(params *model.DeployNewVersionParams) error {
	if params == nil {
		return fmt.Errorf("发布参数不能为空")
	}
	if params.Service == "" {
		return fmt.Errorf("服务名称不能为空")
	}
	if params.Version == "" {
		return fmt.Errorf("版本号不能为空")
	}
	if len(params.Instances) == 0 {
		return fmt.Errorf("实例列表不能为空")
	}
	if params.PackageURL == "" {
		return fmt.Errorf("包URL不能为空")
	}
	return nil
}

// downloadPackage 下载包文件到临时文件，支持HTTP URL和本地路径
func (f *floyDeployService) downloadPackage(packageURL string) (string, []byte, error) {
	// 检测是否为HTTP URL
	if strings.HasPrefix(packageURL, "http://") || strings.HasPrefix(packageURL, "https://") {
		return f.downloadFromHTTP(packageURL)
	} else {
		return f.copyFromLocal(packageURL)
	}
}

// downloadFromHTTP 从HTTP URL下载包文件
func (f *floyDeployService) downloadFromHTTP(packageURL string) (string, []byte, error) {
	// 创建临时文件
	tmpFile, err := os.CreateTemp("", "deploy-package-*.tmp")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp file: %v", err)
	}
	tmpFilePath := tmpFile.Name()

	// 下载包文件（使用带超时的客户端）
	client := &http.Client{Timeout: 300 * time.Second} // 与 pushPackage 超时保持一致
	resp, err := client.Get(packageURL)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFilePath)
		return "", nil, fmt.Errorf("failed to download package: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		tmpFile.Close()
		os.Remove(tmpFilePath)
		return "", nil, fmt.Errorf("failed to download package: status %d", resp.StatusCode)
	}

	// 流式写入临时文件并计算MD5
	h := md5.New()
	multiWriter := io.MultiWriter(tmpFile, h)

	_, err = io.Copy(multiWriter, resp.Body)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFilePath)
		return "", nil, fmt.Errorf("failed to write package data: %v", err)
	}

	// 关闭文件
	err = tmpFile.Close()
	if err != nil {
		os.Remove(tmpFilePath)
		return "", nil, fmt.Errorf("failed to close temp file: %v", err)
	}

	md5sum := h.Sum(nil)
	return tmpFilePath, md5sum, nil
}

// copyFromLocal 从本地路径复制包文件
func (f *floyDeployService) copyFromLocal(filePath string) (string, []byte, error) {
	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", nil, fmt.Errorf("local file not found: %s", filePath)
	}

	// 创建临时文件
	tmpFile, err := os.CreateTemp("", "deploy-package-*.tmp")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp file: %v", err)
	}
	tmpFilePath := tmpFile.Name()

	// 打开源文件
	srcFile, err := os.Open(filePath)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFilePath)
		return "", nil, fmt.Errorf("failed to open local file: %v", err)
	}
	defer srcFile.Close()

	// 复制文件并计算MD5
	h := md5.New()
	multiWriter := io.MultiWriter(tmpFile, h)

	_, err = io.Copy(multiWriter, srcFile)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFilePath)
		return "", nil, fmt.Errorf("failed to copy file data: %v", err)
	}

	// 关闭文件
	err = tmpFile.Close()
	if err != nil {
		os.Remove(tmpFilePath)
		return "", nil, fmt.Errorf("failed to close temp file: %v", err)
	}

	md5sum := h.Sum(nil)
	return tmpFilePath, md5sum, nil
}

// calculateFversion 计算版本号，包含包内容和配置文件内容
func (f *floyDeployService) calculateFversion(service, env, version string, packageFilePath string) string {
	h := md5.New()
	io.WriteString(h, fmt.Sprintf("%s:%s:%s", service, env, version))

	// 添加包文件内容（二进制tar文件）
	packageContent, err := os.ReadFile(packageFilePath)
	if err == nil {
		io.WriteString(h, ":")
		h.Write(packageContent)
	}

	// 添加配置文件内容
	io.WriteString(h, ":app.conf")
	io.WriteString(h, "\n\n")
	io.WriteString(h, "# Configuration placeholder for fversion calculation")

	fversion := base64.URLEncoding.EncodeToString(h.Sum(nil))
	fversion = strings.TrimRight(fversion, "=")
	fversion = strings.TrimLeft(fversion, "-_")

	return fversion
}

// deployToSingleInstance 部署到单个实例
func (f *floyDeployService) deployToSingleInstance(instanceIP, service, version, fversion string, packageFilePath string, md5sum []byte) error {
	// 1. Ping检查
	wantPkg, wantConfig, err := f.ping(instanceIP, service, fversion, version, "Auto deploy")
	if err != nil {
		return fmt.Errorf("ping检查失败: %v", err)
	}

	// 2. 推送包文件
	if wantPkg {
		if err := f.pushPackage(instanceIP, service, fversion, version, packageFilePath, md5sum); err != nil {
			return fmt.Errorf("推送包文件失败: %v", err)
		}
	}

	// 3. 推送配置文件
	if wantConfig {
		if err := f.pushConfig(instanceIP, service, fversion); err != nil {
			return fmt.Errorf("推送配置文件失败: %v", err)
		}
	}

	// 4. 切换版本（激活新版本）
	if err := f.switchVersion(instanceIP, service, fversion, false); err != nil {
		return fmt.Errorf("版本切换失败: %v", err)
	}

	// 5. 运行服务
	if err := f.runService(instanceIP, service, "start.sh", "/home/qboxserver", 300); err != nil {
		return fmt.Errorf("运行服务失败: %v", err)
	}

	return nil
}

// rollbackToSingleInstance 回滚到单个实例
func (f *floyDeployService) rollbackToSingleInstance(instanceIP, service, targetVersion, fversion string, packageFilePath string, md5sum []byte) error {
	// 1. Ping检查
	wantPkg, wantConfig, err := f.ping(instanceIP, service, fversion, targetVersion, "Auto rollback")
	if err != nil {
		return fmt.Errorf("ping检查失败: %v", err)
	}

	// 2. 推送回滚包文件
	if wantPkg {
		if err := f.pushPackage(instanceIP, service, fversion, targetVersion, packageFilePath, md5sum); err != nil {
			return fmt.Errorf("推送回滚包文件失败: %v", err)
		}
	}

	// 3. 推送配置文件
	if wantConfig {
		if err := f.pushConfig(instanceIP, service, fversion); err != nil {
			return fmt.Errorf("推送配置文件失败: %v", err)
		}
	}

	// 4. 切换版本（激活回滚版本）
	if err := f.switchVersion(instanceIP, service, fversion, false); err != nil {
		return fmt.Errorf("版本切换失败: %v", err)
	}

	// 5. 运行服务
	if err := f.runService(instanceIP, service, "start.sh", "/home/qboxserver", 300); err != nil {
		return fmt.Errorf("运行服务失败: %v", err)
	}

	return nil
}

// signRequest 为HTTP请求添加RSA签名
func (f *floyDeployService) signRequest(req *http.Request) error {
	// 读取请求体
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return fmt.Errorf("failed to read request body: %v", err)
		}
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	// 生成时间戳
	timestamp := strconv.FormatInt(time.Now().UnixNano(), 10)

	// 计算签名内容：请求体 + 时间戳 + URI（与 floy 客户端保持一致）
	sh := crypto.SHA1.New()
	if len(bodyBytes) > 0 {
		sh.Write(bodyBytes)
	}
	sh.Write([]byte(timestamp))
	sh.Write([]byte(req.URL.RequestURI()))
	hash := sh.Sum(nil)

	// RSA签名
	signature, err := rsa.SignPKCS1v15(rand.Reader, f.rsaPrivateKey, crypto.SHA1, hash)
	if err != nil {
		return fmt.Errorf("failed to sign request: %v", err)
	}

	// 设置请求头
	req.Header.Set("TimeStamp", timestamp)
	req.Header.Set("Authorization", hex.EncodeToString(signature))

	return nil
}

// ping 检查floyd服务状态
func (f *floyDeployService) ping(instanceIP, service, fversion, version, message string) (bool, bool, error) {
	// 构造请求URL
	baseURL := fmt.Sprintf("http://%s:%s", instanceIP, f.port)

	// 构造请求参数
	params := url.Values{}
	params.Add("service", service)
	params.Add("fversion", fversion)
	params.Add("pkgOwner", "qboxserver")
	params.Add("installDir", "")
	params.Add("pkg", version)
	params.Add("message", base64.URLEncoding.EncodeToString([]byte(message)))

	// 创建请求
	req, err := http.NewRequest("POST", baseURL+"/ping", strings.NewReader(params.Encode()))
	if err != nil {
		return false, false, fmt.Errorf("failed to create ping request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// 签名请求
	if err := f.signRequest(req); err != nil {
		return false, false, fmt.Errorf("failed to sign ping request: %v", err)
	}

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, false, fmt.Errorf("failed to send ping request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 202 {
		// Nothing to do
		return false, false, nil
	}

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return false, false, fmt.Errorf("ping failed: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// 解析JSON响应
	var pingResp PingResponse
	if err := json.NewDecoder(resp.Body).Decode(&pingResp); err != nil {
		return false, false, fmt.Errorf("failed to decode ping response: %v", err)
	}

	return pingResp.WantPkg, pingResp.WantConfig, nil
}

// pushPackage 推送包文件
func (f *floyDeployService) pushPackage(instanceIP, service, fversion, version string, packageFilePath string, md5sum []byte) error {
	baseURL := fmt.Sprintf("http://%s:%s", instanceIP, f.port)

	// 使用 multipart.Writer 构造请求体
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// 写入表单字段
	writer.WriteField("service", service)
	writer.WriteField("fversion", fversion)
	writer.WriteField("pkgOwner", "qboxserver")
	writer.WriteField("installDir", "")

	// 打开包文件
	packageFile, err := os.Open(packageFilePath)
	if err != nil {
		return fmt.Errorf("failed to open package file: %v", err)
	}
	defer packageFile.Close()

	// 创建文件字段，设置 Content-Md5 头
	header := make(map[string][]string)
	header["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name="file"; filename="%s"`, version)}
	header["Content-Type"] = []string{"application/octet-stream"}
	header["Content-Md5"] = []string{base64.URLEncoding.EncodeToString(md5sum)}

	fileWriter, err := writer.CreatePart(header)
	if err != nil {
		return fmt.Errorf("failed to create form file: %v", err)
	}

	// 流式复制文件数据
	_, err = io.Copy(fileWriter, packageFile)
	if err != nil {
		return fmt.Errorf("failed to copy file data: %v", err)
	}

	// 关闭 writer 以完成 multipart 格式
	err = writer.Close()
	if err != nil {
		return fmt.Errorf("failed to close multipart writer: %v", err)
	}

	// 创建请求
	req, err := http.NewRequest("POST", baseURL+"/pushPkg", &buf)
	if err != nil {
		return fmt.Errorf("failed to create pushPkg request: %v", err)
	}

	// 设置 Content-Type，包含自动生成的边界
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// 签名请求
	if err := f.signRequest(req); err != nil {
		return fmt.Errorf("failed to sign pushPkg request: %v", err)
	}

	// 发送请求
	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send pushPkg request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pushPkg failed: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// pushConfig 推送配置文件
func (f *floyDeployService) pushConfig(instanceIP, service, fversion string) error {
	baseURL := fmt.Sprintf("http://%s:%s", instanceIP, f.port)

	// 简单的配置文件示例
	configContent := fmt.Sprintf("# Configuration for %s\nservice.name=%s\nservice.version=%s\n",
		service, service, fversion)
	configMD5 := md5.Sum([]byte(configContent))

	// 使用 multipart.Writer 构造请求体
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// 写入表单字段
	writer.WriteField("service", service)
	writer.WriteField("fversion", fversion)
	writer.WriteField("pkgOwner", "qboxserver")
	writer.WriteField("installDir", "")

	// 创建文件字段，设置 Content-Md5 头
	header := make(map[string][]string)
	header["Content-Disposition"] = []string{`form-data; name="file"; filename="app.conf"`}
	header["Content-Type"] = []string{"application/octet-stream"}
	header["Content-Md5"] = []string{base64.URLEncoding.EncodeToString(configMD5[:])}
	header["File-Mode"] = []string{"644"}

	fileWriter, err := writer.CreatePart(header)
	if err != nil {
		return fmt.Errorf("failed to create form file: %v", err)
	}

	// 写入配置文件内容
	_, err = fileWriter.Write([]byte(configContent))
	if err != nil {
		return fmt.Errorf("failed to write config data: %v", err)
	}

	// 关闭 writer 以完成 multipart 格式
	err = writer.Close()
	if err != nil {
		return fmt.Errorf("failed to close multipart writer: %v", err)
	}

	// 创建请求
	req, err := http.NewRequest("POST", baseURL+"/pushConfig", &buf)
	if err != nil {
		return fmt.Errorf("failed to create pushConfig request: %v", err)
	}

	// 设置 Content-Type，包含自动生成的边界
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// 签名请求
	if err := f.signRequest(req); err != nil {
		return fmt.Errorf("failed to sign pushConfig request: %v", err)
	}

	// 发送请求
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send pushConfig request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pushConfig failed: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// switchVersion 切换版本
func (f *floyDeployService) switchVersion(instanceIP, service, fversion string, force bool) error {
	baseURL := fmt.Sprintf("http://%s:%s", instanceIP, f.port)

	// 构造请求参数
	params := url.Values{}
	params.Add("service", service)
	params.Add("fversion", fversion)
	params.Add("pkgOwner", "qboxserver")
	params.Add("installDir", "")
	if force {
		params.Add("force", "1")
	}

	// 创建请求
	req, err := http.NewRequest("POST", baseURL+"/switch", strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create switch request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// 签名请求
	if err := f.signRequest(req); err != nil {
		return fmt.Errorf("failed to sign switch request: %v", err)
	}

	// 发送请求
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send switch request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 202 {
		// Nothing to do
		return nil
	}

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("switch failed: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// runService 运行服务
func (f *floyDeployService) runService(instanceIP, service, bashfile, installDir string, timeout int) error {
	baseURL := fmt.Sprintf("http://%s:%s", instanceIP, f.port)

	// 构造请求参数
	params := url.Values{}
	params.Add("service", service)
	params.Add("bashfile", bashfile)
	params.Add("installDir", installDir)
	params.Add("timeout", fmt.Sprintf("%d", timeout))

	// 创建请求
	req, err := http.NewRequest("POST", baseURL+"/run", strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create run request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// 签名请求
	if err := f.signRequest(req); err != nil {
		return fmt.Errorf("failed to sign run request: %v", err)
	}

	// 发送请求
	client := &http.Client{Timeout: time.Duration(timeout+60) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send run request: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read run response: %v", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("run failed: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// 解析响应结果
	var result RunRet
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return fmt.Errorf("failed to parse run response: %v", err)
	}

	// 检查运行结果
	if result.Error != "" {
		return fmt.Errorf("service run error: %s, stderr: %s", result.Error, result.Stderr)
	}

	// 记录运行输出（可选）
	if result.Output != "" {
		fmt.Printf("Service run output: %s\n", result.Output)
	}

	return nil
}

// createInstanceRecord 创建实例记录到数据库
func (f *floyDeployService) createInstanceRecord(instanceID, serviceName, serviceVersion, hostID, hostIP, instanceIP string, port int) (*model.Instance, error) {
	// 初始化数据库连接
	_, err := initDatabase()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database connection: %w", err)
	}

	// 创建实例记录
	instance := &model.Instance{
		ID:             instanceID, // 使用传入的实例ID
		ServiceName:    serviceName,
		ServiceVersion: serviceVersion,
		HostID:         hostID,
		HostIPAddress:  hostIP,
		IPAddress:      instanceIP, // ip_address和host_ip_address相同
		Port:           port,
		Status:         "active", // 默认状态为active
		IsStopped:      false,    // 默认未停止
	}

	// 保存实例到数据库
	if err := instanceRepo.CreateInstance(instance); err != nil {
		return nil, fmt.Errorf("failed to create instance record: %w", err)
	}

	fmt.Printf("成功创建实例记录，实例ID: %s\n", instance.ID)
	return instance, nil
}

// createVersionHistoryRecord 创建版本历史记录到数据库
func (f *floyDeployService) createVersionHistoryRecord(instanceID, serviceName, serviceVersion string) (*model.VersionHistory, error) {
	// 初始化数据库连接
	_, err := initDatabase()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database connection: %w", err)
	}

	// 创建版本历史记录
	versionHistory := &model.VersionHistory{
		InstanceID:     instanceID,
		ServiceName:    serviceName,
		ServiceVersion: serviceVersion,
		Status:         "active", // 默认状态为active
	}

	// 保存版本历史到数据库
	if err := instanceRepo.CreateVersionHistory(versionHistory); err != nil {
		return nil, fmt.Errorf("failed to create version history record: %w", err)
	}

	fmt.Printf("成功创建版本历史记录，版本历史ID: %d\n", versionHistory.ID)
	return versionHistory, nil
}

// updateInstanceVersion 更新实例版本信息（通用方法）
func (f *floyDeployService) updateInstanceVersion(instanceID, serviceName, serviceVersion string) error {
	// 初始化数据库连接
	_, err := initDatabase()
	if err != nil {
		return fmt.Errorf("failed to initialize database connection: %w", err)
	}

	// 更新instances表中的版本信息
	if err := instanceRepo.UpdateInstanceVersion(instanceID, serviceName, serviceVersion); err != nil {
		return fmt.Errorf("failed to update instance version: %w", err)
	}

	fmt.Printf("成功更新实例 %s 版本信息到数据库，新版本: %s\n", instanceID, serviceVersion)
	return nil
}

// createInstanceVersionHistory 创建实例版本历史记录
func (f *floyDeployService) createInstanceVersionHistory(instanceID, serviceName, serviceVersion, status string) error {
	// 初始化数据库连接
	_, err := initDatabase()
	if err != nil {
		return fmt.Errorf("failed to initialize database connection: %w", err)
	}

	// 创建版本历史记录
	if err := instanceRepo.CreateInstanceVersionHistory(instanceID, serviceName, serviceVersion, status); err != nil {
		return fmt.Errorf("failed to create version history: %w", err)
	}

	fmt.Printf("成功创建实例 %s 版本历史记录，版本: %s，状态: %s\n", instanceID, serviceVersion, status)
	return nil
}

// getCurrentActiveVersion 获取当前活跃版本
func (f *floyDeployService) getCurrentActiveVersion(instanceID, serviceName string) (string, error) {
	// 初始化数据库连接
	_, err := initDatabase()
	if err != nil {
		return "", fmt.Errorf("failed to initialize database connection: %w", err)
	}

	return instanceRepo.GetCurrentActiveVersion(instanceID, serviceName)
}

// updateVersionStatus 更新版本状态
func (f *floyDeployService) updateVersionStatus(instanceID, serviceName, serviceVersion, newStatus string) error {
	// 初始化数据库连接
	_, err := initDatabase()
	if err != nil {
		return fmt.Errorf("failed to initialize database connection: %w", err)
	}

	return instanceRepo.UpdateVersionStatus(instanceID, serviceName, serviceVersion, newStatus)
}

// validateRollbackParams 验证回滚参数
func (f *floyDeployService) validateRollbackParams(params *model.RollbackParams) error {
	if params == nil {
		return fmt.Errorf("回滚参数不能为空")
	}
	if params.Service == "" {
		return fmt.Errorf("服务名称不能为空")
	}
	if params.TargetVersion == "" {
		return fmt.Errorf("目标版本号不能为空")
	}
	if len(params.Instances) == 0 {
		return fmt.Errorf("实例列表不能为空")
	}
	if params.PackageURL == "" {
		return fmt.Errorf("包URL不能为空")
	}
	return nil
}

// getServiceBasePort 从包文件中获取服务基础端口
func (f *floyDeployService) getServiceBasePort(packagePath, serviceName string) (int, error) {
	// 1. 创建临时目录
	tempDir, err := os.MkdirTemp("", "read-config-*")
	if err != nil {
		return 0, fmt.Errorf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 2. 解压tar.gz到临时目录
	err = f.extractTarGz(packagePath, tempDir)
	if err != nil {
		return 0, fmt.Errorf("解压包文件失败: %v", err)
	}

	// 3. 读取配置文件
	configPath := filepath.Join(tempDir, "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return 0, fmt.Errorf("读取配置文件失败: %v", err)
	}

	// 4. 解析YAML获取端口
	var config map[string]interface{}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return 0, fmt.Errorf("解析配置文件失败: %v", err)
	}

	// 5. 获取service.port字段
	if service, ok := config["service"].(map[string]interface{}); ok {
		if port, ok := service["port"].(int); ok {
			return port, nil
		}
		return 0, fmt.Errorf("配置文件中未找到有效的端口号")
	}

	return 0, fmt.Errorf("配置文件格式错误，未找到service字段")
}

// getExistingInstancePorts 查询指定主机上已存在的实例端口
func (f *floyDeployService) getExistingInstancePorts(serviceName, instanceIP string) ([]int, error) {
	// 初始化数据库连接
	_, err := initDatabase()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database connection: %w", err)
	}

	// 查询已存在的实例端口
	ports, err := instanceRepo.GetExistingInstancePorts(serviceName, instanceIP)
	if err != nil {
		return nil, fmt.Errorf("failed to query existing instance ports: %w", err)
	}

	return ports, nil
}

// getNextAvailablePort 获取下一个可用端口
func (f *floyDeployService) getNextAvailablePort(serviceName, instanceIP string, basePort int) (int, error) {
	// 查询已存在的端口
	existingPorts, err := f.getExistingInstancePorts(serviceName, instanceIP)
	if err != nil {
		return 0, fmt.Errorf("查询已存在实例端口失败: %v", err)
	}

	// 找到下一个可用端口
	port := basePort
	for {
		if !f.isPortInUse(port, existingPorts) {
			return port, nil
		}
		port++

		// 防止无限循环，设置最大端口限制
		if port > basePort+1000 {
			return 0, fmt.Errorf("无法找到可用端口，已超过最大限制")
		}
	}
}

// isPortInUse 检查端口是否被占用
func (f *floyDeployService) isPortInUse(port int, existingPorts []int) bool {
	for _, existingPort := range existingPorts {
		if port == existingPort {
			return true
		}
	}
	return false
}

// processPackageWithPort 处理包文件，修改配置文件中的端口
func (f *floyDeployService) processPackageWithPort(packagePath, serviceName string, port int) (string, []byte, error) {
	// 1. 创建临时目录
	tempDir, err := os.MkdirTemp("", "deploy-*")
	if err != nil {
		return "", nil, fmt.Errorf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 2. 解压tar.gz到临时目录
	err = f.extractTarGz(packagePath, tempDir)
	if err != nil {
		return "", nil, fmt.Errorf("解压包文件失败: %v", err)
	}

	// 3. 修改配置文件中的端口
	err = f.modifyConfigPort(tempDir, serviceName, port)
	if err != nil {
		return "", nil, fmt.Errorf("修改配置文件失败: %v", err)
	}

	// 3.1 修改start.sh脚本中的端口逻辑
	err = f.modifyStartScript(tempDir, port)
	if err != nil {
		return "", nil, fmt.Errorf("修改启动脚本失败: %v", err)
	}

	// 4. 重新打包到临时目录
	tempPackagePath := filepath.Join(tempDir, "modified-"+filepath.Base(packagePath))
	err = f.createTarGz(tempDir, tempPackagePath)
	if err != nil {
		return "", nil, fmt.Errorf("重新打包失败: %v", err)
	}

	// 5. 计算修改后包文件的MD5值
	newMd5sum, err := f.calculateFileMD5(tempPackagePath)
	if err != nil {
		return "", nil, fmt.Errorf("计算MD5失败: %v", err)
	}

	// 6. 将修改后的包文件移动到系统临时目录，避免被清理
	finalPackagePath := filepath.Join(os.TempDir(), "modified-"+filepath.Base(packagePath))
	err = os.Rename(tempPackagePath, finalPackagePath)
	if err != nil {
		return "", nil, fmt.Errorf("移动修改后的包文件失败: %v", err)
	}

	return finalPackagePath, newMd5sum, nil
}

// calculateFileMD5 计算文件的MD5值
func (f *floyDeployService) calculateFileMD5(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	hash := md5.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return nil, err
	}

	return hash.Sum(nil), nil
}

// extractTarGz 解压tar.gz文件
func (f *floyDeployService) extractTarGz(src, dest string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dest, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(targetPath, os.FileMode(header.Mode))
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(targetPath), 0755)
			outFile, err := os.Create(targetPath)
			if err != nil {
				return err
			}
			_, err = io.Copy(outFile, tarReader)
			outFile.Close()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// modifyConfigPort 修改配置文件中的端口
func (f *floyDeployService) modifyConfigPort(tempDir, serviceName string, port int) error {
	configPath := filepath.Join(tempDir, "config.yaml")

	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %v", err)
	}

	// 解析YAML
	var config map[string]interface{}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return fmt.Errorf("解析配置文件失败: %v", err)
	}

	// 修改service.port字段
	if service, ok := config["service"].(map[string]interface{}); ok {
		service["port"] = port
		fmt.Printf("修改服务 %s 端口为: %d\n", serviceName, port)
	} else {
		return fmt.Errorf("配置文件格式错误，未找到service字段")
	}

	// 写回配置文件
	newData, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化配置文件失败: %v", err)
	}

	err = os.WriteFile(configPath, newData, 0644)
	if err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
	}

	return nil
}

// modifyStartScript 修改启动脚本中的端口
func (f *floyDeployService) modifyStartScript(tempDir string, port int) error {
	startScriptPath := filepath.Join(tempDir, "start.sh")

	// 检查start.sh文件是否存在
	if _, err := os.Stat(startScriptPath); os.IsNotExist(err) {
		// 如果start.sh不存在，说明这个包可能没有start.sh脚本，跳过
		return nil
	}

	// 读取start.sh文件内容
	content, err := os.ReadFile(startScriptPath)
	if err != nil {
		return fmt.Errorf("读取启动脚本失败: %v", err)
	}
	contentStr := string(content)

	// 检查是否已经是新版本的脚本（从配置文件读取端口）
	if strings.Contains(contentStr, "从配置文件读取端口") {
		// 如果已经是新版本脚本，只需要更新端口号
		oldPattern := `SERVICE_PORT="8080"`
		newPattern := fmt.Sprintf(`SERVICE_PORT="%d"`, port)
		newContent := strings.Replace(contentStr, oldPattern, newPattern, -1)

		err = os.WriteFile(startScriptPath, []byte(newContent), 0755)
		if err != nil {
			return fmt.Errorf("写入启动脚本失败: %v", err)
		}

		fmt.Printf("更新启动脚本端口，端口: %d\n", port)
		return nil
	}

	// 替换环境变量设置，改为从配置文件读取端口
	oldPattern := `export SERVICE_PORT="${SERVICE_PORT:-8080}"`
	newPattern := fmt.Sprintf(`# 从配置文件读取端口
if [ -f "config.yaml" ]; then
    SERVICE_PORT=$(grep -E "^\s*port:\s*" config.yaml | sed 's/.*port:\s*\([0-9]*\).*/\1/')
    if [ -z "$SERVICE_PORT" ]; then
        SERVICE_PORT="%d"
    fi
else
    SERVICE_PORT="%d"
fi`, port, port)

	// 执行替换
	newContent := strings.Replace(contentStr, oldPattern, newPattern, 1)

	// 写回文件
	err = os.WriteFile(startScriptPath, []byte(newContent), 0755)
	if err != nil {
		return fmt.Errorf("写入启动脚本失败: %v", err)
	}

	fmt.Printf("修改启动脚本端口逻辑，端口: %d\n", port)
	return nil
}

// createTarGz 创建tar.gz文件
func (f *floyDeployService) createTarGz(src, dest string) error {
	file, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer file.Close()

	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if path == src {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		header.Name = relPath

		err = tarWriter.WriteHeader(header)
		if err != nil {
			return err
		}

		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(tarWriter, file)
			if err != nil {
				return err
			}
		}

		return nil
	})
}
