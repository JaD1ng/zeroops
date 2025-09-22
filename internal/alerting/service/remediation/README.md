# remediation — 告警治愈与下钻分析

本包实现一个后台处理器：消费 `healthcheck` 投递到进程内 channel 的告警消息，根据告警等级进行分流处理：
- **P0 告警**：进入"故障治愈"模块，执行自动修复操作
- **P1/P2 告警**：进入"下钻分析"模块，进行深度分析

——

## 1. 目标

- 订阅 `healthcheck` 的 `AlertMessage`（进程内 channel）
- 根据 `level` 字段进行分流：
  - **P0 告警**：故障治愈流程
    1) 确认故障域（从 labels 分析 service_name + version）
    2) 查询 `heal_actions` 表获取治愈方案
    3) 执行治愈操作（当前仅支持回滚）
    4) 治愈成功后启动观察窗口（默认30分钟）
    5) 观察窗口内如果出现新告警，取消观察并重新处理
    6) 观察窗口完成后，更新服务状态为正常
  - **P1/P2 告警**：直接进入下钻分析流程
    1) 执行 AI 分析
    2) 更新告警状态为恢复
    3) 记录分析结果到评论

> 说明：本阶段实现故障域识别和治愈方案查询，真实回滚接口与鉴权可后续接入 `internal/service_manager` 的部署 API。

——

## 2. 输入消息（与 healthcheck 对齐）

```go
// healthcheck/types.go
// 由 healthcheck 投递到 channel
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

- 故障域识别：从 `Labels` 中提取 `service_name` 和 `version` 信息
- deployID 的来源（用于构造回滚 URL）：
  - 可从 `Labels["deploy_id"]`（若存在）读取
  - 若为空，可按 `{service}:{version}` 组装一个占位 ID

——

## 3. 运行方式与配置

- 进程内消费者：
  - 在 `cmd/zeroops/main.go` 中创建 `make(chan AlertMessage, N)` 并同时传给 `healthcheck` 与 `remediation`，形成发布-订阅。
  - 当前 README 仅描述，具体接线可在实现阶段加入。

- 环境变量建议：
```
# 通道容量
REMEDIATION_ALERT_CHAN_SIZE=1024

# 回滚接口（Mock）
REMEDIATION_ROLLBACK_URL=http://localhost:8080/v1/deployments/%s/rollback
REMEDIATION_ROLLBACK_SLEEP=30s

# DB/Redis 复用已有：DB_* 与 REDIS_*
```

——

## 4. 处理流程（伪代码）

```go
func StartConsumer(ctx context.Context, ch <-chan AlertMessage, db *Database, rdb *redis.Client) {
    for {
        select {
        case <-ctx.Done():
            return
        case m := <-ch:
            switch m.Level {
            case "P0":
                // P0 告警：故障治愈流程
                handleP0Alert(ctx, m, db, rdb)
            case "P1", "P2":
                // P1/P2 告警：下钻分析流程
                handleP1P2Alert(ctx, m, db, rdb)
            default:
                log.Printf("Unknown alert level: %s", m.Level)
            }
        }
    }
}

// P0 告警处理：故障治愈流程
func handleP0Alert(ctx context.Context, m AlertMessage, db *Database, rdb *redis.Client) {
    // 1) 确认故障域
    faultDomain := identifyFaultDomain(m.Labels)
    
    // 2) 查询治愈方案
    healAction, err := queryHealAction(ctx, db, faultDomain)
    if err != nil {
        log.Printf("Failed to query heal action: %v", err)
        // 治愈方案查询失败，直接进入下钻分析
        handleDrillDownAnalysis(ctx, m, db, rdb)
        return
    }
    
    // 3) 执行治愈操作
    success := executeHealAction(ctx, healAction, m)
    if !success {
        log.Printf("Heal action failed for alert %s", m.ID)
        // 治愈操作失败，直接进入下钻分析
        handleDrillDownAnalysis(ctx, m, db, rdb)
        return
    }
    
    // 4) 治愈成功后启动观察窗口，延迟状态更新
    handleDrillDownAnalysisWithObservation(ctx, m, db, rdb)
}

// P1/P2 告警处理：下钻分析流程
func handleP1P2Alert(ctx context.Context, m AlertMessage, db *Database, rdb *redis.Client) {
    handleDrillDownAnalysis(ctx, m, db, rdb)
}

