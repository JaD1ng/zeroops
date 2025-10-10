-- Deploy模块数据库架构
-- 部署相关的主机、实例和版本历史表

-- 创建hosts表：主机信息
CREATE TABLE hosts (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) UNIQUE,
    ip_address VARCHAR(45) UNIQUE,
    is_stopped BOOLEAN
);

-- 创建instances表：服务实例信息
CREATE TABLE instances (
    id VARCHAR(255) NOT NULL PRIMARY KEY,  -- VARCHAR类型主键，非自增，不为空
    service_name VARCHAR(255),
    service_version VARCHAR(255),
    host_id VARCHAR(255),
    host_ip_address VARCHAR(45),
    ip_address VARCHAR(45),
    port INT,
    status VARCHAR(50),
    is_stopped BOOLEAN,
    -- 保留ip_address和port的组合唯一约束
    CONSTRAINT unique_ip_port UNIQUE (ip_address, port)
);

-- 1. 创建service_name和service_version的联合索引
CREATE INDEX idx_instances_service_name_version
ON instances (service_name, service_version);

-- 2. 创建service_name和ip_address的联合索引
CREATE INDEX idx_instances_service_name_ip
ON instances (service_name, ip_address);

-- 3. 创建version_histories表：版本历史记录
CREATE TABLE version_histories (
    id SERIAL PRIMARY KEY,
    instance_id VARCHAR(255),
    service_name VARCHAR(255),
    service_version VARCHAR(255),
    status VARCHAR(50)
);

-- 初始化主机数据
-- 插入 jfcs1021 主机数据
INSERT INTO hosts (name, ip_address, is_stopped)
VALUES ('jfcs1021', '10.210.10.33', false);

-- 插入 jfcs1022 主机数据
INSERT INTO hosts (name, ip_address, is_stopped)
VALUES ('jfcs1022', '10.210.10.30', false);

-- 插入 jfcs1023 主机数据
INSERT INTO hosts (name, ip_address, is_stopped)
VALUES ('jfcs1023', '10.210.10.31', false);