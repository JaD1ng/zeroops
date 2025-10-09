//go:build integration

package receiver

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	adb "github.com/qiniu/zeroops/internal/alerting/database"
)

func ensureSchema(t *testing.T, db *adb.Database) {
	t.Helper()
	// TODO：更新 alert_issues 表
	const schema = `
CREATE TABLE IF NOT EXISTS alert_issues (
  id           varchar(64)  PRIMARY KEY,
  state        varchar(16)  NOT NULL,
  level        varchar(32)  NOT NULL,
  alert_state  varchar(32)  NOT NULL,
  title        varchar(255) NOT NULL,
  labels       json          NOT NULL,
  alert_since  timestamp(6)  NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_alert_issues_state_level_since ON alert_issues(state, level, alert_since);
CREATE INDEX IF NOT EXISTS idx_alert_issues_alertstate_since ON alert_issues(alert_state, alert_since);
`
	if _, err := db.ExecContext(context.Background(), schema); err != nil {
		t.Fatalf("init schema: %v", err)
	}
}

func TestPgDAO_InsertAlertIssue(t *testing.T) {
	dsn := "host=" + os.Getenv("DB_HOST") +
		" port=" + os.Getenv("DB_PORT") +
		" user=" + os.Getenv("DB_USER") +
		" password=" + os.Getenv("DB_PASSWORD") +
		" dbname=" + os.Getenv("DB_NAME") +
		" sslmode=" + os.Getenv("DB_SSLMODE")

	db, err := adb.New(dsn)
	if err != nil {
		t.Fatalf("db connect: %v", err)
	}
	defer db.Close()

	ensureSchema(t, db)

	dao := NewPgDAO(db)
	row := &AlertIssueRow{
		ID:         uuid.NewString(),
		State:      "Open",
		Level:      "P1",
		AlertState: "Pending",
		Title:      "integration insert",
		LabelJSON:  []byte(`[{"key":"k","value":"v"}]`),
		AlertSince: time.Now().UTC(),
	}
	if err := dao.InsertAlertIssue(context.Background(), row); err != nil {
		t.Fatalf("insert: %v", err)
	}
}
