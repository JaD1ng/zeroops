## Ruleset（规则与阈值管理）

本目录为“规则与阈值管理（ruleset）”实现说明。内容聚焦于：表结构、核心接口、编排流程、Prometheus 同步方式、并发与一致性、测试与使用示例。文档与当前代码实现保持一致。

---

## 1) 目标与边界（已实现）

- 通过 `alert_rules` 与 `alert_rule_metas`，为同一告警规则按标签维度（如 `service`、`version`）配置阈值与持续时间（`watch_time`）。
- 变更阈值后，立刻同步到内存 Exporter（无需 Prometheus reload）。
- 多告警等级（P0/P1…）通过“多条规则”实现（如 `latency_p95_P0` 与 `latency_p95_P1`）。
- 记录变更日志，支持审计，便于后续扩展回滚能力。

---

## 2) Go 组件与接口

### 2.1 关键类型与接口（节选）

```go
// types.go
type AlertRule struct {
    Name        string
    Description string
    Expr        string
    Op          string
    Severity    string
}

type LabelMap map[string]string

type AlertRuleMeta struct {
    AlertName string
    Labels    LabelMap
    Threshold float64
    WatchTime time.Duration // interval 映射
}

type ChangeLog struct {
    ID           string
    AlertName    string
    ChangeType   string
    Labels       LabelMap
    OldThreshold *float64
    NewThreshold *float64
    OldWatch     *time.Duration
    NewWatch     *time.Duration
    ChangeTime   time.Time
}

type Store interface {
    // rules
    CreateRule(ctx context.Context, r *AlertRule) error
    GetRule(ctx context.Context, name string) (*AlertRule, error)
    UpdateRule(ctx context.Context, r *AlertRule) error
    DeleteRule(ctx context.Context, name string) error

    // metas (UPSERT by alert_name + labels)
    UpsertMeta(ctx context.Context, m *AlertRuleMeta) (created bool, err error)
    GetMetas(ctx context.Context, name string, labels LabelMap) ([]*AlertRuleMeta, error)
    DeleteMeta(ctx context.Context, name string, labels LabelMap) error

    // change logs
    InsertChangeLog(ctx context.Context, log *ChangeLog) error

    // tx helpers
    WithTx(ctx context.Context, fn func(Store) error) error
}

type PromSync interface {
    AddToPrometheus(ctx context.Context, r *AlertRule) error         // 新增时更新 rule 文件并 reload（当前实现为占位）
    DeleteFromPrometheus(ctx context.Context, name string) error     // 删除（当前实现为占位）
    SyncMetaToPrometheus(ctx context.Context, m *AlertRuleMeta) error
}

type AlertRuleMgr interface {
    LoadRule(ctx context.Context) error
    UpsertRuleMetas(ctx context.Context, m *AlertRuleMeta) error
    AddAlertRule(ctx context.Context, r *AlertRule) error
    DeleteAlertRule(ctx context.Context, name string) error

    AddToPrometheus(ctx context.Context, r *AlertRule) error
    DeleteFromPrometheus(ctx context.Context, name string) error
    SyncMetaToPrometheus(ctx context.Context, m *AlertRuleMeta) error
    RecordMetaChangeLog(ctx context.Context, oldMeta, newMeta *AlertRuleMeta) error
}
```

### 2.2 Manager 核心流程（与实现一致）

```go
func (m *Manager) UpsertRuleMetas(ctx context.Context, meta *AlertRuleMeta) error {
    meta.Labels = NormalizeLabels(meta.Labels, m.aliasMap)
    if err := validateMeta(meta); err != nil { return err }
    return m.store.WithTx(ctx, func(tx Store) error {
        oldList, err := tx.GetMetas(ctx, meta.AlertName, meta.Labels)
        if err != nil { return err }
        var old *AlertRuleMeta
        if len(oldList) > 0 { old = oldList[0] }
        if _, err := tx.UpsertMeta(ctx, meta); err != nil { return err }
        if err := m.RecordMetaChangeLog(ctx, old, meta); err != nil { return err }
        return m.prom.SyncMetaToPrometheus(ctx, meta)
    })
}
```

---

## 3) Prometheus 同步

- 实现为内存版 Exporter（`ExporterSync`），维护 `(rule + 规范化 labels) → {threshold, watch_time}`。
- `SyncMetaToPrometheus` 直接更新内存映射，变更即时生效。
- `AddToPrometheus`/`DeleteFromPrometheus` 作为占位，当前不写规则文件。
- 如需以 metrics 暴露阈值，可在同进程 `/metrics` 将 `ExporterSync` 中的映射导出（按规则维度命名指标）。

---

## 4) 事务、并发与一致性

