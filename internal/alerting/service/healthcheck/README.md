# healthcheck — 告警扫描与分发任务

本包提供两个定时任务：

## 1. Pending 告警扫描与分发任务
- 周期性扫描 Pending 状态的告警
- 将告警投递到 channel（供下游处理器消费），后续再接入消息队列
- 成功投递后，原子地把缓存中的状态更新：
  - `alert:issue:{id}` 的 `alertState`：Pending → InProcessing
  - `service_state:{service}:{version}` 的 `health_state`：由告警等级推导（P0→Error；P1/P2→Warning）

此任务默认只更新缓存，不直接更新数据库，以降低耦合与避免与业务处理竞争。数据库状态可由下游处理器在处理开始时回写，或由后续补偿任务兜底。

## 2. Prometheus 时序数据异常检测任务
- 周期性从 Prometheus 获取埋点时序数据
- 从 `alert_rules` 和 `alert_rule_metas` 表组合生成完整的 PromQL 查询
- 调用异常检测 API 分析时序数据
- 过滤异常结果，只处理 P0/P1 级别的异常
- 与现有告警时间区间进行比对，避免重复告警

——

## 1. Pending 告警扫描任务

### 1.1 触发与频率

- 间隔：默认每 10s 扫描一次（可配置）
- 批量：每次最多处理 200 条 Pending（可配置）
- 并发：串行或小并发（<= 4），避免重复投递

环境变量建议：
```
HC_SCAN_INTERVAL=10s
HC_SCAN_BATCH=200
HC_WORKERS=1
```

——

### 1.2 数据来源与过滤

优先以数据库为准，结合缓存加速：

- 数据库查询（推荐）
  ```sql
  SELECT id, level, title, labels, alert_since
  FROM alert_issues
  WHERE alert_state = 'Pending'
  ORDER BY alert_since ASC
  LIMIT $1;
  ```

当告警切换为 InProcessing 时，需要更新对应 `service_states.report_at` 为该 service/version 关联的 `alert_issue_ids` 中，所有 alert_issues 里 alert_state=InProcessing 的 `alert_since` 最早时间（min）。可通过下游处理器或本任务的补充逻辑回填：

```sql
UPDATE service_states ss
SET report_at = sub.min_since
FROM (
  SELECT si.service, si.version, MIN(ai.alert_since) AS min_since
  FROM service_states si
  JOIN alert_issues ai ON ai.id = ANY(si.alert_issue_ids)
  WHERE ai.alert_state = 'InProcessing'
  GROUP BY si.service, si.version
) AS sub
WHERE ss.service = sub.service AND ss.version = sub.version;
```

- 或仅用缓存（可选）：
  - 维护集合 `alert:index:alert_state:Pending`（若未维护，可临时 SCAN `alert:issue:*` 并过滤 JSON 中的 `alertState`，但不推荐在大规模下使用 SCAN）。

——

### 1.3 通道（channel）

现阶段通过进程内 channel 向下游处理器传递告警消息，后续再平滑切换为消息队列（Kafka/NATS 等）。

消息格式保留为 `AlertMessage`：
```go
type AlertMessage struct {
    ID         string            `json:"id"`
    Service    string            `json:"service"`
    Version    string            `json:"version,omitempty"`
    Level      string            `json:"level"`
    Title      string            `json:"title"`
    AlertSince time.Time         `json:"alert_since"`
    Labels     map[string]string `json:"labels"`
}
```

发布样例（避免阻塞可用非阻塞写）：
```go
func publishToChannel(ctx context.Context, ch chan<- AlertMessage, m AlertMessage) error {
    select {
    case ch <- m:
        return nil
    default:
        return fmt.Errorf("alert channel full")
    }
}
```

配置：当前无需队列相关配置。未来切换到消息队列时，可启用以下配置项：
```
# ALERT_QUEUE_KIND=redis_stream|kafka|nats
# ALERT_QUEUE_DSN=redis://localhost:6379/0
# ALERT_QUEUE_TOPIC=alerts.pending
```

——

### 1.4 缓存键与原子更新

现有（或建议）键：
- 告警：`alert:issue:{id}` → JSON，字段包含 `alertState`
- 指数（可选）：`alert:index:alert_state:{Pending|InProcessing|...}`
- 服务态：`service_state:{service}:{version}` → JSON，字段包含 `health_state`
- 指数：`service_state:index:health:{Error|Warning|...}`

