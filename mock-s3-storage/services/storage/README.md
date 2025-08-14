# 文件存储服务 (File Storage Service)

这是一个支持文本文件上传、下载和删除的存储服务，使用PostgreSQL作为存储后端。服务设计为可扩展架构，未来可以轻松支持S3等云存储服务。

## 功能特性

- ✅ **文件上传**: 支持文本文件上传到PostgreSQL数据库
- ✅ **文件下载**: 通过文件ID下载文件
- ✅ **文件删除**: 删除指定文件
- ✅ **文件信息**: 获取文件详细信息
- ✅ **文件列表**: 列出所有已上传的文件
- ✅ **健康检查**: 服务状态监控
- ✅ **可扩展架构**: 支持未来扩展到S3等云存储

## 支持的文件类型

目前支持以下文本文件类型：
- `text/plain` - 纯文本文件
- `text/html` - HTML文件
- `text/css` - CSS样式文件
- `text/javascript` - JavaScript文件
- `application/json` - JSON文件
- `application/xml` - XML文件
- `application/javascript` - JavaScript应用文件
- `application/x-yaml` - YAML文件
- `application/x-toml` - TOML文件
- `application/x-csv` - CSV文件

## 系统要求

- Go 1.21 或更高版本
- PostgreSQL 12 或更高版本
- Docker (用于运行PostgreSQL)

## 快速开始

### 1. 启动PostgreSQL数据库

使用Docker启动PostgreSQL：

```bash
docker run --name postgres-file-storage \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=file_storage \
  -p 5432:5432 \
  -d postgres:15
```

### 2. 安装依赖

```bash
cd storage
go mod tidy
```

### 3. 启动服务

```bash
go run cmd/main.go
```

服务将在 `http://localhost:8080` 启动。

## API接口

### 健康检查
```
GET /api/health
```

**响应示例:**
```json
{
  "status": "healthy",
  "timestamp": "2024-01-01T12:00:00Z",
  "service": "file-storage-service"
}
```

### 文件上传
```
POST /api/files/upload
Content-Type: multipart/form-data
```

**请求参数:**
- `file`: 要上传的文件

**响应示例:**
```json
{
  "success": true,
  "message": "文件上传成功",
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "file_name": "550e8400-e29b-41d4-a716-446655440000.txt",
    "file_size": 1024,
    "content_type": "text/plain",
    "created_at": "2024-01-01 12:00:00",
    "updated_at": "2024-01-01 12:00:00"
  }
}
```

### 文件下载
```
GET /api/files/download/{fileID}
```

**响应:**
- 文件内容作为附件下载

### 文件删除
```
DELETE /api/files/{fileID}
```

**响应示例:**
```json
{
  "success": true,
  "message": "文件删除成功",
  "fileID": "550e8400-e29b-41d4-a716-446655440000"
}
```

### 获取文件信息
```
GET /api/files/{fileID}/info
```

**响应示例:**
```json
{
  "success": true,
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "file_name": "550e8400-e29b-41d4-a716-446655440000.txt",
    "file_size": 1024,
    "content_type": "text/plain",
    "created_at": "2024-01-01 12:00:00",
    "updated_at": "2024-01-01 12:00:00"
  }
}
```

### 文件列表
```
GET /api/files
```

**响应示例:**
```json
{
  "success": true,
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "file_name": "550e8400-e29b-41d4-a716-446655440000.txt",
      "file_size": 1024,
      "content_type": "text/plain",
      "created_at": "2024-01-01 12:00:00",
      "updated_at": "2024-01-01 12:00:00"
    }
  ],
  "count": 1
}
```

## 使用示例

### 使用curl上传文件

```bash
# 上传文本文件
curl -X POST http://localhost:8080/api/files/upload \
  -F "file=@/path/to/your/file.txt"

# 上传JSON文件
curl -X POST http://localhost:8080/api/files/upload \
  -F "file=@/path/to/your/data.json"
```

### 使用curl下载文件

```bash
# 下载文件
curl -O -J http://localhost:8080/api/files/download/{fileID}
```

### 使用curl删除文件

```bash
# 删除文件
curl -X DELETE http://localhost:8080/api/files/{fileID}
```

### 使用curl获取文件信息

