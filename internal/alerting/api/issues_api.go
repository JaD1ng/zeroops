package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fox-gonic/fox"
	adb "github.com/qiniu/zeroops/internal/alerting/database"
	"github.com/qiniu/zeroops/internal/config"
	"github.com/redis/go-redis/v9"
)

type IssueAPI struct {
	R  *redis.Client
	DB *adb.Database
}

// RegisterIssueRoutes registers issue query routes. If rdb is nil, a client is created from env.
// db can be nil; when nil, comments will be empty.
func RegisterIssueRoutes(router *fox.Engine, rdb *redis.Client, db *adb.Database) {
	if rdb == nil {
		rdb = newRedisFromEnv()
	}
	api := &IssueAPI{R: rdb, DB: db}
	router.GET("/v1/issues/:issueID", api.GetIssueByID)
	router.GET("/v1/issues", api.ListIssues)
}

func newRedisFromEnv() *redis.Client { return nil }

func newRedisFromConfig(c *config.RedisConfig) *redis.Client {
	if c == nil {
		return newRedisFromEnv()
	}
	return redis.NewClient(&redis.Options{Addr: c.Addr, Password: c.Password, DB: c.DB})
}

type labelKV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type issueCacheRecord struct {
	ID         string          `json:"id"`
	State      string          `json:"state"`
	Level      string          `json:"level"`
	AlertState string          `json:"alertState"`
	Title      string          `json:"title"`
	Labels     json.RawMessage `json:"labels"`
	AlertSince string          `json:"alertSince"`
}

type issueDetailResponse struct {
	ID         string    `json:"id"`
	State      string    `json:"state"`
	Level      string    `json:"level"`
	AlertState string    `json:"alertState"`
	Title      string    `json:"title"`
	Labels     []labelKV `json:"labels"`
	AlertSince string    `json:"alertSince"`
	Comments   []comment `json:"comments"`
}

type comment struct {
	CreatedAt string `json:"createdAt"`
	Content   string `json:"content"`
}

func (api *IssueAPI) GetIssueByID(c *fox.Context) {
	issueID := c.Param("issueID")
	if issueID == "" {
		c.JSON(http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "INVALID_PARAMETER", "message": "missing issueID"}})
		return
	}
	ctx := context.Background()
	key := "alert:issue:" + issueID
	val, err := api.R.Get(ctx, key).Result()
	if err == redis.Nil || val == "" {
		c.JSON(http.StatusNotFound, map[string]any{"error": map[string]any{"code": "NOT_FOUND", "message": "issue not found"}})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": "INTERNAL_ERROR", "message": err.Error()}})
		return
	}

	var record issueCacheRecord
	if uerr := json.Unmarshal([]byte(val), &record); uerr != nil {
		c.JSON(http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": "INTERNAL_ERROR", "message": "invalid cache format"}})
		return
	}

	var labels []labelKV
	if len(record.Labels) > 0 {
		_ = json.Unmarshal(record.Labels, &labels)
	}

	resp := issueDetailResponse{
		ID:         record.ID,
		State:      record.State,
		Level:      record.Level,
		AlertState: record.AlertState,
		Title:      record.Title,
		Labels:     labels,
		AlertSince: normalizeTimeString(record.AlertSince),
		Comments:   api.fetchComments(c.Request.Context(), record.ID),
	}
	c.JSON(http.StatusOK, resp)
}

func normalizeTimeString(s string) string {
	if s == "" {
		return s
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC().Format(time.RFC3339Nano)
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC().Format(time.RFC3339Nano)
	}
	return s
}

func (api *IssueAPI) fetchComments(ctx context.Context, issueID string) []comment {
	if api.DB == nil || issueID == "" {
		return []comment{}
	}
	const q = `SELECT create_at, content FROM alert_issue_comments WHERE issue_id=$1 ORDER BY create_at ASC`
	rows, err := api.DB.QueryContext(ctx, q, issueID)
	if err != nil {
		return []comment{}
	}
	defer rows.Close()
	out := make([]comment, 0, 4)
	for rows.Next() {
		var t time.Time
		var content string
		if err := rows.Scan(&t, &content); err != nil {
			continue
		}
		out = append(out, comment{CreatedAt: t.UTC().Format(time.RFC3339Nano), Content: content})
	}
	return out
}

type listResponse struct {
	Items []issueListItem `json:"items"`
	Next  string          `json:"next,omitempty"`
}

type issueListItem struct {
	ID         string    `json:"id"`
	State      string    `json:"state"`
	Level      string    `json:"level"`
	AlertState string    `json:"alertState"`
	Title      string    `json:"title"`
	Labels     []labelKV `json:"labels"`
	AlertSince string    `json:"alertSince"`
}

func (api *IssueAPI) ListIssues(c *fox.Context) {
	start := strings.TrimSpace(c.Query("start"))
	limitStr := strings.TrimSpace(c.Query("limit"))
	if limitStr == "" {
		c.JSON(http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "INVALID_PARAMETER", "message": "limit is required"}})
		return
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		c.JSON(http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "INVALID_PARAMETER", "message": "limit must be 1-100"}})
		return
	}

	state := strings.TrimSpace(c.Query("state"))
	idxKey := "alert:index:open"
	if state != "" {
		if strings.EqualFold(state, "Open") {
			idxKey = "alert:index:open"
		} else if strings.EqualFold(state, "Closed") {
			idxKey = "alert:index:closed"
		} else {
			c.JSON(http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "INVALID_PARAMETER", "message": "state must be Open or Closed"}})
			return
		}
	}

	var cursor uint64
	if start != "" {
		if cv, err := strconv.ParseUint(start, 10, 64); err == nil {
			cursor = cv
		}
	}

	ctx := context.Background()
	ids, nextCursor, err := api.R.SScan(ctx, idxKey, cursor, "", int64(limit)).Result()
	if err != nil && err != redis.Nil {
		c.JSON(http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": "INTERNAL_ERROR", "message": err.Error()}})
		return
	}

	if len(ids) == 0 {
		c.JSON(http.StatusOK, listResponse{Items: []issueListItem{}, Next: ""})
		return
	}

	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		keys = append(keys, "alert:issue:"+id)
	}

	vals, err := api.R.MGet(ctx, keys...).Result()
	if err != nil && err != redis.Nil {
		c.JSON(http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": "INTERNAL_ERROR", "message": err.Error()}})
		return
	}

	items := make([]issueListItem, 0, len(vals))
	for _, v := range vals {
		if v == nil {
			continue
		}
		var rec issueCacheRecord
		switch t := v.(type) {
		case string:
			_ = json.Unmarshal([]byte(t), &rec)
		case []byte:
			_ = json.Unmarshal(t, &rec)
		default:
			b, _ := json.Marshal(t)
			_ = json.Unmarshal(b, &rec)
		}
		var labels []labelKV
		if len(rec.Labels) > 0 {
			_ = json.Unmarshal(rec.Labels, &labels)
		}
		items = append(items, issueListItem{
			ID:         rec.ID,
			State:      rec.State,
			Level:      rec.Level,
			AlertState: rec.AlertState,
			Title:      rec.Title,
			Labels:     labels,
			AlertSince: normalizeTimeString(rec.AlertSince),
		})
	}

	resp := listResponse{Items: items}
	if nextCursor != 0 {
		resp.Next = strconv.FormatUint(nextCursor, 10)
	}
	c.JSON(http.StatusOK, resp)
}