- `Store.WithTx`：当前 PgStore 直接调用 fn（占位），可按需扩展为真正事务。
- 写入采用单条 UPSERT（见下文 SQL），满足幂等。
- 如存在同一 `(alert_name, labels)` 的高并发写入，建议使用 Postgres advisory lock。
- Exporter 同步在 Upsert 成功后执行，生产中建议串行化该步骤以避免竞态。

提示：
- 标签命名不一致（例如 `service_version`/`version` 混用）通过 `NormalizeLabels` 的别名映射解决。
- 多层阈值优先级（`{}`, `{service}`, `{service,version}`）建议仅导出“最具体”的一条（当前实现未裁剪，可扩展）。

---

## 5) SQL 示例（与代码一致）

### 5.1 UPSERT Meta（带审计在应用层做）

```sql
-- 假设参数：$1 alert_name, $2 labels::jsonb, $3 threshold::numeric, $4 watch::interval
INSERT INTO alert_rule_metas(alert_name, labels, threshold, watch_time)
VALUES ($1, $2, $3, $4)
ON CONFLICT (alert_name, labels) DO UPDATE SET
  threshold  = EXCLUDED.threshold,
  watch_time = EXCLUDED.watch_time,
  updated_at = now();
```

### 5.2 查询：按部分标签匹配

```sql
-- 传入 {"service":"stg"}，返回该规则下 service=stg 的 metas（无视 version）
SELECT * FROM alert_rule_metas
WHERE alert_name = $1
  AND labels @> $2::jsonb;   -- 包含关系
```

---

## 6) 使用示例（最小化）

```go
db, _ := database.New(os.Getenv("ALERTING_PG_DSN"))
store := ruleset.NewPgStore(db)
prom  := ruleset.NewExporterSync()
mgr   := ruleset.NewManager(store, prom, map[string]string{"service_version":"version"})

meta := &ruleset.AlertRuleMeta{
    AlertName: "latency_p95_P0",
    Labels:    ruleset.LabelMap{"Service": "s3", "service_version": "v1"},
    Threshold: 450,
    WatchTime: 2 * time.Minute,
}
_ = mgr.UpsertRuleMetas(context.Background(), meta)
```

---

## 7) Exporter 要点（当前实现）

- 使用 `CanonicalLabelKey` 生成稳定键。
- 当前未实现“优先级裁剪”（`{}`, `{service}`, `{service,version}` 仅导出最具体），可按需扩展。
- 多副本部署需共享或拉取状态（可由 DB 拉取或事件广播）。

---

## 8) 测试

### 8.1 单元测试

- NormalizeLabels 与 CanonicalLabelKey：
  - 输入包含大小写、空白、别名键（如 `service_version`）的 labels，断言小写化、去空白、别名映射、移除空值；
  - 对乱序键，`CanonicalLabelKey` 结果一致。
- Manager.UpsertRuleMetas：
  - 使用内存实现的 Store 与 ExporterSync：
    - 首次 Upsert 走 Create 分支，写入 metas，并同步到 ExporterSync；
    - 再次 Upsert 走 Update 分支，产生变更日志；
    - 断言阈值已生效到 ExporterSync。

对应测试用例：`internal/alerting/service/ruleset/normalize_test.go`，`internal/alerting/service/ruleset/manager_test.go`

运行：

```bash
go test ./internal/alerting/service/ruleset -v
```

### 8.2 手动测试（本地）

- 数据库准备：
  - 按本文数据库设计创建表（或参考 `docs/alerting/database-design.md`）。
  - 在 `alert_rules` 插入一条规则，如：`latency_p95_P0`。
- 启动 Exporter/服务：
  - 代码中使用 `NewExporterSync()` 并注入到 `NewManager(...)`。
  - 通过 `Manager.UpsertRuleMetas` 传入 `AlertRuleMeta{AlertName:"latency_p95_P0", Labels:{service:"s3",version:"v1"}, Threshold:450, WatchTime:2m}`。
  - 验证内存 Exporter `ForTestingGet` 返回阈值为 450。
- 变更验证：
  - 再次调用 `UpsertRuleMetas`，阈值改为 500，检查 `alert_meta_change_logs` 新增 Update 记录。
- 回滚演练：
  - 读取上一条变更日志的 old 值，再次 Upsert 即可实现回滚（可后续补充接口）。

---

## 9) 需求映射

- 同一规则多阈值等级（P0/P1）→ 通过多条 `alert_rules`（如 `_P0` 与 `_P1`）。
- 告警变更接口（service + meta 参数）→ 统一落在 `alert_rule_metas`（已支持 labels 任意组合）。
- 变更记录查询 → `alert_meta_change_logs`。

---

## 10) 后续增强（建议）

1. Exporter 端优先级裁剪（仅导出最具体标签的阈值）。
2. PgStore 接入真实事务（BeginTx），必要时使用 advisory lock。
3. 增加回滚接口：基于 change_log 的 old 值再 Upsert 一次。
4. 阈值 metrics 暴露：统一命名（每条规则单独 threshold metric）。

---