# Monitoring & Alerting Service

## 概述

本模块提供 监控/告警处理能力，实现从告警接收到自动治愈的完整生命周期管理。

**核心思想：**
- 告警规则采用 模版 + 服务元数据 (Metadata) 的方式定义，便于统一管理和灵活扩展。
- 支持自动化处理、AI 辅助分析和多级告警分级。

---

## 目录结构

```
alerting/
├── api/            # API 接口层，提供 RESTful 访问
├── database/       # 数据库定义和 migration
├── model/          # 数据模型 (ORM / DTO)
├── service/        # 核心业务逻辑
│   ├── receiver/      # 告警接收与处理
│   ├── ruleset/       # 告警规则模版 + metadata
│   ├── healthcheck/   # 周期体检任务
│   ├── severity/      # 告警等级计算
│   └── remediation/   # 自动化治愈行为
└── README.md       # 项目说明文档
```


---

## 数据库设计

数据库采用4张核心表设计：告警问题表、评论表、模版表和服务元数据表。

详细的表结构设计、索引建议和性能优化方案请参考：**[数据库设计文档](../../docs/alerting/database-design.md)**


---

## 告警规则机制

告警规则由两部分组成：

### 1. 模版 (Template)

定义规则逻辑，带占位符：

```json
{
  "id": "tmpl_apitime",
  "expr": "apitime > {apitime_threshold}",
  "level": "P1"
}
```

### 2. 服务 Metadata (Service Config)

不同服务定义不同的阈值：

```json
{
  "serviceA": { "apitime_threshold": 100 },
  "serviceB": { "apitime_threshold": 50 }
}
```

### 3. 展开后的实际规则 (Resolved Rule)

模版 + metadata → 生成实际规则：

```json
{
  "service": "serviceA",
  "expr": "apitime > 100",
  "level": "P1"
}
```

```json
{
  "service": "serviceB",
  "expr": "apitime > 50",
  "level": "P1"
}
```


---

## 流程图

### 规则生成流程

```mermaid
flowchart TD
    TPL[告警模版<br/>(Templates)<br/>例: apitime > {apitime_threshold}] --> META
    META[服务 Metadata<br/>(Service Config)<br/>例: serviceA=100, serviceB=50] --> RESOLVED
    RESOLVED[实际告警规则<br/>(Resolved Rules)<br/>例: serviceA: apitime>100<br/>serviceB: apitime>50] --> ALERT
    ALERT[告警触发<br/>(Alert Trigger)<br/>生成 Issue & 进入处理流程]
```


---

## API 接口

服务提供 RESTful API 接口，支持告警列表查询、详情获取等核心功能。

主要接口包括：
- `GET /v1/issues` - 获取告警列表
- `GET /v1/issues/{issueID}` - 获取告警详情

完整的接口文档、请求参数、响应格式和使用示例请参考：**[API 文档](../../docs/alerting/api.md)**


---

## 模块功能说明