为避免并发写冲突，建议使用 Lua CAS（Compare-And-Set）脚本原子修改值与索引：

```lua
-- KEYS[1] = alert key, ARGV[1] = expected, ARGV[2] = next, KEYS[2] = idx:old, KEYS[3] = idx:new, ARGV[3] = id
local v = redis.call('GET', KEYS[1])
if not v then return 0 end
local obj = cjson.decode(v)
if obj.alertState ~= ARGV[1] then return -1 end
obj.alertState = ARGV[2]
redis.call('SET', KEYS[1], cjson.encode(obj), 'KEEPTTL')
if KEYS[2] ~= '' then redis.call('SREM', KEYS[2], ARGV[3]) end
if KEYS[3] ~= '' then redis.call('SADD', KEYS[3], ARGV[3]) end
return 1
```

服务态类似（示例将态切换到推导的新态）：
```lua
-- KEYS[1] = service_state key, ARGV[1] = expected(optional), ARGV[2] = next, KEYS[2] = idx:old(optional), KEYS[3] = idx:new, ARGV[3] = member
local v = redis.call('GET', KEYS[1])
if not v then return 0 end
local obj = cjson.decode(v)
if ARGV[1] ~= '' and obj.health_state ~= ARGV[1] then return -1 end
obj.health_state = ARGV[2]
redis.call('SET', KEYS[1], cjson.encode(obj), 'KEEPTTL')
if KEYS[2] ~= '' then redis.call('SREM', KEYS[2], ARGV[3]) end
if KEYS[3] ~= '' then redis.call('SADD', KEYS[3], ARGV[3]) end
return 1
```

——

### 1.5 任务流程（伪代码）

```go
func runOnce(ctx context.Context, db *Database, rdb *redis.Client, ch chan<- AlertMessage, batch int) error {
    rows := queryPendingFromDB(ctx, db, batch) // id, level, title, labels(JSON), alert_since
    for _, it := range rows {
        svc := it.Labels["service"]
        ver := it.Labels["service_version"]
        // 1) 投递消息到 channel（非阻塞）
        select {
        case ch <- AlertMessage{ID: it.ID, Service: svc, Version: ver, Level: it.Level, Title: it.Title, AlertSince: it.AlertSince, Labels: it.Labels}:
            // ok
        default:
            // 投递失败：通道已满，跳过状态切换，计数并继续
            continue
        }
        // 2) 缓存状态原子切换（告警）
        alertKey := "alert:issue:" + it.ID
        rdb.Eval(ctx, alertCAS, []string{alertKey, "alert:index:alert_state:Pending", "alert:index:alert_state:InProcessing"}, "Pending", "InProcessing", it.ID)
        // 3) 缓存状态原子切换（服务态：按告警等级推导）
        if svc != "" { // version 可空
            target := deriveHealth(it.Level) // P0->Error; P1/P2->Warning; else Warning
            svcKey := "service_state:" + svc + ":" + ver
            -- 可按需指定旧态索引，否则留空
            localOld := ''
            newIdx := "service_state:index:health:" + target
            member := svcKey
            rdb.Eval(ctx, svcCAS, []string{svcKey, localOld, newIdx}, '', target, member)
        }
    }
    return nil
}

func StartScheduler(ctx context.Context, deps Deps) {
    t := time.NewTicker(deps.Interval)
    defer t.Stop()
    for {
        select {
        case <-ctx.Done(): return
        case <-t.C:
            _ = runOnce(ctx, deps.DB, deps.Redis, deps.AlertCh, deps.Batch)
        }
    }
}
```

——

### 1.6 可观测与重试

- 指标：扫描次数、选出数量、成功投递数量、CAS 成功/失败数量、用时分位
- 日志：每批开始/结束、首尾 ID、错误明细
- 重试：
  - 消息投递失败：不更改缓存状态，等待下次扫描重试
  - CAS 返回 -1（状态被他处更改）：记录并跳过

——

### 1.7 本地验证

1) 准备 Redis 与 DB（见 receiver/README.md）

