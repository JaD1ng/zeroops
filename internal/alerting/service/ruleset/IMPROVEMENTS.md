# PostgreSQL Interval 解析改进

## 问题描述

原始的 `timeParseDurationPG` 函数存在以下问题：

1. **忽略 `fmt.Sscanf` 的错误**：这可能导致在解析格式不正确的输入时，函数静默失败或返回错误的值
2. **支持的 PostgreSQL `interval` 格式有限**：只支持 "01:02:03", "02:03", "3600 seconds" 等少数格式
3. **缺乏健壮性**：没有适当的错误处理和输入验证

## 改进方案

### 方案一：改进现有函数（已实现）

我们大幅改进了 `timeParseDurationPG` 函数，使其支持更多格式并具有更好的错误处理：

#### 支持的格式

1. **时间格式**：
   - `"01:02:03"` - 小时:分钟:秒
   - `"02:03"` - 分钟:秒
   - `"01:02:03.456"` - 带小数秒的时间格式

2. **秒数格式**：
   - `"3600 seconds"` - 整数秒
   - `"1.5 seconds"` - 小数秒

3. **ISO 8601 格式**：
   - `"PT1H2M3S"` - 组合格式
   - `"PT1H"` - 仅小时
   - `"PT2M"` - 仅分钟
   - `"PT3S"` - 仅秒
   - `"PT1.5S"` - 小数秒

4. **SQL 标准格式**：
   - `"1h"` - 小时
   - `"2m"` - 分钟
   - `"3s"` - 秒

5. **PostgreSQL 格式**：
   - `"1d"` - 天
   - `"2h"` - 小时
   - `"3m"` - 分钟
   - `"4s"` - 秒

#### 改进的错误处理

- 检查并处理 `fmt.Sscanf` 返回的错误
- 验证时间值的合理性（如分钟和秒数不能超过 60）
- 检查负值
- 提供清晰的错误消息

#### 测试覆盖

创建了全面的单元测试，覆盖：
- 所有支持的格式
- 错误情况
- 边界条件
- 辅助函数

### 方案二：使用 pgx 原生类型（推荐）

我们创建了一个改进的存储实现 `PgStoreImproved`，使用 pgx 的原生类型系统：

#### 优势

1. **类型安全**：使用 `pgtype.Interval` 避免手动解析
2. **自动转换**：pgx 驱动程序自动处理 PostgreSQL 类型转换
3. **更好的错误处理**：明确的错误信息，特别是对于包含月份的间隔
4. **性能更好**：避免了字符串解析的开销

#### 实现细节

```go
// 转换 time.Duration 到 pgtype.Interval
func durationToPgInterval(d time.Duration) pgtype.Interval {
    totalMicroseconds := d.Microseconds()
    days := totalMicroseconds / (24 * 60 * 60 * 1000000)
    remainingMicroseconds := totalMicroseconds % (24 * 60 * 60 * 1000000)
    
    return pgtype.Interval{
        Microseconds: remainingMicroseconds,
        Days:         int32(days),
        Months:       0,
        Valid:        true,
    }
}

// 转换 pgtype.Interval 到 time.Duration
func pgIntervalToDuration(interval pgtype.Interval) (time.Duration, error) {
    if !interval.Valid {
        return 0, fmt.Errorf("interval is not valid")
    }
    
    if interval.Months != 0 {
        return 0, fmt.Errorf("cannot convert interval with months to duration: %d months", interval.Months)
    }
    
    totalMicroseconds := interval.Microseconds + int64(interval.Days)*24*60*60*1000000
    return time.Duration(totalMicroseconds) * time.Microsecond, nil
}
```

## 使用建议

### 当前实现

`PgStore` 现在使用 pgx 原生类型系统来处理 PostgreSQL interval 类型，提供了：

1. **类型安全**：使用 `pgtype.Interval` 避免手动解析
2. **清晰的错误处理**：明确的错误信息，特别是对于包含月份的间隔
3. **更好的性能**：避免了字符串解析的开销
4. **与 PostgreSQL 类型系统的更好集成**：直接使用 pgx 的原生类型系统

## 迁移指南

改进已经集成到 `PgStore` 中，无需额外的迁移步骤：

1. **使用现有接口**：
   ```go
   store := NewPgStore(db)
   ```

2. **确保依赖**：
   项目已经导入了 `github.com/jackc/pgx/v5/pgtype`

3. **测试验证**：
   所有测试都已通过，确保功能正常

## 注意事项

1. **月份处理**：PostgreSQL 的 `interval` 类型可以包含月份，但 `time.Duration` 不能。改进的实现会拒绝包含月份的间隔。

2. **精度**：转换过程中可能涉及精度损失，特别是在处理非常大的时间间隔时。

3. **向后兼容性**：改进的实现保持了相同的接口，确保向后兼容。

## 测试结果

所有测试都通过，包括：
- 原始函数的改进版本测试
- pgx 类型转换测试
- 往返转换测试
- 错误处理测试

这确保了改进的健壮性和可靠性。
