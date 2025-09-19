# 发布系统实现说明

## 总体发布流程

发布系统基于 Floy 协议实现，采用 RSA 签名认证，支持包文件和配置文件的推送部署。整个发布过程分为以下几个主要步骤：

### 1. 参数验证与准备阶段
- 验证发布参数的完整性和有效性
- 验证包URL的可访问性
- 检查RSA私钥是否正确加载

### 2. 包文件处理阶段
- 从指定URL下载包文件
- 计算包文件的MD5校验和
- 生成部署版本号(fversion)

### 3. 实例部署阶段
- 遍历目标实例列表
- 对每个实例执行健康检查
- 获取实例IP地址
- 执行单实例部署流程

### 4. 单实例部署流程
- Ping检查：确认实例状态和需要推送的内容
- 推送包文件：如果需要，推送二进制包文件
- 推送配置：如果需要，推送应用配置文件
- 验证部署结果

## 核心方法详解

### 主流程方法

#### `ExecuteDeployment(params *model.DeployParams) (*model.OperationResult, error)`
**作用**: 执行完整的发布操作
**实现步骤**:
1. 调用 `validateDeployParams()` 验证参数
2. 调用 `ValidatePackageURL()` 验证包URL
3. 检查RSA私钥是否可用
4. 调用 `downloadPackage()` 下载包文件
5. 调用 `calculateFversion()` 计算版本号
6. 遍历实例列表，对每个实例执行部署

**为什么需要**: 这是发布系统的核心入口方法，协调整个发布流程的执行。

#### `deployToSingleInstance(instanceIP, service, version, fversion string, packageData, md5sum []byte) error`
**作用**: 对单个实例执行部署操作
**实现步骤**:
1. 调用 `ping()` 检查实例状态
2. 根据ping结果决定是否推送包文件和配置
3. 调用 `pushPackage()` 推送包文件（如需要）
4. 调用 `pushConfig()` 推送配置文件（如需要）

**为什么需要**: 将复杂的多实例部署拆分为单实例操作，便于错误处理和状态跟踪。

### 网络通信方法

#### `ping(instanceIP, service, fversion, version, message string) (bool, bool, error)`
**作用**: 检查Floyd服务状态，确定需要推送的内容
**实现方式**:
- 构造POST请求到 `/ping` 端点
- 发送服务名、版本号等参数
- 使用RSA签名认证请求
- 解析响应确定是否需要推送包和配置

**为什么需要**: Floyd协议要求先ping确认实例状态，避免不必要的文件传输。

#### `pushPackage(instanceIP, service, fversion, version string, packageData, md5sum []byte) error`
**作用**: 推送二进制包文件到目标实例
**实现方式**:
- 构造multipart/form-data格式的POST请求
- 包含服务信息、版本信息和二进制文件数据
- 添加MD5校验头确保文件完整性
- 使用RSA签名认证请求

**为什么需要**: 将应用程序包传输到目标实例是部署的核心步骤。

#### `pushConfig(instanceIP, service, fversion string) error`
**作用**: 推送配置文件到目标实例
**实现方式**:
- 生成应用配置内容
- 构造multipart/form-data格式的POST请求
- 设置文件权限（644）
- 使用RSA签名认证请求

**为什么需要**: 应用程序通常需要特定的配置文件才能正确运行。

#### `signRequest(req *http.Request) error`
**作用**: 为HTTP请求添加RSA签名认证
**实现方式**:
- 读取请求体内容
- 生成纳秒级时间戳
- 计算 SHA1(请求体 + 时间戳 + URI) 哈希
- 使用RSA私钥对哈希进行PKCS1v15签名
- 设置 TimeStamp 和 Authorization 请求头

**为什么需要**: Floyd协议要求所有请求都必须经过RSA签名认证，确保请求的完整性和来源可信。

### 文件处理方法

#### `downloadPackage(packageURL string) ([]byte, []byte, error)`
**作用**: 从指定URL下载包文件并计算MD5
**实现方式**:
- 发送HTTP GET请求到包URL
- 读取响应体内容到内存
- 使用MD5算法计算文件校验和
- 返回文件内容和MD5值