2) 造数据：插入一条 `alert_issues.alert_state='Pending'` 且缓存中存在 `alert:issue:{id}` 的 JSON。

3) 启动任务：观察日志/指标。

4) 验证缓存：
```bash
redis-cli --raw GET alert:issue:<id> | jq
redis-cli --raw SMEMBERS alert:index:alert_state:InProcessing | head -n 20
redis-cli --raw GET service_state:<service>:<version> | jq
redis-cli --raw SMEMBERS service_state:index:health:Processing | head -n 20
```

5) 验证 channel：在消费端确认是否收到消息。

——

## 2. Prometheus 时序数据异常检测任务

### 2.1 触发与频率

- 间隔：默认每 6 小时执行一次（可配置）
- 查询范围：默认查询过去 6 小时的时序数据（可配置）
- 查询步长：默认 1 分钟步长（可配置）

环境变量建议：
```
PROMETHEUS_ANOMALY_INTERVAL=6h
PROMETHEUS_QUERY_RANGE=6h
PROMETHEUS_QUERY_STEP=1m
```

### 2.2 数据来源与处理流程

1. **获取告警规则**：从 `alert_rules` 表获取所有告警规则
2. **获取规则元信息**：从 `alert_rule_metas` 表获取标签和阈值信息
3. **构建 PromQL**：将规则表达式与标签组合成完整的 PromQL 查询
4. **执行查询**：调用 Prometheus `/api/v1/query_range` API 获取时序数据
5. **异常检测**：将每条时序数据单独发送到异常检测 API 进行分析
6. **结果过滤**：只保留 P0/P1 级别的异常
7. **时间区间过滤**：与现有告警时间区间进行比对，避免重复告警

### 2.3 PromQL 构建示例

原始规则：
- `alert_rules.expr`: `sum(rate(http_request_duration{}[5m])) by (service, service_version)`
- `alert_rule_metas.labels`: `{"service":"s3","service_version":"1.0.4"}`

组合后的 PromQL：
```promql
sum(rate(http_request_duration{service="s3", service_version="1.0.4"}[5m])) by (service, service_version)
```

### 2.4 异常检测处理方式

系统采用**逐条处理**的方式调用异常检测 API：

- **处理方式**：将每条时序数据单独发送到异常检测 API 进行分析
- **优势**：
  - 提高处理精度：每条时序数据独立分析，避免相互干扰
  - 增强容错性：单条数据检测失败不影响其他数据的处理
  - 便于调试：可以精确定位哪条时序数据出现问题
  - 支持并发：可以并行处理多条时序数据（未来扩展）

- **处理流程**：
  1. 遍历所有收集到的时序数据
  2. 对每条时序数据单独调用异常检测 API
  3. 收集所有检测结果
  4. 合并异常检测结果

### 2.5 异常检测 API

#### 2.5.1 API 请求格式

系统向异常检测 API 发送的请求格式：
```json
{
    "metadata": {
        "alert_name": "http_latency",
        "severity": "P0",
        "labels": {
            "service": "s3",
            "version": "v1.0.4"
        }
    },
    "data": [
        {
          "timestamp": "2024-01-01T00:00:00Z",
          "value": 45.2
        },
        {
          "timestamp": "2024-01-01T00:01:00Z",
          "value": 48.5
        }
    ]
}
```

#### 2.5.2 API 响应格式

**检测出异常**：
```json
{
    "metadata": {
        "alert_name": "http_latency",
        "severity": "P0",
        "labels": {
            "service":"s3",
            "version":"v1.0.4"
        }
    },
    "anomalies": [
        {
            "start": "2025-09-24T00:00:00Z",
            "end": "2025-09-24T00:06:00Z",
        }
    ]
}
```

**无异常**：
```json
{
    "metadata": {
        "alert_name": "http_latency",
        "severity": "P0",
        "labels": {
            "service":"s3",
            "version":"v1.0.4"
        }
    },
    "anomalies": []
}
```

#### 2.5.3 处理优势

通过携带 metadata 信息，系统可以：
- **精确识别**：直接知道是哪个告警规则检测出了异常
- **快速定位**：明确知道是哪个服务、版本组合出现问题
- **便于处理**：可以根据告警等级和标签信息进行差异化处理
- **简化逻辑**：无需额外的映射和查找逻辑

