# API 文档 - Monitoring & Alerting Service

## 概述

本文档描述了监控告警服务的 RESTful API 接口，包括告警列表查询、详情获取等核心功能。

人工模拟prometheus调用我们的接受告警接口，收到告警事件


实现状态说明：
- 已实现：接收 Alertmanager Webhook（/v1/integrations/alertmanager/webhook）
- 规划中：告警列表与详情查询接口（本文档描述为对外契约，后续实现）


## 基础信息

- **Base URL**: `/v1`
- **Content-Type**: `application/json`
- **认证方式**: Webhook 端点可通过环境变量启用 Basic 或 Bearer 认证（见下文）。其他查询接口在实现时将采用 Bearer Token。

## 接口列表

### 1. 获取告警列表

获取告警问题列表，支持分页和状态筛选。

**请求：**
```http
GET /v1/issues?start={start}&limit={limit}[&state={state}]
```

**查询参数：**

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| start | string | 否 | 游标。第一页可不传或传空；翻页使用上次响应的 `next` |
| limit | integer | 是 | 每页返回数量，建议范围：1-100 |
| state | string | 否 | 问题状态筛选：`Open`、`Closed` |

分页说明：服务采用基于游标（cursor）的分页。首次请求建议省略 `start`；当返回结果较多时，响应体会包含 `next` 字段，表示下一页的游标。继续翻页时，将该 `next` 作为 `start` 传回。

**响应示例：**
```json
{
  "items": [
    {
      "id": "xxx",
      "state": "Closed",
      "level": "P0",
      "alertState": "Restored",
      "title": "yzh S3APIV2s3apiv2.putobject 0_64K上传响应时间95值:50012ms > 450ms",
      "labels": [
        {"key": "api", "value": "s3apiv2.putobject"},
        {"key": "idc", "value": "yzh"}
      ],
      "alertSince": "2025-05-05T11:00:00.000Z"
    }
  ],
  "next": "xxxx"
}
```

**状态码：**
- `200 OK`: 成功获取列表
- `400 Bad Request`: 参数错误
- `401 Unauthorized`: 认证失败
- `500 Internal Server Error`: 服务器内部错误

### 2. 获取告警详情

获取指定告警问题的详细信息，包括处理历史。

**请求：**
```http
GET /v1/issues/{issueID}
```

**路径参数：**

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| issueID | string | 是 | 告警问题ID |

**响应示例：**
```json
{
  "id": "issue_20250505_001",
  "state": "Closed",
  "level": "P0",
  "alertState": "Restored",
  "title": "yzh S3APIV2s3apiv2.putobject 0_64K上传响应时间95值:50012ms > 450ms",
  "labels": [
    {"key": "api", "value": "s3apiv2.putobject"},
    {"key": "idc", "value": "yzh"},
    {"key": "service", "value": "s3api"}
  ],
  "alertSince": "2025-05-05T11:00:00.000Z",
  "comments": [
    {
      "createdAt": "2025-05-05T11:00:30.000Z",
      "content": "## 自动分析\n\n检测到 S3 API 响应时间异常，可能原因：\n- 后端存储负载过高\n- 网络延迟增加\n\n## 建议处理\n1. 检查存储节点状态\n2. 分析网络监控数据"
    },
    {
      "createdAt": "2025-05-05T11:05:00.000Z",
      "content": "## 自动治愈开始\n\n执行治愈策略：重启相关服务实例"
    },
    {
      "createdAt": "2025-05-05T11:15:00.000Z",
      "content": "## 问题已解决\n\n响应时间恢复正常，告警自动关闭"
    }
  ]
}
```

**状态码：**
- `200 OK`: 成功获取详情
- `404 Not Found`: 告警问题不存在
- `401 Unauthorized`: 认证失败
- `500 Internal Server Error`: 服务器内部错误

## 数据模型

### AlertIssue 对象

| 字段名 | 类型 | 说明 |
|--------|------|------|
| id | string | 告警问题唯一标识 |
| state | string | 告警工单的生命周期状态状态：`Open`、`Closed` |
| level | string | 告警等级：`P0`、`P1`、`P2` |
| alertState | string | 告警本身的实时状态：`Pending`、`Restored`、`AutoRestored`、`InProcessing` |
| title | string | 告警标题描述 |
| labels | Label[] | 标签数组 |
| alertSince | string | 告警发生时间（ISO 8601格式） |
| comments | Comment[] | 处理评论列表（仅详情接口返回） |

### Label 对象

| 字段名 | 类型 | 说明 |
|--------|------|------|
| key | string | 标签键 |
| value | string | 标签值 |

### Comment 对象

| 字段名 | 类型 | 说明 |
|--------|------|------|
| createdAt | string | 评论创建时间（ISO 8601格式） |
| content | string | 评论内容（Markdown格式） |

### 状态语义与映射

- **state**：工单生命周期状态，取值：`Open`、`Closed`。
- **alertState**：告警实时状态，取值：`Pending`、`InProcessing`、`Restored`、`AutoRestored`。

