package receiver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type mockDAO struct{ calls int }

func (m *mockDAO) InsertAlertIssue(_ context.Context, _ *AlertIssueRow) error { m.calls++; return nil }

func TestHandlerCreatesIssues(t *testing.T) {
	r := gin.New()
	m := &mockDAO{}
	h := NewHandler(m)
	RegisterReceiverRoutes(r, h)

	payload := AMWebhook{
		Status: "firing",
		Alerts: []AMAlert{{Status: "firing", StartsAt: time.Now()}},
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/integrations/alertmanager/webhook", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}