// 故障域识别
func identifyFaultDomain(labels map[string]string) string {
    service := labels["service_name"]
    version := labels["version"]
    
    if service != "" && version != "" {
        return "service_version_issue"
    }
    
    // 可根据更多条件扩展其他故障域
    return "unknown"
}

// 查询治愈方案
func queryHealAction(ctx context.Context, db *Database, faultDomain string) (*HealAction, error) {
    const q = `SELECT id, desc, type, rules FROM heal_actions WHERE type = $1 LIMIT 1`
    // 实现查询逻辑
    return nil, nil
}

// 执行治愈操作
func executeHealAction(ctx context.Context, action *HealAction, m AlertMessage) bool {
    // 根据 action.rules 中的条件执行相应操作
    // 当前仅支持回滚操作
    if action.Rules["action"] == "rollback" {
        return executeRollback(ctx, m)
    } else if action.Rules["action"] == "alert" {
        log.Printf("Alert: %s", action.Rules["message"])
        return false
    }
    return false
}

// 执行回滚操作
func executeRollback(ctx context.Context, m AlertMessage) bool {
    deployID := m.Labels["deploy_id"]
    if deployID == "" {
        deployID = fmt.Sprintf("%s:%s", m.Service, m.Version)
    }
    
    // Mock 回滚：sleep 指定时间
    sleep(os.Getenv("REMEDIATION_ROLLBACK_SLEEP"), 30*time.Second)
    // TODO: 真实 HTTP 调用回滚接口
    
    return true
}

// 下钻分析处理（P1/P2 告警直接使用）
func handleDrillDownAnalysis(ctx context.Context, m AlertMessage, db *Database, rdb *redis.Client) {
    // 1) 执行 AI 分析
    _ = addAIAnalysisComment(ctx, db, m)
    
    // 2) 更新告警状态为恢复
    _ = markRestoredInDB(ctx, db, m)
    
    // 3) 更新缓存状态
    _ = markRestoredInCache(ctx, rdb, m)
}

// 下钻分析处理（P0 告警治愈后使用，延迟状态更新）
func handleDrillDownAnalysisWithObservation(ctx context.Context, m AlertMessage, db *Database, rdb *redis.Client) {
    // 1) 执行 AI 分析
    _ = addAIAnalysisComment(ctx, db, m)
    
    // 2) 记录治愈完成评论，但不更新告警状态
    _ = addHealingCompletedComment(ctx, db, m)
    
    // 3) 启动观察窗口，等待30分钟
    _ = startObservationWindow(ctx, m.Service, m.Version, m.ID, 30*time.Minute)
    
    // 注意：此时不更新 alert_issues.alert_state 和 service_states.health_state
    // 状态更新将在观察窗口完成后进行
}

// 观察窗口完成后的处理
func completeObservationWindow(ctx context.Context, service, version string, db *Database, rdb *redis.Client) {
    // 1) 完成观察窗口
    _ = completeObservation(ctx, service, version)
    
    // 2) 更新 alert_issues.alert_state 为 'Restored'
    // 3) 更新 service_states.health_state 为 'Normal'
    // 4) 更新相关缓存
    _ = markServiceAsNormal(ctx, service, version, db, rdb)
    
    log.Printf("Observation window completed for service %s version %s, status updated to Normal", service, version)
}
```

——

## 5. 故障域识别与治愈方案

### 故障域类型

当前支持的故障域类型：

1. **service_version_issue**：服务版本问题
   - 识别条件：`labels["service_name"]` 和 `labels["version"]` 都存在
   - 治愈方案：
     - 发布中版本：执行回滚操作
     - 已完成发布版本：提示暂不支持自动回滚

2. **unknown**：未知故障域
   - 识别条件：无法从标签中识别出已知故障域
   - 处理方式：跳过治愈，直接进入下钻分析

### 治愈方案规则

`heal_actions.rules` 字段的 JSON 结构：

```json
{
  "deployment_status": "deploying|deployed",
  "action": "rollback|alert",
  "target": "previous_version",
  "message": "版本已发布，暂不支持自动回滚"
}
```

### 治愈操作类型

1. **rollback**：执行回滚操作
   - 调用部署系统的回滚接口
   - 回滚到上一个稳定版本

2. **alert**：仅告警，不执行自动操作
   - 记录告警信息
   - 需要人工介入处理

### 扩展性设计

- 故障域类型可扩展：整体问题、单机房问题、网络问题等
- 治愈方案可扩展：重启服务、扩容、切换流量等
- 规则条件可扩展：基于更多标签和指标进行判断

#### 添加新的故障域类型

1. 在 `types.go` 中添加新的 `FaultDomain` 常量
2. 在 `IdentifyFaultDomain` 方法中添加识别逻辑
3. 在数据库中配置对应的治愈方案

#### 添加新的治愈操作类型

1. 在 `HealActionRules` 结构体中添加新字段
2. 在 `ExecuteHealAction` 方法中添加新的 case 分支
3. 实现具体的治愈操作逻辑

### 观察窗口机制

观察窗口是治愈操作完成后的验证期，用于确保治愈操作的有效性：

1. **启动条件**：P0 告警治愈操作成功完成后自动启动
2. **持续时间**：默认30分钟，可配置
3. **监控内容**：观察该服务是否在窗口期内出现新的告警
4. **处理逻辑**：
   - 如果窗口期内出现新告警：取消观察窗口，重新进入治愈流程
   - 如果窗口期内无新告警：完成观察窗口，更新服务状态为正常
5. **状态更新时机**：
   - **治愈操作完成后**：不立即更新状态，只记录治愈完成评论
   - **观察窗口完成后**：同时更新 `alert_issues.alert_state` 为 `Restored` 和 `service_states.health_state` 为 `Normal`
6. **关键原则**：每次修改 `service_states.health_state` 为 `Normal` 时，都必须同时修改 `alert_issues.alert_state` 为 `Restored`

——

## 6. 代码使用示例

### 数据库初始化

```bash
# 执行初始化脚本
psql -U postgres -d zeroops -f init_heal_actions.sql
```

### 代码使用

```go
// 创建服务
healDAO := NewPgHealActionDAO(db)
healService := NewHealActionService(healDAO)

