-- Metadata service database schema
-- 对齐原始服务的元数据表结构

CREATE TABLE IF NOT EXISTS metadata (
    id TEXT PRIMARY KEY,
    key TEXT UNIQUE NOT NULL,                    -- 完整的key，包含bucket前缀 (bucket/object-key)
    size BIGINT NOT NULL,
    content_type TEXT NOT NULL,
    md5_hash TEXT NOT NULL,
    storage_nodes TEXT NOT NULL,                 -- JSON array of storage node IDs
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

-- 索引优化
CREATE INDEX IF NOT EXISTS idx_metadata_key ON metadata(key);
CREATE INDEX IF NOT EXISTS idx_metadata_created_at ON metadata(created_at);
CREATE INDEX IF NOT EXISTS idx_metadata_size ON metadata(size);
CREATE INDEX IF NOT EXISTS idx_metadata_content_type ON metadata(content_type);

-- 支持按bucket前缀查询的索引
CREATE INDEX IF NOT EXISTS idx_metadata_key_prefix ON metadata(key text_pattern_ops);