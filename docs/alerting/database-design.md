# 数据库设计 - Monitoring & Alerting Service

## 概述

本文档为最新数据库设计，总计包含 6 张表：

- alert_issues
- alert_issue_comments
- alert_meta_change_logs
- alert_rules
- alert_rule_metas
- service_states

## 数据表设计

### 1) alert_issues（告警问题表）

存储告警问题的主要信息。

| 字段名 | 类型 | 说明 |
|--------|------|------|
| id | varchar(64) PK | 告警 issue ID |
| state | enum(Closed, Open) | 问题状态 |
| level | varchar(32) | 告警等级：如 P0/P1/Px |
| alert_state | enum(Pending, Restored, AutoRestored, InProcessing) | 处理状态 |
| title | varchar(255) | 告警标题 |
| labels | json | 标签，格式：[{key, value}] |
| alert_since | TIMESTAMP(6) | 告警首次发生时间 |

**索引建议：**
- PRIMARY KEY: `id`
- INDEX: `(state, level, alert_since)`
- INDEX: `(alert_state, alert_since)`

---

### 2) alert_issue_comments（告警评论/处理记录表）

记录 AI/系统/人工在处理告警过程中的动作与备注。

| 字段名 | 类型 | 说明 |
|--------|------|------|
| issue_id | varchar(64) FK | 对应 `alert_issues.id` |
| create_at | TIMESTAMP(6) | 评论创建时间 |
| content | text | Markdown 内容 |

**索引建议：**
- PRIMARY KEY: `(issue_id, create_at)`
- FOREIGN KEY: `issue_id` REFERENCES `alert_issues(id)`

---

### 3) alert_meta_change_logs（阈值变更记录表）

用于追踪规则阈值（threshold）与观察窗口（watch_time）的变更历史。

| 字段名 | 类型 | 说明 |
|--------|------|------|
| id | varchar(64) PK | 幂等/去重标识 |
| change_type | varchar(16) | 变更类型：Create / Update / Delete / Rollback |
| change_time | timestamptz | 变更时间 |
| alert_name | varchar(255) | 规则名 |
| labels | text | labels 的 JSON 字符串表示（规范化后） |
| old_threshold | numeric | 旧阈值（可空） |
| new_threshold | numeric | 新阈值（可空） |
| old_watch | interval | 旧观察窗口（可空） |
| new_watch | interval | 新观察窗口（可空） |


**索引建议：**
- PRIMARY KEY: `id`
- INDEX: `(change_time)`
- INDEX: `(alert_name, change_time)`

---

### 4) alert_rules（告警规则表）

| 字段名 | 类型 | 说明 |
|--------|------|------|
| name | varchar(255) | 主键，告警规则名称 |
| description | text | 可读标题，可拼接渲染为可读的 title |
| expr | text | 左侧业务指标表达式，（通常对应 PromQL 左侧的聚合，如 sum(apitime) by (service, version)） |
| op | varchar(4) | 阈值比较方式（枚举：>, <, =, !=） |
| severity | varchar(32) | 告警等级，通常进入告警的 labels.severity |

**约束建议：**
- CHECK 约束：`op IN ('>', '<', '=', '!=')`

⸻

### 5) alert_rule_metas（规则阈值元信息表）

| 字段名 | 类型 | 说明 |
|--------|------|------|
| alert_name | varchar(255) | 关联 `alert_rules.name` |
| labels | jsonb | 适用标签（示例：{"service":"s3","version":"v1"}）；为空 `{}` 表示全局 |
| threshold | numeric | 阈值（会被渲染成特定规则的 threshold metric 数值） |
| watch_time | interval | 持续时长（映射 Prometheus rule 的 for:） |

**约束与索引建议：**
- FOREIGN KEY: `(alert_name)` REFERENCES `alert_rules(name)` ON DELETE CASCADE
- UNIQUE: `(alert_name, labels)`
- GIN INDEX: `labels`（`CREATE INDEX idx_metas_labels_gin ON alert_rule_metas USING gin(labels);`）

⸻

说明：
- labels 建议用 jsonb，方便在 Postgres 中做索引和查询。
- labels 的键名与值格式应在应用层规范化（排序/小写/去空值）以确保唯一性和可查询性一致。

---

### 7) service_states（服务状态表）

追踪服务在某一版本上的健康状态与处置进度。

| 字段名 | 类型 | 说明 |
|--------|------|------|
| service | varchar(255) PK | 服务名 |
| version | varchar(255) PK | 版本号 |
| report_at | TIMESTAMP(6) | 同步alert_issue_ids中，alert_issue中alert_state=InProcessing状态的alert_since的最早时间 |
| resolved_at | TIMESTAMP(6) | 解决时间（可空） |
| health_state | enum(Normal,Warning,Error) | 处置阶段 |
| alert_issue_ids | [] alert_issue_id | 关联alert_issues表的id |

**索引建议：**
- PRIMARY KEY: `(service, version)`

## 数据关系（ER）

```mermaid
erDiagram
    alert_issues ||--o{ alert_issue_comments : "has comments"

    alert_rules {
        varchar name PK
        text description
        text expr
        varchar op
        varchar severity
    }

    alert_rule_metas {
        varchar alert_name FK
        jsonb labels
        numeric threshold
        interval watch_time
    }

    service_states {
        varchar service PK
        varchar version PK
        <!-- enum level -->
        text detail
        timestamp report_at
        timestamp resolved_at
        varchar health_state
        varchar correlation_id
    }

    alert_issues {
        varchar id PK
        enum state
        varchar level
        enum alert_state
        varchar title
        json labels
        timestamp alert_since
    }

    alert_issue_comments {
        varchar issue_id FK
        timestamp create_at
        text content
    }

    %% 通过 service 等标签在应用层逻辑关联
    alert_rule_metas ||..|| alert_rules : "by alert_name"
    service_states ||..|| alert_rule_metas : "by service/version labels"
```

## 数据流转

1. 以 `alert_rules` 为模版，结合 `alert_rule_metas` 渲染出面向具体服务/版本等的规则（labels 可为空 `{}` 表示全局默认，或包含如 service/version 等标签）。
2. 指标或规则参数发生调整时，记录到 `alert_meta_change_logs`。
3. 规则触发创建 `alert_issues`；处理过程中的动作写入 `alert_issue_comments`。