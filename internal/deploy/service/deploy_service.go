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
	"io/ioutil"
	"net/http"
	"net/url"
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

	// 解析RSA私钥
	rsaPrivateKey, err := parseRSAPrivateKey(privateKeyPEM)
	if err != nil {
		// 如果解析失败，返回空的服务实例（实际生产环境应该panic或返回error）
		return &floyDeployService{
			privateKey: privateKeyPEM,
			port:       "9902", // 默认端口
		}
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

	data, err := ioutil.ReadFile(configPath)
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

	// 3. 检查RSA私钥是否可用
	if f.rsaPrivateKey == nil {
		return nil, fmt.Errorf("RSA私钥未正确加载")
	}

	// 4. 下载包文件
	packageData, md5sum, err := f.downloadPackage(params.PackageURL)
	if err != nil {
		return nil, err
	}

	// 5. 计算fversion
	fversion := f.calculateFversion(params.Service, "prod", params.Version)

	// 6. 遍历主机列表，对每个主机执行部署
	successfulHosts := []string{}
	for _, hostIP := range params.Hosts {
		// 对单个主机执行部署
		if err := f.deployToSingleHost(hostIP, params.Service, params.Version, fversion, packageData, md5sum); err != nil {
			// 记录错误但继续处理其他主机
			fmt.Printf("部署到主机 %s 失败: %v\n", hostIP, err)
			continue
		}
		successfulHosts = append(successfulHosts, hostIP)
	}

	// 7. 构造返回结果
	result := &model.OperationResult{
		Service:        params.Service,
		Version:        params.Version,
		Instances:      successfulHosts,
		TotalInstances: len(params.Hosts),
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

	// 3. 检查RSA私钥是否可用
	if f.rsaPrivateKey == nil {
		return nil, fmt.Errorf("RSA私钥未正确加载")
	}

	// 4. 下载包文件
	packageData, md5sum, err := f.downloadPackage(params.PackageURL)
	if err != nil {
		return nil, fmt.Errorf("下载包文件失败: %v", err)
	}

	// 5. 计算fversion
	fversion := f.calculateFversion(params.Service, "prod", params.Version)

	// 6. 串行部署到各个实例
	successInstances := []string{}
	for _, instanceID := range params.Instances {

		// 6.1 检查实例健康状态
		healthy, err := CheckInstanceHealth(instanceID)
		if err != nil {
			return nil, fmt.Errorf("实例 %s 健康检查失败: %v", instanceID, err)
		}
		if !healthy {
			return nil, fmt.Errorf("实例 %s 健康检查失败", instanceID)
		}

		// 6.2 获取实例IP
		instanceIP, err := GetInstanceHost(instanceID)
		if err != nil {
			return nil, fmt.Errorf("获取实例 %s 的IP失败: %v", instanceID, err)
		}

		// 6.3 部署到单个实例
		if err := f.deployToSingleInstance(instanceIP, params.Service, params.Version, fversion, packageData, md5sum); err != nil {
			return nil, fmt.Errorf("部署到实例 %s (%s) 失败: %v", instanceID, instanceIP, err)
		}

		successInstances = append(successInstances, instanceID)
	}

	// 8. 返回结果
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

	// 3. 检查目标实例健康状态
	for _, instanceID := range params.Instances {
		healthy, err := CheckInstanceHealth(instanceID)
		if err != nil {
			return nil, err
		}
		if !healthy {
			return nil, fmt.Errorf("实例 %s 健康检查失败", instanceID)
		}
	}

	// 4. 执行回滚逻辑
	// TODO: 实现具体的回滚逻辑
	// - 下载目标版本包
	// - 停止当前服务
	// - 部署目标版本
	// - 启动服务
	// - 验证回滚结果

	// 5. 返回结果
	result := &model.OperationResult{
		Service:        params.Service,
		Version:        params.TargetVersion,
		Instances:      params.Instances,
		TotalInstances: len(params.Instances),
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
	if len(params.Hosts) == 0 {
		return fmt.Errorf("主机列表不能为空")
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

// downloadPackage 下载包文件
func (f *floyDeployService) downloadPackage(packageURL string) ([]byte, []byte, error) {
	resp, err := http.Get(packageURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to download package: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, nil, fmt.Errorf("failed to download package: status %d", resp.StatusCode)
	}

	// 读取包内容
	packageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read package data: %v", err)
	}

	// 计算MD5
	h := md5.New()
	h.Write(packageData)
	md5sum := h.Sum(nil)

	return packageData, md5sum, nil
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

// deployToSingleHost 部署到单个主机
func (f *floyDeployService) deployToSingleHost(hostIP, service, version, fversion string, packageData, md5sum []byte) error {
	// 1. Ping检查
	wantPkg, wantConfig, err := f.ping(hostIP, service, fversion, version, "Auto deploy new service")
	if err != nil {
		return fmt.Errorf("ping检查失败: %v", err)
	}

	// 2. 推送包文件
	if wantPkg {
		if err := f.pushPackage(hostIP, service, fversion, version, packageData, md5sum); err != nil {
			return fmt.Errorf("推送包文件失败: %v", err)
		}
	}

	// 3. 推送配置文件
	if wantConfig {
		if err := f.pushConfig(hostIP, service, fversion); err != nil {
			return fmt.Errorf("推送配置文件失败: %v", err)
		}
	}

	return nil
}

// deployToSingleInstance 部署到单个实例
func (f *floyDeployService) deployToSingleInstance(instanceIP, service, version, fversion string, packageData, md5sum []byte) error {
	// 1. Ping检查
	wantPkg, wantConfig, err := f.ping(instanceIP, service, fversion, version, "Auto deploy")
	if err != nil {
		return fmt.Errorf("ping检查失败: %v", err)
	}

	// 2. 推送包文件
	if wantPkg {
		if err := f.pushPackage(instanceIP, service, fversion, version, packageData, md5sum); err != nil {
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
func (f *floyDeployService) pushPackage(instanceIP, service, fversion, version string, packageData, md5sum []byte) error {
	baseURL := fmt.Sprintf("http://%s:%s", instanceIP, f.port)

	// 构造multipart请求体
	var buf bytes.Buffer
	boundary := fmt.Sprintf("----floy%d", time.Now().Unix())

	// 写入form字段
	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Disposition: form-data; name=\"service\"\r\n\r\n")
	fmt.Fprintf(&buf, "%s\r\n", service)

	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Disposition: form-data; name=\"fversion\"\r\n\r\n")
	fmt.Fprintf(&buf, "%s\r\n", fversion)

	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Disposition: form-data; name=\"pkgOwner\"\r\n\r\n")
	fmt.Fprintf(&buf, "qboxserver\r\n")

	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Disposition: form-data; name=\"installDir\"\r\n\r\n")
	fmt.Fprintf(&buf, "\r\n")

	// 写入文件
	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Disposition: form-data; name=\"file\"; filename=\"%s\"\r\n", version)
	fmt.Fprintf(&buf, "Content-Type: application/octet-stream\r\n")
	fmt.Fprintf(&buf, "Content-Md5: %s\r\n", base64.URLEncoding.EncodeToString(md5sum))
	fmt.Fprintf(&buf, "\r\n")
	buf.Write(packageData)
	fmt.Fprintf(&buf, "\r\n--%s--\r\n", boundary)

	// 创建请求
	req, err := http.NewRequest("POST", baseURL+"/pushPkg", &buf)
	if err != nil {
		return fmt.Errorf("failed to create pushPkg request: %v", err)
	}

	req.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%s", boundary))

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

	// 构造multipart请求体
	var buf bytes.Buffer
	boundary := fmt.Sprintf("----floy%d", time.Now().Unix())

	// 写入form字段
	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Disposition: form-data; name=\"service\"\r\n\r\n")
	fmt.Fprintf(&buf, "%s\r\n", service)

	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Disposition: form-data; name=\"fversion\"\r\n\r\n")
	fmt.Fprintf(&buf, "%s\r\n", fversion)

	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Disposition: form-data; name=\"pkgOwner\"\r\n\r\n")
	fmt.Fprintf(&buf, "qboxserver\r\n")

	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Disposition: form-data; name=\"installDir\"\r\n\r\n")
	fmt.Fprintf(&buf, "\r\n")

	// 写入配置文件
	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Disposition: form-data; name=\"file\"; filename=\"app.conf\"\r\n")
	fmt.Fprintf(&buf, "Content-Type: application/octet-stream\r\n")
	fmt.Fprintf(&buf, "Content-Md5: %s\r\n", base64.URLEncoding.EncodeToString(configMD5[:]))
	fmt.Fprintf(&buf, "File-Mode: 644\r\n")
	fmt.Fprintf(&buf, "\r\n")
	fmt.Fprintf(&buf, "%s", configContent)
	fmt.Fprintf(&buf, "\r\n--%s--\r\n", boundary)

	// 创建请求
	req, err := http.NewRequest("POST", baseURL+"/pushConfig", &buf)
	if err != nil {
		return fmt.Errorf("failed to create pushConfig request: %v", err)
	}

	req.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%s", boundary))

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