```bash
# 获取文件信息
curl http://localhost:8080/api/files/{fileID}/info
```

### 使用curl获取文件列表

```bash
# 获取所有文件列表
curl http://localhost:8080/api/files
```

## 配置说明

### 环境变量

- `PORT`: 服务端口号 (默认: 8080)

### 数据库配置

在 `cmd/main.go` 中可以修改数据库连接配置：

```go
const (
    dbHost     = "localhost"    // 数据库主机
    dbPort     = "5432"         // 数据库端口
    dbUser     = "postgres"     // 数据库用户名
    dbPassword = "postgres"     // 数据库密码
    dbName     = "file_storage" // 数据库名称
    dbSSLMode  = "disable"      // SSL模式
)
```

## 项目结构

```
storage/
├── cmd/
│   └── main.go              # 应用程序入口点
├── internal/
│   ├── config/              # 配置管理
│   │   └── config.go        # 配置结构和加载逻辑
│   ├── handler/             # HTTP处理器
│   │   ├── file_handler.go  # 文件操作处理器
│   │   ├── fault_handler.go # 故障注入处理器
│   │   └── router.go        # 路由配置
│   ├── model/               # 数据模型
│   │   └── file.go          # 文件相关数据结构
│   ├── repository/          # 数据访问层
│   │   ├── factory.go       # 存储工厂
│   │   ├── postgres_storage.go # PostgreSQL存储实现
│   │   ├── s3_storage.go    # S3存储实现（示例）
│   │   └── fault_serviceImpl.go # 故障服务实现
│   └── service/             # 服务接口
│       ├── storage_service.go # 存储服务接口
│       └── fault_service.go # 故障服务接口
├── config.yaml              # 配置文件
├── go.mod                   # Go模块文件
├── go.sum                   # Go依赖校验文件
└── README.md               # 项目文档
```

## 架构设计

### 接口设计

服务采用接口分离原则，定义了 `StorageService` 接口：

```go
type StorageService interface {
    UploadFile(ctx context.Context, fileID, fileName, contentType string, reader io.Reader) (*FileInfo, error)
    DownloadFile(ctx context.Context, fileID string) (io.Reader, *FileInfo, error)
    DeleteFile(ctx context.Context, fileID string) error
    GetFileInfo(ctx context.Context, fileID string) (*FileInfo, error)
    ListFiles(ctx context.Context) ([]*FileInfo, error)
    Close() error
}
```

### 实现架构

- **service包**: 只包含接口定义，遵循接口分离原则
- **repository包**: 包含具体的数据访问实现
  - `PostgresStorage`: PostgreSQL存储实现
  - `S3Storage`: S3存储实现（示例）
  - `StorageFactory`: 存储工厂，用于创建不同的存储实例
- **model包**: 包含数据模型定义
- **config包**: 包含配置管理逻辑

### 扩展性

- **存储后端**: 可以轻松添加新的存储实现（如S3、本地文件系统等）
- **文件类型**: 可以扩展支持图片、视频等二进制文件
- **功能扩展**: 可以添加文件版本控制、权限管理等功能

## 错误处理

服务提供详细的错误信息：

- `400 Bad Request`: 请求参数错误
- `404 Not Found`: 文件不存在
- `405 Method Not Allowed`: 不支持的HTTP方法
- `500 Internal Server Error`: 服务器内部错误

## 性能考虑

- 文件大小限制：1MB（适合文本文件）
- 数据库连接池：自动管理
- 超时设置：读写超时30秒，空闲超时60秒

## 安全考虑

- 文件类型验证：只允许上传文本文件
- 文件大小限制：防止大文件攻击
- CORS支持：允许跨域请求
- 输入验证：验证文件ID格式

## 监控和日志

- 健康检查接口：监控服务状态
- 详细错误日志：便于问题排查
- 请求日志：记录所有API调用

## 未来计划

- [ ] 支持S3云存储
- [ ] 支持图片和视频文件
- [ ] 添加文件版本控制
- [ ] 实现文件权限管理
- [ ] 添加文件压缩功能
- [ ] 支持文件分片上传
- [ ] 添加文件预览功能
- [ ] 实现文件搜索功能

## 贡献

欢迎提交Issue和Pull Request来改进这个项目。

## 许可证

本项目采用MIT许可证。
