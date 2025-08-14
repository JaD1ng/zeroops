# Metadata Service

提供元数据存储和查询服务。

## 设计原则

- **被动式服务**：只负责存储和查询元数据，不主动创建对象
- **单一职责**：专注于元数据管理，不涉及文件存储逻辑
- **S3兼容**：支持 bucket/key 格式，但内部统一为完整key存储

## 核心功能

### 元数据操作
- `POST /metadata` - 保存元数据（由上层文件上传服务调用）
- `GET /metadata/get?key=bucket/object` - 获取元数据
- `DELETE /metadata/delete?key=bucket/object` - 删除元数据
- `PUT /metadata/update?key=bucket/object` - 更新元数据
- `GET /metadata` - 列出所有元数据（分页）

### 查询功能
- `GET /metadata/search?q=keyword` - 关键字搜索
- `GET /metadata/pattern?pattern=bucket/*` - 模式匹配
- `GET /buckets?bucket=mybucket` - 按bucket列出对象（S3兼容）

### 管理功能
- `GET /metadata/stats` - 统计信息
- `GET /metadata/export` - 导出元数据
- `POST /metadata/import` - 导入元数据

## 数据模型

```go
type MetadataEntry struct {
    ID           string    `json:"id"`
    Key          string    `json:"key"`              // 完整key: bucket/object-key
    Size         int64     `json:"size"`
    ContentType  string    `json:"content_type"`
    MD5Hash      string    `json:"md5_hash"`
    StorageNodes []string  `json:"storage_nodes"`    // 存储节点列表
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}
```

## 与原始服务对比

| 功能 | 原始服务 | 新微服务 | 状态 |
|------|----------|----------|------|
| 保存元数据 | ✅ SaveMetadata | ✅ SaveMetadata | ✅ |
| 获取元数据 | ✅ GetMetadata | ✅ GetMetadata | ✅ |
| 删除元数据 | ✅ DeleteMetadata | ✅ DeleteMetadata | ✅ |
| 列表查询 | ✅ ListMetadata | ✅ ListMetadata | ✅ |
| 搜索功能 | ✅ SearchMetadata | ✅ SearchMetadata | ✅ |
| 模式匹配 | ✅ GetMetadataByPattern | ✅ GetMetadataByPattern | ✅ |
| 导入导出 | ✅ Import/ExportMetadata | ✅ Import/ExportMetadata | ✅ |
| 统计信息 | ✅ GetStats | ✅ GetStats | ✅ |

## 使用示例

### 上层服务保存元数据
```bash
curl -X POST http://localhost:8080/metadata \
  -H "Content-Type: application/json" \
  -d '{
    "id": "uuid-123",
    "key": "mybucket/myfile.txt",
    "size": 1024,
    "content_type": "text/plain",
    "md5_hash": "abc123...",
    "storage_nodes": ["node1", "node2"]
  }'
```

### 获取对象元数据
```bash
curl "http://localhost:8080/metadata/get?key=mybucket/myfile.txt"
```

### 按bucket列出对象
```bash
curl "http://localhost:8080/buckets?bucket=mybucket"
```

### 搜索元数据
```bash
curl "http://localhost:8080/metadata/search?q=txt&limit=10"
```

## 配置

使用 `config.yaml` 或环境变量配置服务：

```yaml
service:
  name: "metadata-service"
  port: 8080

database:
  type: "sqlite"
  database: "metadata.db"
```

## 故障演练

支持内置故障注入：
- 内存泄漏模拟
- 随机错误注入（保存5%，获取2%失败率）

## 架构简化

相比之前的复杂设计，新架构：
- 减少了约60%的代码量
- 移除了不必要的Bucket独立管理
- 对齐了原始服务的API设计
- 保持了微服务的核心优势