**为什么需要**: 部署前需要获取要部署的包文件，MD5用于传输完整性验证。

#### `calculateFversion(service, env, version string) string`
**作用**: 计算部署版本号
**实现方式**:
- 将服务名、环境、版本号组合成字符串
- 添加配置文件占位符信息
- 计算MD5哈希值
- 转换为Base64URL编码
- 去除填充字符和特殊前缀

**为什么需要**: Floyd协议使用fversion作为部署版本的唯一标识，包含了服务和配置信息。

### 实例管理方法

#### `GetInstanceHost(instanceID string) (string, error)` (内部工具函数)
**作用**: 根据实例ID获取对应的IP地址
**实现方式**:
- 验证实例ID格式
- 查询静态映射表或调用外部API
- 支持多种查询策略（硬编码、规则解析、数据库、API）

**为什么需要**: 实例ID是逻辑标识，需要转换为实际的IP地址才能进行网络通信。

#### `CheckInstanceHealth(instanceID string) (bool, error)` (内部工具函数)
**作用**: 检查实例是否健康可用
**实现方式**:
- 根据实例ID获取IP地址
- 发送健康检查请求
- 验证响应状态

**为什么需要**: 部署前需要确认目标实例可用，避免向不可用实例部署。

#### `ValidatePackageURL(packageURL string) error` (内部工具函数)
**作用**: 验证包URL是否可访问
**实现方式**:
- 发送HEAD请求检查URL有效性
- 验证响应状态码
- 检查内容类型和大小

**为什么需要**: 提前验证包URL可以避免在部署过程中出现下载失败。

### 密钥管理方法

#### `loadPrivateKeyFromConfig() string`
**作用**: 从配置文件加载RSA私钥
**实现方式**:
- 读取YAML配置文件
- 解析privateKey字段
- 清理多余的空白字符

**为什么需要**: RSA私钥用于请求签名，需要从安全的配置文件中加载。

#### `parseRSAPrivateKey(privateKeyPEM string) (*rsa.PrivateKey, error)`
**作用**: 解析PEM格式的RSA私钥
**实现方式**:
- 添加PEM头尾（如果缺失）
- 使用pem.Decode解码PEM块
- 使用x509.ParsePKCS1PrivateKey解析私钥

**为什么需要**: 将字符串格式的私钥转换为Go可用的RSA私钥对象。

### 参数验证方法

#### `validateDeployParams(params *model.DeployParams) error`
**作用**: 验证发布参数的完整性
**实现方式**:
- 检查参数是否为nil
- 验证必填字段是否为空
- 检查实例列表是否有效

**为什么需要**: 参数验证可以提前发现问题，避免执行无效的部署操作。

## 错误处理策略

### 1. 快速失败原则
- 参数验证失败立即返回错误
- 私钥加载失败立即返回错误
- 包下载失败立即返回错误

### 2. 单实例容错
- 单个实例部署失败不影响其他实例
- 记录成功部署的实例列表
- 提供详细的错误信息定位问题

### 3. 网络超时控制
- 下载包文件：300秒超时
- Ping检查：30秒超时
- 推送配置：60秒超时

## 安全特性

### 1. RSA签名认证
- 所有HTTP请求都经过RSA私钥签名
- 使用SHA1哈希算法
- 包含时间戳防止重放攻击

### 2. 文件完整性验证
- 使用MD5校验和验证文件传输完整性
- 在HTTP头中传递校验信息

### 3. 参数安全
- 敏感信息（如私钥）从配置文件加载
- 参数验证防止注入攻击
- 错误信息不暴露敏感细节

## 扩展点

### 1. 实例发现
- `GetInstanceHost()` 方法支持多种实例发现策略
- 可集成数据库、API、服务发现系统

### 2. 健康检查
- `CheckInstanceHealth()` 可定制健康检查逻辑
- 支持不同的健康检查协议

### 3. 包存储
- 支持不同的包存储后端（HTTP、S3、本地文件等）
- 可扩展包格式支持

### 4. 配置管理
- 配置文件生成逻辑可定制
- 支持模板化配置生成