典型关系与约定：
- 当 `state = Open` 时，通常表示仍存在未恢复的相关告警，`alertState` 多为 `Pending`或`InProcessing`。
- 当 `state = Closed` 时，通常表示所有相关告警已恢复，`alertState` 多为 `Restored` 或 `AutoRestored`（自动恢复）。

注意：`alertState` 反映的是告警流的当前快照，而 `state` 管理的是问题工单的生命周期。二者并非强绑定，边界时刻可能出现 `state = Open` 但部分告警已恢复的情况。

## 错误响应

所有接口在出错时返回统一的错误格式：

```json
{
  "error": {
    "code": "INVALID_PARAMETER",
    "message": "参数 limit 必须在 1-100 范围内",
    "details": {
      "field": "limit",
      "value": "150"
    }
  }
}
```

### 错误代码

| 错误代码 | 说明 |
|----------|------|
| INVALID_PARAMETER | 请求参数错误 |
| UNAUTHORIZED | 认证失败 |
| FORBIDDEN | 权限不足 |
| NOT_FOUND | 资源不存在 |
| INTERNAL_ERROR | 服务器内部错误 |

## 使用示例

### curl 示例

```bash
# 获取告警列表
curl -X GET "https://api.example.com/v1/issues?limit=10&state=Open" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json"

# 获取第二页（使用上一页响应中的 next 作为 start）
curl -X GET "https://api.example.com/v1/issues?start=c_abcdef12345&limit=10&state=Open" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json"

# 获取告警详情
curl -X GET "https://api.example.com/v1/issues/issue_20250505_001" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json"
```

### JavaScript 示例

```javascript
// 获取告警列表
const response = await fetch('/v1/issues?limit=10', {
  headers: {
    'Authorization': 'Bearer ' + token,
    'Content-Type': 'application/json'
  }
});
const data = await response.json();

// 获取第二页（将上一页的 next 作为 start 传回）
const nextResponse = await fetch(`/v1/issues?start=${data.next}&limit=10`, {
  headers: {
    'Authorization': 'Bearer ' + token,
    'Content-Type': 'application/json'
  }
});
const nextPage = await nextResponse.json();

// 获取告警详情
const detailResponse = await fetch(`/v1/issues/${issueId}`, {
  headers: {
    'Authorization': 'Bearer ' + token,
    'Content-Type': 'application/json'
  }
});
const detail = await detailResponse.json();
```

### 3. 接收 Alertmanager Webhook（告警接入）

用于接收 Alertmanager 推送的告警事件。

**请求：**
```http
POST /v1/integrations/alertmanager/webhook
Content-Type: application/json
```

**认证：**
- 可选鉴权（通过环境变量开启）：
  - Basic：设置 `ALERT_WEBHOOK_BASIC_USER` 与 `ALERT_WEBHOOK_BASIC_PASS`
  - Bearer：设置 `ALERT_WEBHOOK_BEARER`
  - 若上述变量均未设置，则该端点不强制鉴权（开发/测试便捷）

**请求体（示例 - firing）：**
```json
{
  "receiver": "our-webhook",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "HighRequestLatency",
        "service": "serviceA",
        "severity": "P1",
        "idc": "yzh"
      },
      "annotations": {
        "summary": "p95 latency over threshold",
        "description": "apitime p95 > 450ms"
      },
      "startsAt": "2025-05-05T11:00:00Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://prometheus/graph?g0.expr=...",
      "fingerprint": "3b1b7f4e8f0e"
    }
  ],
  "groupLabels": {"alertname": "HighRequestLatency"},
  "commonLabels": {"service": "serviceA", "severity": "P1"},
  "version": "4"
}
```

**字段要点：**
- `status`: `firing` | `resolved`
- `alerts[]`: 多条告警，关键字段 `labels`、`annotations`、`startsAt`、`fingerprint`
- `fingerprint + startsAt`：用于应用层幂等

**响应：**
- `200 OK {"ok": true, "created": <n>}` 当 `status=firing` 时返回本次创建条数
- `200 OK {"ok": true, "msg": "ignored (not firing)"}` 当非 `firing` 时快速返回

**curl 示例：**
```bash
# firing
curl -X POST http://localhost:8080/v1/integrations/alertmanager/webhook \
  -H 'Content-Type: application/json' \
  -d '{
    "receiver":"our-webhook",
    "status":"firing",
    "alerts":[{
      "status":"firing",
      "labels":{"alertname":"HighRequestLatency","service":"serviceA","severity":"P1","idc":"yzh"},
      "annotations":{"summary":"p95 latency over threshold","description":"apitime p95 > 450ms"},
      "startsAt":"2025-05-05T11:00:00Z",
      "endsAt":"0001-01-01T00:00:00Z",
      "generatorURL":"http://prometheus/graph?g0.expr=...",
      "fingerprint":"3b1b7f4e8f0e"
    }],
    "groupLabels":{"alertname":"HighRequestLatency"},
    "commonLabels":{"service":"serviceA","severity":"P1"},
    "version":"4"
  }'
```

## 版本历史

- **v1.0** (2025-09-11): 初始版本，支持基础的告警列表和详情查询