### 2.6 时间区间过滤逻辑

对于每个检测到的异常：
2. 查询 `alert_issues` 表中 `alert_state` 为 'InProcessing' 或 'Pending' 的记录
3. 检查异常时间区间是否与现有告警时间区间重叠：
   - 如果告警已解决（`resolved_at` 不为空）：检查异常是否在 `alert_since` 到 `resolved_at` 区间内
   - 如果告警未解决（`resolved_at` 为空）：检查异常是否在 `alert_since` 之后
4. 如果异常时间区间与现有告警重叠，则跳过该异常
5. 对于未重叠的异常，执行后续处理逻辑（待实现）

### 2.7 任务流程（伪代码）

```go
func runPrometheusAnomalyDetection(ctx context.Context, deps PrometheusDeps) error {
    // 1. 获取告警规则和元信息
    rules := QueryAlertRules(ctx, db)
    metas := QueryAlertRuleMetas(ctx, db)
    
    // 2. 构建 PromQL 查询
    queries := buildPromQLQueries(rules, metas)
    
    // 3. 计算查询时间范围
    end := time.Now()
    start := end.Add(-deps.QueryRange)
    
    // 4. 执行 Prometheus 查询
    for _, query := range queries {
        resp := prometheusClient.QueryRange(ctx, query.Expr, start, end, deps.QueryStep)
        allTimeSeries = append(allTimeSeries, resp.Data.Result...)
    }
    
    // 5. 调用异常检测 API（逐条处理时序数据，携带 metadata）
    anomalyResp := prometheusClient.DetectAnomalies(ctx, allTimeSeries, allQueries)
    
    // 6. 过滤异常结果
    alertIssues := QueryAlertIssuesForTimeFilter(ctx, db)
    filteredAnomalies := FilterAnomaliesByAlertTimeRanges(anomalyResp.Anomalies, alertIssues)
    
    // 7. 处理过滤后的异常（待实现）
    for _, anomaly := range filteredAnomalies {
        // TODO: 实现异常处理逻辑
    }
    
    return nil
}
```

### 2.8 可观测与错误处理

- 指标：查询次数、时序数据点数量、检测到的异常数量、过滤后的异常数量、执行时间
- 日志：任务开始/结束、查询详情、异常检测结果、过滤结果
- 错误处理：
  - Prometheus 查询失败：记录错误并继续处理其他查询
  - 异常检测 API 失败：记录错误并跳过本次检测
  - 单条时序数据检测失败：记录错误并继续处理下一条数据
  - 数据库查询失败：记录错误并终止任务

### 2.9 本地验证

1) 准备环境：
   - Prometheus 实例（可访问 `/api/v1/query_range`）
   - 异常检测 API（或使用 mock 模式）
   - 数据库中有 `alert_rules` 和 `alert_rule_metas` 数据

2) 配置环境变量：
   ```
   PROMETHEUS_URL=http://localhost:9090
   ANOMALY_DETECTION_API_URL=http://localhost:8081/api/v1/anomaly/detect
   ```

3) 启动任务：观察日志输出

4) 验证结果：
   - 检查 Prometheus 查询是否成功执行
   - 验证异常检测 API 调用
   - 确认时间区间过滤逻辑正确

——

## 3. 配置汇总

```
# Pending 告警扫描任务
HC_SCAN_INTERVAL=10s
HC_SCAN_BATCH=200
HC_WORKERS=1

# Prometheus 异常检测任务
PROMETHEUS_ANOMALY_INTERVAL=6h
PROMETHEUS_QUERY_STEP=1m
PROMETHEUS_QUERY_RANGE=6h
PROMETHEUS_URL=http://localhost:9090
PROMETHEUS_QUERY_TIMEOUT=30s

# 异常检测 API
ANOMALY_DETECTION_API_URL=http://localhost:8081/api/v1/anomaly/detect
ANOMALY_DETECTION_API_TIMEOUT=10s

# 通道
# 当前无需额外配置
# 预留（未来切换到消息队列时启用）：
# ALERT_QUEUE_KIND=redis_stream|kafka|nats
# ALERT_QUEUE_DSN=redis://localhost:6379/0
# ALERT_QUEUE_TOPIC=alerts.pending
```

——