- **receiver/**：统一接收告警，写入数据库，触发处理流程
- **ruleset/**：管理告警模版 & 服务 metadata，生成实际规则
- **healthcheck/**：周期性体检，提前发现潜在问题
- **severity/**：计算告警等级 = 原始等级 + 影响范围
- **remediation/**：自动化治愈动作（回滚），并记录处理日志

## 相关文档

- [数据库设计文档](../../docs/alerting/database-design.md) - 详细的表结构和索引设计
- [API 文档](../../docs/alerting/api.md) - RESTful API 接口规范

---

## 联调流程（使用 .env 加载环境变量）

以下步骤演示从告警接收到自动治愈的完整链路，且通过 .env 文件加载环境变量（不使用逐条 export）。

### 0) 准备 .env

```bash
cp env_example.txt .env
# 按需编辑 .env，至少确认：
# - DB_* 指向本机 Postgres
# - REDIS_* 指向本机 Redis
# - ALERT_WEBHOOK_BASIC_USER / ALERT_WEBHOOK_BASIC_PASS
# - HC_SCAN_INTERVAL/HC_SCAN_BATCH/HC_WORKERS（可选：例如 1s/50/1 便于观察）
# - REMEDIATION_ROLLBACK_SLEEP（建议 30s，便于观察 InProcessing→Restored）
```

### 1) 启动依赖容器

```bash
docker rm -f zeroops-redis zeroops-pg 2>/dev/null || true
docker run -d --name zeroops-redis -p 6379:6379 redis:7-alpine
docker run -d --name zeroops-pg \
  -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=zeroops \
  -p 5432:5432 postgres:16
until docker exec zeroops-redis redis-cli ping >/dev/null 2>&1; do sleep 0.5; done
until docker exec zeroops-pg pg_isready -U postgres >/dev/null 2>&1; do sleep 0.5; done
```

初始化/校验最小表（不存在则创建）：

```bash
docker exec -i zeroops-postgres-1 psql -U postgres -d zeroops -c \
  "CREATE TABLE IF NOT EXISTS alert_issues (id text primary key, state text, level text, alert_state text, title text, labels json, alert_since timestamp, resolved_at timestamp);"
docker exec -i zeroops-postgres-1 psql -U postgres -d zeroops -c \
  "CREATE TABLE IF NOT EXISTS service_states (service text, version text, report_at timestamp, resolved_at timestamp, health_state text, alert_issue_ids text[], PRIMARY KEY(service,version));"
docker exec -i zeroops-postgres-1 psql -U postgres -d zeroops -c \
  "CREATE TABLE IF NOT EXISTS alert_issue_comments (issue_id text, create_at timestamp, content text, PRIMARY KEY(issue_id, create_at));"
```

### 2) 初始化/重置规则表（alert_rules / alert_rule_metas / alert_meta_change_logs）

注意：该脚本会 DROP 并重建 `alert_rules`、`alert_rule_metas` 和 `alert_meta_change_logs`，仅用于本地/开发环境。

脚本位置：`scripts/sql/alert_rules_bootstrap.sql`

运行方式（任选一种）：

```bash
# 方式 A：通过 docker 容器执行（推荐 dev，本仓文档默认使用名为 zeroops-postgres-1 的容器）
cat scripts/sql/alert_rules_bootstrap.sql | docker exec -i zeroops-postgres-1 psql -U postgres -d zeroops
# 方式 B：本机已安装 psql 客户端
psql -U postgres -d zeroops -f scripts/sql/alert_rules_bootstrap.sql
```

验证：

```bash
docker exec -i zeroops-postgres-1 psql -U postgres -d zeroops -c "SELECT name, severity FROM alert_rules;"
docker exec -i zeroops-postgres-1 psql -U postgres -d zeroops -c "SELECT alert_name, labels, threshold FROM alert_rule_metas;"
docker exec -i zeroops-postgres-1 psql -U postgres -d zeroops -c "SELECT alert_name, change_type, change_time FROM alert_meta_change_logs;"
```

### 2) 清空数据库与缓存（可选，保证从空开始）

```bash
docker exec -i zeroops-pg psql -U postgres -d zeroops -c "TRUNCATE TABLE alert_issue_comments, service_states, alert_issues, alert_meta_change_logs;"
docker exec -i zeroops-redis redis-cli --raw DEL $(docker exec -i zeroops-redis redis-cli --raw KEYS 'alert:*' | tr '\n' ' ') 2>/dev/null || true
docker exec -i zeroops-redis redis-cli --raw DEL $(docker exec -i zeroops-redis redis-cli --raw KEYS 'service_state:*' | tr '\n' ' ') 2>/dev/null || true
```

### 3) 启动服务（使用 .env 加载）

```bash
set -a; . ./.env; set +a
nohup go run ./cmd/zeroops -- 1>/tmp/zeroops.out 2>&1 & echo $!
# 日志：tail -f /tmp/zeroops.out
```

### 4) 触发告警（Webhook）

```bash
export ALERT_WEBHOOK_BASIC_USER=alert
export ALERT_WEBHOOK_BASIC_PASS=REDACTED
```
```bash
curl -s -u "${ALERT_WEBHOOK_BASIC_USER}:${ALERT_WEBHOOK_BASIC_PASS}" -H 'Content-Type: application/json' \
  -X POST http://localhost:8080/v1/integrations/alertmanager/webhook -d '{
  "receiver":"our-webhook",
  "status":"firing",
  "alerts":[{
    "status":"firing",
    "labels":{"alertname":"HighRequestLatency","service":"stg","service_version":"v1.0.4","severity":"P1","idc":"yzh","deploy_id":"deploy-001"},
    "annotations":{"summary":"p95 latency over threshold","description":"apitime p95 > 450ms"},
    "startsAt":"2025-09-15T11:00:00Z",
    "endsAt":"0001-09-16T00:00:00Z",
    "generatorURL":"http://prometheus/graph?g0.expr=...",
    "fingerprint":"manual-fp-001"
  }],
  "groupLabels":{"alertname":"HighRequestLatency"},
  "commonLabels":{"service":"stg","severity":"P1"},
  "version":"4"
}'
```

### 5) 验证 Pending → InProcessing（healthcheck）

```bash
ISSUE_ID=$(docker exec -i zeroops-pg psql -U postgres -d zeroops -t -A -c "SELECT id FROM alert_issues LIMIT 1;"); echo ISSUE_ID=$ISSUE_ID
docker exec -i zeroops-redis redis-cli --raw GET alert:issue:${ISSUE_ID} | jq .alertState
docker exec -i zeroops-redis redis-cli --raw SMEMBERS alert:index:alert_state:InProcessing | grep -c "${ISSUE_ID}" || true
```

### 6) 验证 InProcessing → Restored（remediation）

等待 `REMEDIATION_ROLLBACK_SLEEP` 指定的时间（建议 30s），然后：

```bash
# DB
docker exec -i zeroops-pg psql -U postgres -d zeroops -c "SELECT id,alert_state FROM alert_issues WHERE id='${ISSUE_ID}';"
docker exec -i zeroops-pg psql -U postgres -d zeroops -c "SELECT service,version,health_state,to_char(resolved_at,'YYYY-MM-DD HH24:MI:SS') FROM service_states WHERE service='serviceA' AND version='v1.3.7';"
docker exec -i zeroops-pg psql -U postgres -d zeroops -c "SELECT to_char(create_at,'YYYY-MM-DD HH24:MI:SS') AS ts, substr(content,1,80) FROM alert_issue_comments WHERE issue_id='${ISSUE_ID}' ORDER BY create_at DESC LIMIT 1;"

# Redis
docker exec -i zeroops-redis redis-cli --raw GET alert:issue:${ISSUE_ID} | jq .alertState
docker exec -i zeroops-redis redis-cli --raw GET service_state:serviceA:v1.3.7 | jq '.health_state, .resolved_at'
```

### 7) 查询 API

```bash
curl -s -H 'Authorization: Bearer test' "http://localhost:8080/v1/issues/${ISSUE_ID}" | jq .
curl -s -H 'Authorization: Bearer test' "http://localhost:8080/v1/issues?limit=10&state=Open" | jq .
```

> 提示：如需更容易观察 InProcessing 状态，可将 `REMEDIATION_ROLLBACK_SLEEP` 调大（如 30s+），或适当增大 `HC_SCAN_INTERVAL`。