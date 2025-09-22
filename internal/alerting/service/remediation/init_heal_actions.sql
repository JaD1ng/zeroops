-- 创建 heal_actions 表
CREATE TABLE IF NOT EXISTS heal_actions (
    id VARCHAR(255) PRIMARY KEY,
    desc TEXT NOT NULL,
    type VARCHAR(255) NOT NULL,
    rules JSONB NOT NULL
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_heal_actions_type ON heal_actions(type);

-- 插入示例数据
INSERT INTO heal_actions (id, desc, type, rules) VALUES 
(
    'service_version_rollback_deploying',
    '服务版本回滚方案（发布中版本）',
    'service_version_issue',
    '{"deployment_status": "deploying", "action": "rollback", "target": "previous_version"}'
),
(
    'service_version_alert_deployed',
    '服务版本告警方案（已完成发布版本）',
    'service_version_issue',
    '{"deployment_status": "deployed", "action": "alert", "message": "版本已发布，暂不支持自动回滚，需要人工介入处理"}'
),
(
    'service_version_rollback_default',
    '服务版本回滚方案（默认）',
    'service_version_issue',
    '{"action": "rollback", "target": "previous_version"}'
)
ON CONFLICT (id) DO UPDATE SET
    desc = EXCLUDED.desc,
    type = EXCLUDED.type,
    rules = EXCLUDED.rules;

-- 查询验证
SELECT id, desc, type, rules FROM heal_actions ORDER BY type, id;
