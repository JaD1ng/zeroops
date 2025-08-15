# Qiniu1024 MCP Server

## 项目简介
这是一个基于Go语言开发的MCP（Model Context Protocol）服务器，主要用于处理Prometheus监控数据和Superset数据可视化。项目提供了完整的数据获取、转换和格式化功能。

## 项目架构

### 目录结构
```
qiniu1024-mcp-server/
├── config/                 # 配置文件处理
│   └── config.go          # 配置结构体和加载逻辑
├── datasource/            # 数据源模块
│   └── prometheus/        # Prometheus数据源
│       ├── client.go      # Prometheus客户端
│       └── fetch.go       # 数据获取方法
├── datahandler/           # 数据处理模块
│   └── prometheus/        # Prometheus数据处理
│       ├── transform.go   # 数据转换工具
│       ├── export.go      # 数据导出功能
│       └── filter_and_convert_time.go  # 时间格式转换
├── mcp/                   # MCP协议实现
├── dataset/               # 数据集相关
├── config.yaml           # 配置文件
├── main.go               # 主程序入口
└── README.md             # 项目说明文档
```

## 核心功能

### 1. Prometheus数据获取
- **功能**: 从不同区域的Prometheus服务器获取监控数据
- **支持区域**: mock, hd, hb, hn, wz
- **方法**: `FetchByPromQl(promql string)` - 执行PromQL查询

### 2. 数据格式转换
- **时间戳转换**: 将Unix时间戳转换为带时区的完整时间格式（ISO 8601标准）
- **JSON格式化**: 将原始数据整理成格式化的JSON格式，支持美化输出
- **时区处理**: 自动处理东八区（Asia/Shanghai）时区，支持自定义时区
- **多数据类型支持**: 支持瞬时向量、范围向量、标量等多种Prometheus查询结果类型
- **元数据管理**: 可选的元数据信息，包含处理时间和结果类型

### 3. 数据处理工具
- **时间格式转换**: 支持多种时间格式的转换
- **数据过滤**: 支持按时间范围过滤数据
- **CSV导出**: 支持将处理后的数据导出为CSV格式

## 配置说明

### 全局配置
```yaml
global:
  default_file_path: "./testdata"  # 默认文件保存路径
```

**配置说明**:
- `default_file_path`: 指定所有生成文件的默认保存路径
- 支持相对路径（相对于项目根目录）和绝对路径
- 如果路径不存在，系统会自动创建
- 默认值为 `"./dataset"`，即项目根目录下的dataset文件夹

### Prometheus配置
```yaml
prometheus:
  regions:
    mock: "http://127.0.0.1:9090/"
    hd: "http://hd-piko.prometheus.qiniu.io/"
    hb: "http://hb-piko.prometheus.qiniu.io/"
    hn: "http://hn-piko.prometheus.qiniu.io/"
    wz: "http://wz-piko.prometheus.qiniu.io/"
  port: 8090
  endpoint: "/prometheus"
```

### Superset配置
```yaml
superset:
  base_url: "http://superset.yzh-logverse.k8s.qiniu.io/"
  username: "chengye"
  password: "Cy921025"
  port: 8091
  endpoint: "/superset"
```

## 使用方法

### 1. 启动服务
```bash
go run main.go
```

### 2. 使用Prometheus客户端
```go
// 创建客户端
client, err := prometheus.NewPrometheusClient("hd")
if err != nil {
    log.Fatal(err)
}

// 执行查询
result, err := client.FetchByPromQl("up")
if err != nil {
    log.Fatal(err)
}
```

### 3. 数据格式转换

#### 3.1 新的格式化方法（推荐）
```go
// 创建数据格式化器
formatter := data_process.NewPrometheusDataFormatter("Asia/Shanghai")

// 格式化时间戳为带时区的完整时间格式
formattedTime, err := formatter.FormatTimestampWithTimezone(1640995200.0, "")
// 输出: "2022-01-01T08:00:00+08:00"

// 格式化Prometheus查询结果为JSON
formattedJSON, err := formatter.FormatPrometheusData(rawData, true)
```

#### 3.2 简化使用方法
```go
// 简单时间戳格式化
formattedTime := data_process.FormatSimpleTimestamp(1640995200.0)
// 输出: "2022-01-01T08:00:00+08:00"

// 一键格式化数据
formattedJSON, err := data_process.FormatAndPrettyPrint(rawData, "Asia/Shanghai", true)
```

#### 3.3 旧方法（向后兼容）
```go
// 转换时间戳为格式化时间
formattedTime := data_process.FormatTimestamp(1640995200.0)
// 输出: "22-01-01 00:00:00"

// 递归处理JSON数据中的时间戳
data_process.ReplaceTimestampField(jsonData)
```

### 4. 文件路径配置使用

#### 4.1 获取默认文件路径
```go
import "mcp-server/config"

// 获取默认文件保存路径
defaultPath, err := config.GetDefaultFilePath()
if err != nil {
    log.Fatal(err)
}
fmt.Printf("默认文件路径: %s\n", defaultPath)
// 输出: /path/to/project/testdata
```

#### 4.2 获取具体文件路径
```go
// 获取特定文件的完整路径
filePath, err := config.GetFilePath("prometheus_data.json")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("文件路径: %s\n", filePath)
// 输出: /path/to/project/testdata/prometheus_data.json
```

#### 4.3 保存文件示例
```go
// 保存Prometheus查询结果到文件
import (
    "os"
    "mcp-server/config"
)

// 获取文件路径
filePath, err := config.GetFilePath("query_result.json")
if err != nil {
    log.Fatal(err)
}

// 写入文件
err = os.WriteFile(filePath, []byte(jsonData), 0644)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("文件已保存到: %s\n", filePath)
```

### 5. 运行演示程序
```bash
# 运行完整演示
go run demo/formatter_demo.go

# 运行测试
go test ./datahandler/prometheus -v

# 运行性能测试
go test ./datahandler/prometheus -bench=.
```

## 时间格式说明

### 支持的时间格式
1. **Unix时间戳**: 1640995200.0 (秒级时间戳，支持浮点数)
2. **短时间格式**: "25-08-14 12:55:16" (年月日 时分秒)
3. **ISO 8601格式**: "2025-08-14T12:55:16+08:00" (带时区的完整格式，推荐使用)

### 时区处理
- 默认使用东八区（Asia/Shanghai）
- 支持自定义时区：UTC、America/New_York、Europe/London等
- 自动添加时区偏移量（+08:00、-05:00等）
- 支持夏令时自动调整
- 毫秒级精度支持

### 格式化输出示例
```json
{
  "timestamp": "2022-01-01T08:00:00+08:00",
  "timestamp_unix": 1640995200,
  "value": "1",
  "metric": {
    "__name__": "up",
    "instance": "localhost:9090"
  },
  "metadata": {
    "processed_at": "2025-08-14T17:46:36+08:00",
    "result_type": "vector"
  }
}
```

## 错误处理
- 网络连接超时：10秒超时设置
- 配置错误：详细的错误信息提示
- 数据格式错误：优雅的错误处理和恢复

## 开发说明
- 使用Go 1.16+版本
- 遵循SOLID设计原则
- 完整的错误处理和日志记录
- 支持多区域配置管理
