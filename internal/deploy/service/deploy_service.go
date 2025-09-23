package service

import (
	"bytes"
	"crypto"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
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
		port:          "9902", // 默认floy端口
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
	packageFilePath, md5sum, err := f.downloadPackage(params.PackageURL)
	if err != nil {
		return nil, err
	}
	// 确保在函数结束时清理临时文件
	defer os.Remove(packageFilePath)

	// 4. 计算fversion（暂时不使用，但保留以备将来实现）
	fversion := f.calculateFversion(params.Service, "prod", params.Version)

	// 5. 获取可用的主机列表
	availableHosts, err := GetAvailableHosts()
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
		hostIP, err := GetHostIp(selectedHost)
		if err != nil {
			// 记录错误但继续处理其他实例
			fmt.Printf("获取主机 %s 的IP失败: %v\n", selectedHost, err)
			continue
		}
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
		// 6.5 部署服务到新创建的实例
		if err := f.deployToSingleInstance(instanceIP, params.Service, params.Version, fversion, packageFilePath, md5sum); err != nil {
			// 记录错误但继续处理其他实例
			fmt.Printf("部署到实例 %s (%s) 失败: %v\n", hostIP, hostIP, err)
			continue
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
	packageFilePath, md5sum, err := f.downloadPackage(params.PackageURL)
	if err != nil {
		return nil, fmt.Errorf("下载包文件失败: %v", err)
	}
	// 确保在函数结束时清理临时文件
	defer os.Remove(packageFilePath)

	// 4. 计算fversion
	fversion := f.calculateFversion(params.Service, "prod", params.Version)

	// 5. 串行部署到各个实例
	successInstances := []string{}
	for _, instanceID := range params.Instances {

		// 5.1 获取实例IP和端口
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

		// 5.2 检查实例健康状态
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

		// 5.3 部署到单个实例
		if err := f.deployToSingleInstance(instanceIP, params.Service, params.Version, fversion, packageFilePath, md5sum); err != nil {
			// 记录错误但继续处理其他实例
			fmt.Printf("部署到实例 %s (%s) 失败: %v\n", instanceID, instanceIP, err)
			continue
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
	packageFilePath, md5sum, err := f.downloadPackage(params.PackageURL)
	if err != nil {
		return nil, fmt.Errorf("下载回滚包文件失败: %v", err)
	}
	// 确保在函数结束时清理临时文件
	defer os.Remove(packageFilePath)

	// 4. 计算fversion
	fversion := f.calculateFversion(params.Service, "prod", params.TargetVersion)

	// 5. 串行回滚到各个实例（单实例容错）
	successInstances := []string{}
	for _, instanceID := range params.Instances {

		// 5.1 获取实例IP和端口
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

		// 5.2 检查实例健康状态
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

		// 5.3 回滚到单个实例
		if err := f.rollbackToSingleInstance(instanceIP, params.Service, params.TargetVersion, fversion, packageFilePath, md5sum); err != nil {
			// 记录错误但继续处理其他实例
			fmt.Printf("回滚到实例 %s (%s) 失败: %v\n", instanceID, instanceIP, err)
			continue
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

// downloadPackage 流式下载包文件到临时文件
func (f *floyDeployService) downloadPackage(packageURL string) (string, []byte, error) {
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

// calculateFversion 计算版本号
func (f *floyDeployService) calculateFversion(service, env, version string) string {
	// 简化的fversion计算（实际应该包含配置文件信息）
	h := md5.New()
	io.WriteString(h, fmt.Sprintf("%s:%s:%s", service, env, version))

	// 添加一个简单的配置文件占位
	io.WriteString(h, ":app.conf")
	io.WriteString(h, "\n\n")
	io.WriteString(h, "# Simple config placeholder")

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

	// 计算签名内容：请求体 + 时间戳 + URI
	sh := crypto.SHA1.New()
	sh.Write(bodyBytes)
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

	// 简化处理：假设总是需要推送包和配置
	// 实际应该解析JSON响应
	return true, true, nil
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

	// 创建文件字段
	fileWriter, err := writer.CreateFormFile("file", version)
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
	// 手动添加 Content-Md5 头
	req.Header.Set("Content-Md5", base64.URLEncoding.EncodeToString(md5sum))

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

	// 创建文件字段
	fileWriter, err := writer.CreateFormFile("file", "app.conf")
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
	// 手动添加自定义头
	req.Header.Set("Content-Md5", base64.URLEncoding.EncodeToString(configMD5[:]))
	req.Header.Set("File-Mode", "644")

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