// 识别故障域
faultDomain := healService.IdentifyFaultDomain(labels)

// 获取治愈方案
healAction, err := healService.GetHealAction(ctx, faultDomain)

// 执行治愈操作
result, err := healService.ExecuteHealAction(ctx, healAction, alertID, labels)
```

### 测试

运行测试：

```bash
go test ./internal/alerting/service/remediation -v
```

测试覆盖：
- 故障域识别逻辑
- 治愈操作执行
- 部署状态判断

## 7. DB 更新（SQL 建议）

- 告警状态：
```sql
UPDATE alert_issues
SET alert_state = 'Restored'
WHERE id = $1;
```

- 服务态：
```sql
UPDATE service_states
SET health_state = 'Normal',
    resolved_at = NOW()
WHERE service = $1 AND version = $2;
```

- 评论写入（AI 分析结果）（`alert_issue_comments.issue_id`对应 `alert_issues.id`）：
```sql
INSERT INTO alert_issue_comments (issue_id, create_at, content)
VALUES (
  $1,
  NOW(),
  $2
);
```

评论内容模板（Markdown，多行，内容暂未设计）：
```
## AI分析结果
**问题类型**：非发版本导致的问题
**根因分析**：数据库连接池配置不足，导致大量请求无法获取数据库连接
**处理建议**：
- 增加数据库连接池大小
- 优化数据库连接管理
- 考虑读写分离缓解压力
**执行状态**：正在处理中，等待指标恢复正常
```

> 说明：若 `service_states` 不存在对应行，可按需 `INSERT ... ON CONFLICT`；或沿用 `receiver.PgDAO.UpsertServiceState` 的写入策略。

——

## 8. 缓存更新（Redis，Lua CAS 建议）

- 告警缓存 `alert:issue:{id}`：
```lua
-- KEYS[1] = alert key
-- KEYS[2] = idx:old1 (例如 alert:index:alert_state:Pending)
-- KEYS[3] = idx:old2 (例如 alert:index:alert_state:InProcessing)
-- KEYS[4] = idx:new  (alert:index:alert_state:Restored)
-- ARGV[1] = next ('Restored'), ARGV[2] = id
local v = redis.call('GET', KEYS[1])
if not v then return 0 end
local obj = cjson.decode(v)
obj.alertState = ARGV[1]
redis.call('SET', KEYS[1], cjson.encode(obj), 'KEEPTTL')
if KEYS[2] ~= '' then redis.call('SREM', KEYS[2], ARGV[2]) end
if KEYS[3] ~= '' then redis.call('SREM', KEYS[3], ARGV[2]) end
if KEYS[4] ~= '' then redis.call('SADD', KEYS[4], ARGV[2]) end
return 1
```

- 服务态缓存 `service_state:{service}:{version}`：
```lua
-- KEYS[1] = service_state key
-- KEYS[2] = idx:new (service_state:index:health:Normal)
-- ARGV[1] = next ('Normal'), ARGV[2] = member (key 本身)
local v = redis.call('GET', KEYS[1])
if not v then v = '{}' end
local obj = cjson.decode(v)
obj.health_state = ARGV[1]
obj.resolved_at = redis.call('TIME')[1] -- 可选：秒级时间戳；或由上层填充分辨率更高的时间串
redis.call('SET', KEYS[1], cjson.encode(obj), 'KEEPTTL')
if KEYS[2] ~= '' then redis.call('SADD', KEYS[2], KEYS[1]) end
return 1
```

- 建议键：
  - `alert:index:alert_state:Pending|InProcessing|Restored`
  - `service_state:index:health:Normal|Warning|Error`

——

## 9. 幂等与重试

- 幂等：同一 `AlertMessage.ID` 的回滚处理应具备幂等性，重复消费不应产生额外副作用。
- 重试：Mock 模式下可忽略；接入真实接口后，对 5xx/网络错误考虑重试与退避，最终写入失败应有告警与补偿。

——

## 10. 验证步骤（与 healthcheck E2E 相衔接）

### 基础验证步骤

1) 启动 Redis/Postgres 与 API（参考 `healthcheck/E2E_VALIDATION.md` 与 `env_example.txt`）
2) 创建 `heal_actions` 表并插入测试数据
3) 创建 channel，并将其同时传给 `healthcheck.StartScheduler(..)` 与 `remediation.StartConsumer(..)`

### P0 告警验证（故障治愈流程）

4) 触发 P0 级别 Webhook，`alert_issues` 入库为 `Pending`
5) 等待 `healthcheck` 将缓存态切到 `InProcessing`
6) 验证故障域识别：检查日志中是否正确识别为 `service_version_issue`
7) 验证治愈方案查询：检查是否从 `heal_actions` 表查询到对应方案
8) 等待 `remediation` 执行治愈操作完成：
   - 验证观察窗口已启动（Redis 中存在观察窗口记录）
   - `alert_issue_comments` 中新增治愈完成评论
   - **重要**：验证 `alert_issues.alert_state` 仍为 `InProcessing`（未更新为 `Restored`）
   - **重要**：验证 `service_states.health_state` 未更新为 `Normal`
9) 等待观察窗口完成（30分钟后）或模拟窗口期内新告警：
   - **如果无新告警**：
     - 验证观察窗口自动完成
     - 验证状态同时更新为 `alert_issues.alert_state = 'Restored'` 和 `service_states.health_state = 'Normal'`
   - **如果有新告警**：
     - 验证观察窗口被取消
     - 验证重新进入治愈流程
     - 验证状态未更新为 `Restored`/`Normal`

### P1/P2 告警验证（下钻分析流程）

9) 触发 P1 或 P2 级别 Webhook
10) 验证直接进入下钻分析流程，跳过故障治愈步骤
11) 验证 AI 分析评论生成和状态更新

### 最终验证

12) 通过 Redis 与 API (`/v1/issues`、`/v1/issues/{id}`) 验证字段已更新
13) 验证不同告警等级的处理路径正确性

——

## 11. 注意事项

1. **service_states 表逻辑**: 当前版本中，`service_states` 表的更新逻辑暂时不实现，但保留了扩展空间
2. **Mock 模式**: 当前回滚操作为 Mock 模式，实际部署时需要接入真实的部署系统 API
3. **错误处理**: 治愈操作失败时会记录日志并继续进入下钻分析流程
4. **幂等性**: 同一告警的重复处理应该具备幂等性

## 12. 后续计划

### 短期计划

- 实现 `heal_actions` 表的完整 CRUD 操作
- 完善故障域识别逻辑，支持更多故障类型
- 接入真实部署系统回滚接口与鉴权
- 实现治愈方案的动态配置和管理界面

### 中期计划

- 扩展治愈操作类型：服务重启、扩容、流量切换等
- 增加治愈方案的执行结果反馈和效果评估
- 将进程内 channel 平滑切换为 MQ（Kafka/NATS）
- 完善指标与可观测：事件消费速率、成功率、时延分位、治愈结果等

### 长期计划

- 基于历史数据训练 AI 模型，自动推荐最优治愈方案
- 增加补偿任务：对"治愈成功但缓存/DB 未一致"的场景进行对账修复
- 实现治愈方案的 A/B 测试和效果对比
- 构建完整的故障自愈知识库和最佳实践库
