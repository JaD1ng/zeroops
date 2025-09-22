package remediation

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	adb "github.com/qiniu/zeroops/internal/alerting/database"
	"github.com/qiniu/zeroops/internal/alerting/service/healthcheck"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

type Consumer struct {
	DB    *adb.Database
	Redis *redis.Client

	// sleepFn allows overriding for tests
	sleepFn func(time.Duration)
}

func NewConsumer(db *adb.Database, rdb *redis.Client) *Consumer {
	return &Consumer{DB: db, Redis: rdb, sleepFn: time.Sleep}
}

// Start consumes alert messages and performs a mocked rollback then marks restored.
func (c *Consumer) Start(ctx context.Context, ch <-chan healthcheck.AlertMessage) {
	if ch == nil {
		log.Warn().Msg("remediation consumer started without channel; no-op")
		return
	}
	sleepDur := parseDuration(os.Getenv("REMEDIATION_ROLLBACK_SLEEP"), 30*time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-ch:
			// 1) Mock rollback: optional URL composition (unused)
			_ = fmt.Sprintf(os.Getenv("REMEDIATION_ROLLBACK_URL"), deriveDeployID(&m))
			// 2) Sleep to simulate rollback time
			if c.sleepFn != nil {
				c.sleepFn(sleepDur)
			}
			// 3) On success: add AI analysis comment, update DB and cache
			if err := c.addAIAnalysisComment(ctx, &m); err != nil {
				log.Error().Err(err).Str("issue", m.ID).Msg("addAIAnalysisComment failed")
			}
			if err := c.markRestoredInDB(ctx, &m); err != nil {
				log.Error().Err(err).Str("issue", m.ID).Msg("markRestoredInDB failed")
			}
			if err := c.markRestoredInCache(ctx, &m); err != nil {
				log.Error().Err(err).Str("issue", m.ID).Msg("markRestoredInCache failed")
			}
		}
	}
}

func deriveDeployID(m *healthcheck.AlertMessage) string {
	if m == nil {
		return ""
	}
	if v := m.Labels["deploy_id"]; v != "" {
		return v
	}
	return fmt.Sprintf("%s:%s", m.Service, m.Version)
}

func (c *Consumer) addAIAnalysisComment(ctx context.Context, m *healthcheck.AlertMessage) error {
	if c.DB == nil || m == nil {
		return nil
	}
	const existsQ = `SELECT 1 FROM alert_issue_comments WHERE issue_id=$1 AND content=$2 LIMIT 1`
	const insertQ = `INSERT INTO alert_issue_comments (issue_id, create_at, content) VALUES ($1, NOW(), $2)`
	content := "## AI分析结果\n" +
		"**问题类型**：非发版本导致的问题\n" +
		"**根因分析**：数据库连接池配置不足，导致大量请求无法获取数据库连接\n" +
		"**处理建议**：\n" +
		"- 增加数据库连接池大小\n" +
		"- 优化数据库连接管理\n" +
		"- 考虑读写分离缓解压力\n" +
		"**执行状态**：正在处理中，等待指标恢复正常"
	if rows, err := c.DB.QueryContext(ctx, existsQ, m.ID, content); err == nil {
		defer rows.Close()
		if rows.Next() {
			return nil
		}
	}
	_, err := c.DB.ExecContext(ctx, insertQ, m.ID, content)
	return err
}

func (c *Consumer) markRestoredInDB(ctx context.Context, m *healthcheck.AlertMessage) error {
	if c.DB == nil || m == nil {
		return nil
	}
	// alert_issues
	if _, err := c.DB.ExecContext(ctx, `UPDATE alert_issues SET alert_state = 'Restored' , state = 'Closed' WHERE id = $1`, m.ID); err != nil {
		return err
	}
	// service_states (upsert)
	if m.Service != "" {
		const upsert = `
INSERT INTO service_states (service, version, report_at, resolved_at, health_state, alert_issue_ids)
VALUES ($1, $2, NULL, NOW(), 'Normal', ARRAY[$3]::text[])
ON CONFLICT (service, version) DO UPDATE
SET health_state = 'Normal',
    resolved_at = NOW();
`
		if _, err := c.DB.ExecContext(ctx, upsert, m.Service, m.Version, m.ID); err != nil {
			return err
		}
	}
	return nil
}

func (c *Consumer) markRestoredInCache(ctx context.Context, m *healthcheck.AlertMessage) error {
	if c.Redis == nil || m == nil {
		return nil
	}
	// 1) alert:issue:{id} → alertState=Restored; state=Closed; move indices
	alertKey := "alert:issue:" + m.ID
	script := redis.NewScript(`
local v = redis.call('GET', KEYS[1])
if not v then return 0 end
local obj = cjson.decode(v)
obj.alertState = ARGV[1]
obj.state = ARGV[3]
redis.call('SET', KEYS[1], cjson.encode(obj), 'KEEPTTL')
if KEYS[2] ~= '' then redis.call('SREM', KEYS[2], ARGV[2]) end
if KEYS[3] ~= '' then redis.call('SREM', KEYS[3], ARGV[2]) end
if KEYS[4] ~= '' then redis.call('SADD', KEYS[4], ARGV[2]) end
-- move open→closed indices
if KEYS[5] ~= '' then redis.call('SREM', KEYS[5], ARGV[2]) end
if KEYS[6] ~= '' then redis.call('SADD', KEYS[6], ARGV[2]) end
-- service scoped indices if service exists in payload
local svc = obj['service']
if svc and svc ~= '' then
  local openSvcKey = 'alert:index:svc:' .. svc .. ':open'
  local closedSvcKey = 'alert:index:svc:' .. svc .. ':closed'
  redis.call('SREM', openSvcKey, ARGV[2])
  redis.call('SADD', closedSvcKey, ARGV[2])
end
return 1
`)
	_, _ = script.Run(ctx, c.Redis, []string{alertKey, "alert:index:alert_state:Pending", "alert:index:alert_state:InProcessing", "alert:index:alert_state:Restored", "alert:index:open", "alert:index:closed"}, "Restored", m.ID, "Closed").Result()

	// 2) service_state:{service}:{version} → health_state=Normal; resolved_at=now; add to Normal index
	if m.Service != "" {
		svcKey := "service_state:" + m.Service + ":" + m.Version
		now := time.Now().UTC().Format(time.RFC3339Nano)
		svcScript := redis.NewScript(`
local v = redis.call('GET', KEYS[1])
if not v then v = '{}' end
local obj = cjson.decode(v)
obj.health_state = ARGV[1]
obj.resolved_at = ARGV[2]
redis.call('SET', KEYS[1], cjson.encode(obj), 'KEEPTTL')
if KEYS[2] ~= '' then redis.call('SADD', KEYS[2], KEYS[1]) end
return 1
`)
		_, _ = svcScript.Run(ctx, c.Redis, []string{svcKey, "service_state:index:health:Normal"}, "Normal", now).Result()
	}
	return nil
}

func parseDuration(s string, d time.Duration) time.Duration {
	if s == "" {
		return d
	}
	if v, err := time.ParseDuration(s); err == nil {
		return v
	}
	return d
}

func parseInt(s string, v int) int {
	if s == "" {
		return v
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return v
}
