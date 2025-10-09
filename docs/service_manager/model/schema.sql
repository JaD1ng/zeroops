-- ZeroOps Service Manager Database Schema

-- 删除现有表（按依赖关系逆序删除）
DROP TABLE IF EXISTS deploy_tasks;
DROP TABLE IF EXISTS service_states;
DROP TABLE IF EXISTS service_instances;
DROP TABLE IF EXISTS service_versions;
DROP TABLE IF EXISTS services;

-- 服务表
CREATE TABLE IF NOT EXISTS services (
    name VARCHAR(255) PRIMARY KEY,
    deps JSONB DEFAULT '[]'::jsonb
);

-- 服务版本表
CREATE TABLE IF NOT EXISTS service_versions (
    version VARCHAR(255),
    service VARCHAR(255),
    create_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (version, service),
    FOREIGN KEY (service) REFERENCES services(name) ON DELETE CASCADE
);

-- 服务实例表
CREATE TABLE IF NOT EXISTS service_instances (
    id VARCHAR(255) PRIMARY KEY,
    service VARCHAR(255),
    version VARCHAR(255),
    FOREIGN KEY (service) REFERENCES services(name) ON DELETE CASCADE
);

-- 服务状态表
CREATE TABLE IF NOT EXISTS service_states (
    service VARCHAR(255),
    version VARCHAR(255),
    report_at TIMESTAMP,
    resolved_at TIMESTAMP,
    health_state VARCHAR(50),
    alert_issue_ids VARCHAR(255),
    PRIMARY KEY (service, version),
    FOREIGN KEY (service) REFERENCES services(name) ON DELETE CASCADE
);

-- 部署任务表 (deploy_tasks)
CREATE TABLE IF NOT EXISTS deploy_tasks (
    service VARCHAR(255),
    version VARCHAR(255),
    start_time TIMESTAMP,
    end_time TIMESTAMP,
    target_ratio DOUBLE PRECISION,
    instances JSONB DEFAULT '[]'::jsonb,
    deploy_state VARCHAR(50),
    PRIMARY KEY (service, version),
    FOREIGN KEY (service) REFERENCES services(name) ON DELETE CASCADE
);

-- 创建索引以提高查询性能
CREATE INDEX IF NOT EXISTS idx_service_states_service ON service_states(service);
CREATE INDEX IF NOT EXISTS idx_service_states_report_at ON service_states(service, report_at DESC);
CREATE INDEX IF NOT EXISTS idx_deploy_tasks_service ON deploy_tasks(service);
CREATE INDEX IF NOT EXISTS idx_deploy_tasks_state ON deploy_tasks(deploy_state);
CREATE INDEX IF NOT EXISTS idx_service_instances_service ON service_instances(service);

-- 插入Mock S3项目的真实服务数据
-- 服务及其依赖关系（基于实际业务流程）
INSERT INTO services (name, deps) VALUES 
    ('storage', '[]'::jsonb),                     -- 存储服务：基础服务
    ('metadata', '["storage"]'::jsonb),           -- 元数据服务：依赖存储服务
    ('queue', '["storage"]'::jsonb),              -- 队列服务：依赖存储服务
    ('third-party', '[]'::jsonb),                 -- 第三方服务：独立
    ('mock-error', '[]'::jsonb)                   -- 错误模拟服务：独立
ON CONFLICT (name) DO NOTHING;

-- 服务版本：metadata, storage, queue, third-party 各有3个版本，mock-error只有1个版本
INSERT INTO service_versions (version, service, create_time) VALUES 
    -- metadata service versions
    ('v1.0.0', 'metadata', CURRENT_TIMESTAMP - INTERVAL '60 days'),
    ('v1.1.0', 'metadata', CURRENT_TIMESTAMP - INTERVAL '30 days'),
    ('v1.2.0', 'metadata', CURRENT_TIMESTAMP - INTERVAL '7 days'),
    -- storage service versions  
    ('v1.0.0', 'storage', CURRENT_TIMESTAMP - INTERVAL '55 days'),
    ('v1.1.0', 'storage', CURRENT_TIMESTAMP - INTERVAL '25 days'),
    ('v1.2.0', 'storage', CURRENT_TIMESTAMP - INTERVAL '5 days'),
    -- queue service versions
    ('v1.0.0', 'queue', CURRENT_TIMESTAMP - INTERVAL '50 days'),
    ('v1.1.0', 'queue', CURRENT_TIMESTAMP - INTERVAL '20 days'),
    ('v1.2.0', 'queue', CURRENT_TIMESTAMP - INTERVAL '3 days'),
    -- third-party service versions
    ('v1.0.0', 'third-party', CURRENT_TIMESTAMP - INTERVAL '45 days'),
    ('v1.1.0', 'third-party', CURRENT_TIMESTAMP - INTERVAL '15 days'),
    ('v1.2.0', 'third-party', CURRENT_TIMESTAMP - INTERVAL '1 day'),
    -- mock-error service version
    ('v1.0.0', 'mock-error', CURRENT_TIMESTAMP - INTERVAL '40 days')
ON CONFLICT (version, service) DO NOTHING;
