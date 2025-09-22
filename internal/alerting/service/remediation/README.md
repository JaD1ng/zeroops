# remediation — 通道消费与自动回滚（Mock）

本包规划一个后台处理器：消费 `healthcheck` 投递到进程内 channel 的告警消息，模拟执行“自动回滚”，回滚成功后将相关告警与服务态标记为恢复。

——

## 1. 目标

- 订阅 `healthcheck` 的 `AlertMessage`（进程内 channel）
- 对每条消息：
  1) Mock 调用回滚接口 `POST /v1/deployments/:deployID/rollback`
  2) `sleep 30s` 后返回“回滚成功”的模拟响应
  3) 若成功，则更新 DB 与缓存：
     - `alert_issues.alert_state = 'Restored'`
     - `alert_issues.state = 'Closed'`
     - `service_states.health_state = 'Normal'`
     - `service_states.resolved_at = NOW()`（当前时间）
     - 同时在 `alert_issue_comments` 中追加一条 AI 分析评论（见下文内容模板）

> 说明：本阶段仅实现消费与 Mock，真实回滚接口与鉴权可后续接入 `internal/service_manager` 的部署 API。

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

- deployID 的来源（用于构造回滚 URL）：
  - Mock 阶段：可从 `Labels["deploy_id"]`（若存在）读取；若为空，可按 `{service}:{version}` 组装一个占位 ID。

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

## 4. 流程（伪代码）

```go
func StartConsumer(ctx context.Context, ch <-chan AlertMessage, db *Database, rdb *redis.Client) {
    for {
        select {
        case <-ctx.Done():
            return
        case m := <-ch:
            // 1) 组装回滚 URL（Mock）
            deployID := m.Labels["deploy_id"]
            if deployID == "" {
                // 仅 Mock：用 service:version 兜底
                deployID = fmt.Sprintf("%s:%s", m.Service, m.Version)
            }
            url := fmt.Sprintf(os.Getenv("REMEDIATION_ROLLBACK_URL"), deployID)

            // 2) 发起回滚（Mock）：sleep 指定时间再判为成功
            sleep(os.Getenv("REMEDIATION_ROLLBACK_SLEEP"), 30*time.Second)
            // TODO: 如需真实 HTTP 调用，可在此发起 POST 并根据响应判断

            // 3) 成功后，先写入 AI 分析评论，再更新 DB 与缓存状态
            _ = addAIAnalysisComment(ctx, db, m)
            _ = markRestoredInDB(ctx, db, m)
            _ = markRestoredInCache(ctx, rdb, m)
        }
    }
}
```

——

## 5. DB 更新（SQL 建议）

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

评论内容模板（Markdown，多行）：
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

## 6. 缓存更新（Redis，Lua CAS 建议）

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

## 7. 幂等与重试

- 幂等：同一 `AlertMessage.ID` 的回滚处理应具备幂等性，重复消费不应产生额外副作用。
- 重试：Mock 模式下可忽略；接入真实接口后，对 5xx/网络错误考虑重试与退避，最终写入失败应有告警与补偿。

——

## 8. 验证步骤（与 healthcheck E2E 相衔接）

1) 启动 Redis/Postgres 与 API（参考 `healthcheck/E2E_VALIDATION.md` 与 `env_example.txt`）
2) 创建 channel，并将其同时传给 `healthcheck.StartScheduler(..)` 与 `remediation.StartConsumer(..)`
3) `curl` 触发 Webhook，`alert_issues` 入库为 `Pending`
4) 等待 `healthcheck` 将缓存态切到 `InProcessing`
5) 等待 `remediation` mock 回滚完成 → DB 与缓存更新：
   - `alert_issues.alert_state = 'Restored'`
   - `service_states.health_state = 'Normal'`
   - `service_states.resolved_at = NOW()`
6) 通过 Redis 与 API (`/v1/issues`、`/v1/issues/{id}`) 验证字段已更新（comments 仍为 mock）

——

## 9. 后续计划

- 接入真实部署系统回滚接口与鉴权
- 将进程内 channel 平滑切换为 MQ（Kafka/NATS）
- 完善指标与可观测：事件消费速率、成功率、时延分位、回滚结果等
- 增加补偿任务：对“回滚成功但缓存/DB 未一致”的场景进行对账修复
