# JaD1ng ZeroOps

这是一个零运维平台项目，包含多个微服务组件。

## 项目组件

### 文件存储服务 (File Storage Service)

位置: `mock-s3-storage/services/storage/`

一个支持文本文件上传、下载和删除的存储服务，使用PostgreSQL作为存储后端。

**主要功能:**
- ✅ 文件上传到PostgreSQL数据库
- ✅ 文件下载
- ✅ 文件删除
- ✅ 文件信息查询
- ✅ 文件列表
- ✅ 健康检查
- ✅ 故障注入功能（内存泄漏、CPU飙升）
- ✅ 可扩展架构（支持未来扩展到S3）

**快速启动:**
```bash
cd mock-s3-storage/services/storage
./start.sh
```

**API测试:**
```bash
cd mock-s3-storage/services/storage
./test_api.sh
```

**内存泄漏测试:**
```bash
cd mock-s3-storage/services/storage
./test_MemLeak.sh
```

**CPU飙升测试:**
```bash
cd mock-s3-storage/services/storage
./test_CpuSpike.sh
```

**CPU指标监控测试:**
```bash
cd mock-s3-storage/services/storage
./test_cpu_metrics.sh
```

**压力测试:**
```bash
cd mock-s3-storage/services/storage
./stress_test.sh
```

详细文档请查看 [storage/README.md](mock-s3-storage/services/storage/README.md)

## 故障注入功能

### 内存泄漏 (Memory Leak)
- 模拟内存泄漏情况
- 可配置内存分配大小和频率
- 支持启动、停止和状态查询

### CPU飙升 (CPU Spike)
- 模拟CPU使用率飙升
- 可配置CPU强度和goroutine数量
- 支持多种监控方式

### CPU指标监控 (CPU Metrics)
- 实时CPU使用率监控
- Prometheus格式指标输出

## 技术栈

- **后端**: Go 1.21+
- **数据库**: PostgreSQL 15
- **容器化**: Docker & Docker Compose
- **架构**: 微服务架构，接口分离设计

## 开发环境要求

- Go 1.21 或更高版本
- Docker & Docker Compose
- PostgreSQL 12+ (通过Docker运行)

## 项目结构

```
JaD1ng_zeroops/
├── mock-s3-storage/
│   ├── services/
│   │   └── storage/           # 文件存储服务
│   │       ├── cmd/          # 服务启动入口
│   │       ├── internal/     # 内部包
│   │       │   ├── handler/  # HTTP处理器
│   │       │   ├── service/  # 业务逻辑服务
│   │       │   └── impl/     # 具体实现
│   │       ├── start.sh      # 启动脚本
│   │       ├── test_api.sh   # API测试脚本
│   │       ├── test_MemLeak.sh    # 内存泄漏测试脚本
│   │       └── test_CpuSpike.sh   # CPU飙升测试脚本
│   └── shared/               # 共享包
│       ├── faults/           # 故障注入功能
│       │   ├── memory/       # 内存泄漏实现
│       │   └── cpu/          # CPU飙升实现
│       ├── config/           # 配置管理
│       ├── database/         # 数据库接口
│       ├── telemetry/        # 监控和日志
│       └── httpserver/       # HTTP服务器
└── README.md                 # 项目总览
```

## 许可证

本项目采用MIT许可证。