# 执行顺序设计原则

## 问题背景

在告警规则管理中，需要同时维护两个数据源：
1. **Prometheus**：实际执行告警规则的地方
2. **数据库**：持久化存储规则配置

当这两个操作可能失败时，执行顺序的选择会影响系统的健壮性和数据一致性。

## 设计原则

### 优先保证 Prometheus 数据正确性

**原则**：确保 Prometheus 中的数据始终是正确的，即使数据库操作失败。

### 原因分析

1. **告警缺失 vs 重复告警**：
   - 告警缺失：可能导致严重问题未被及时发现
   - 重复告警：相对较轻，可以通过去重机制处理

2. **数据一致性**：
   - Prometheus 中的数据直接影响告警行为
   - 数据库中的数据主要用于配置管理和审计

## 实现方案

### 添加规则 (AddAlertRule)

```go
func (m *Manager) AddAlertRule(ctx context.Context, r *AlertRule) error {
    // 1. 先确保规则成功添加到 Prometheus
    if err := m.prom.AddToPrometheus(ctx, r); err != nil {
        return fmt.Errorf("failed to add rule to Prometheus: %w", err)
    }
    
    // 2. 然后持久化到数据库
    if err := m.store.CreateRule(ctx, r); err != nil {
        return fmt.Errorf("failed to create rule in database: %w", err)
    }
    
    return nil
}
```

**优势**：
- 如果 Prometheus 添加失败，整个操作失败，不会产生不一致状态
- 如果数据库写入失败，规则仍在 Prometheus 中，告警功能正常
- 重复执行时，Prometheus 会处理重复规则（去重或报错）

### 删除规则 (DeleteAlertRule)

```go
func (m *Manager) DeleteAlertRule(ctx context.Context, name string) error {
    // 1. 先从 Prometheus 中移除，立即停止告警
    if err := m.prom.DeleteFromPrometheus(ctx, name); err != nil {
        return fmt.Errorf("failed to delete rule from Prometheus: %w", err)
    }
    
    // 2. 然后从数据库中删除
    if err := m.store.DeleteRule(ctx, name); err != nil {
        return fmt.Errorf("failed to delete rule from database: %w", err)
    }
    
    return nil
}
```

**优势**：
- 如果 Prometheus 删除失败，整个操作失败，规则继续生效
- 如果数据库删除失败，规则已从 Prometheus 中移除，不会产生误报

### 更新规则元数据 (UpsertRuleMetas)

```go
func (m *Manager) UpsertRuleMetas(ctx context.Context, meta *AlertRuleMeta) error {
    // 1. 先获取旧数据用于变更日志
    oldList, err := m.store.GetMetas(ctx, meta.AlertName, meta.Labels)
    // ...
    
    // 2. 先同步到 Prometheus，确保阈值数据正确
    if err := m.prom.SyncMetaToPrometheus(ctx, meta); err != nil {
        return fmt.Errorf("failed to sync meta to Prometheus: %w", err)
    }
    
    // 3. 然后在事务中更新数据库和记录变更日志
    return m.store.WithTx(ctx, func(tx Store) error {
        _, err = tx.UpsertMeta(ctx, meta)
        if err != nil {
            return err
        }
        if err := m.RecordMetaChangeLog(ctx, old, meta); err != nil {
            return err
        }
        return nil
    })
}
```

**优势**：
- 如果 Prometheus 同步失败，整个操作失败，不会产生不一致状态
- 如果数据库更新失败，阈值数据仍在 Prometheus 中正确，告警功能正常
- 变更日志记录在数据库事务中，保证审计数据的完整性
- **重要**：Prometheus 操作不在事务中，避免长时间持有数据库锁
- **性能优化**：所有耗时操作（时间生成、字符串处理、逻辑判断）都在事务外完成

## 事务优化原则

### 最小化事务锁时间

**原则**：在事务中只执行必要的数据库操作，所有耗时操作都在事务外完成。

### 优化策略

1. **参数准备**：
   ```go
   // 在事务外准备所有参数
   changeLog := m.prepareChangeLog(old, meta)
   
   // 事务中只执行数据库操作
   return m.store.WithTx(ctx, func(tx Store) error {
       _, err = tx.UpsertMeta(ctx, meta)
       if err != nil {
           return err
       }
       if changeLog != nil {
           return tx.InsertChangeLog(ctx, changeLog)
       }
       return nil
   })
   ```

2. **避免在事务中的耗时操作**：
   - ❌ `time.Now()` - 系统调用
   - ❌ `fmt.Sprintf()` - 字符串格式化
   - ❌ 复杂逻辑判断
   - ❌ 外部服务调用（如 Prometheus）
   - ✅ 简单的数据库 CRUD 操作

3. **参数预计算**：
   ```go
   // 事务外完成所有计算
   now := time.Now()
   changeTime := now.UTC()
   id := fmt.Sprintf("%s-%s-%d", alertName, labelKey, now.UnixNano())
   changeType := classifyChange(oldMeta, newMeta)
   ```

### 性能影响

- **锁时间减少**：从毫秒级减少到微秒级
- **并发性能提升**：减少锁竞争
- **系统稳定性**：避免长时间事务导致的死锁

## 错误处理策略

### 重复执行场景

1. **添加规则重复执行**：
   - Prometheus 通常有去重机制
   - 如果没有去重，会报错"规则已存在"，可以认为是成功状态
   - 数据库的 `ON CONFLICT` 处理重复插入

2. **删除规则重复执行**：
   - Prometheus 删除不存在的规则通常不会报错
   - 数据库删除不存在的记录通常不会报错

### 部分失败恢复

1. **Prometheus 成功，数据库失败**：
   - 规则在 Prometheus 中生效，告警功能正常
   - 可以通过重新执行操作来同步数据库
   - 系统功能不受影响

2. **Prometheus 失败，数据库成功**：
   - 规则不在 Prometheus 中，告警功能缺失
   - 需要手动修复或重新执行操作
   - 系统功能受影响

## 总结

这种执行顺序设计确保了：

1. **高可用性**：告警功能优先保证
2. **数据一致性**：Prometheus 数据始终正确
3. **可恢复性**：部分失败场景可以自动或手动恢复
4. **用户体验**：减少因数据不一致导致的告警问题

这是一个以业务连续性为优先考虑的设计选择。
