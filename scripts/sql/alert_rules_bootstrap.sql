-- Recreate alert_rules and alert_rule_metas, then seed one sample rule/meta.
-- NOTE: This will DROP existing tables. Use only in local/dev environments.

BEGIN;

-- Drop tables if exist (order matters due to FK)
DROP TABLE IF EXISTS alert_rule_metas CASCADE;
DROP TABLE IF EXISTS alert_rules CASCADE;
DROP TABLE IF EXISTS alert_meta_change_logs CASCADE;

-- Create alert_rules
CREATE TABLE alert_rules (
    name         varchar(255) PRIMARY KEY,
    description  text,
    expr         text NOT NULL,
    op           varchar(4) NOT NULL CHECK (op IN ('>', '<', '=', '!=')),
    severity     varchar(32) NOT NULL,
    watch_time   interval NOT NULL DEFAULT '0 minutes'
);

-- Create alert_rule_metas
CREATE TABLE alert_rule_metas (
    alert_name   varchar(255) NOT NULL REFERENCES alert_rules(name) ON DELETE CASCADE,
    labels       jsonb NOT NULL DEFAULT '{}'::jsonb,
    threshold    numeric NOT NULL,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (alert_name, labels)
);

-- Create alert_meta_change_logs
CREATE TABLE alert_meta_change_logs (
    id              varchar(64) PRIMARY KEY,
    change_type     varchar(16) NOT NULL CHECK (change_type IN ('Create', 'Update', 'Delete', 'Rollback')),
    change_time     timestamptz NOT NULL DEFAULT now(),
    alert_name      varchar(255) NOT NULL,
    labels          text NOT NULL,
    old_threshold   numeric,
    new_threshold   numeric
);

-- Create indexes for alert_meta_change_logs
CREATE INDEX idx_alert_meta_change_logs_change_time ON alert_meta_change_logs (change_time);
CREATE INDEX idx_alert_meta_change_logs_alert_name_time ON alert_meta_change_logs (alert_name, change_time);

-- Seed sample data per request
INSERT INTO alert_rules(name, description, expr, op, severity, watch_time)
VALUES (
  'http_request_latency_p98_seconds_P0',
  'test',
  'histogram_quantile(0.98, sum(rate(http_latency_seconds_bucket{}[2m])) by (service, service_version, le))',
  '>',
  'P0',
  '5 minutes'::interval
);

INSERT INTO alert_rule_metas(alert_name, labels, threshold)
VALUES (
  'http_request_latency_p98_seconds_P0',
  '{"service":"storage-service","service_version":"1.0.0"}',
  1000
)
ON CONFLICT (alert_name, labels) DO UPDATE SET
  threshold = EXCLUDED.threshold,
  updated_at = now();

COMMIT;
