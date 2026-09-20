package team

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Northlatch-Labs-LLC/hivex/internal/config"
)

// TestPortalTurnMeterReportsSettledTurns pins the tier-enforcement wire: a
// settled turn reaches the portal endpoint as a Bearer-authenticated POST
// with the bot, task, and failed flag. The report is fire-and-forget on a
// goroutine, so the test waits on a channel the handler closes — no sleeps.
func TestPortalTurnMeterReportsSettledTurns(t *testing.T) {
	reported := make(chan map[string]any, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test-account-key" {
			t.Errorf("auth = %q, want the account key", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		reported <- body
	}))
	defer srv.Close()

	t.Setenv("HIVEX_PORTAL_TURNS_URL", srv.URL)
	t.Setenv("HIVEX_PORTAL_ACCOUNT_KEY", "sk-test-account-key")
	m := newPortalTurnMeterFromEnv()
	if m == nil {
		t.Fatal("configured env must build a meter")
	}

	b := newTestBroker(t)
	b.SetTurnMeter(m)
	id := b.TurnBegin("researcher", "task-42", "general")
	b.TurnSettle(id, "settled")

	select {
	case body := <-reported:
		if body["agent_slug"] != "researcher" {
			t.Errorf("agent_slug = %v, want researcher", body["agent_slug"])
		}
		if body["task_id"] != "task-42" {
			t.Errorf("task_id = %v, want task-42", body["task_id"])
		}
		if failed, _ := body["failed"].(bool); failed {
			t.Error("failed = true, want false for a settled turn")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("meter report never reached the endpoint")
	}
}

// TestPortalTurnMeterDisabledWithoutEnv pins the fail-safe: with the env
// contract unset, no meter is built and turns settle without any reporting.
func TestPortalTurnMeterDisabledWithoutEnv(t *testing.T) {
	t.Setenv("HIVEX_PORTAL_TURNS_URL", "")
	t.Setenv("HIVEX_PORTAL_ACCOUNT_KEY", "")
	if m := newPortalTurnMeterFromEnv(); m != nil {
		t.Fatal("unconfigured env must disable metering")
	}
}

// TestPortalTurnMeterNilIsTheBrokerDefault pins the broker default: without
// env configuration, NewBrokerAt brokers are unmetered and still settle.
func TestPortalTurnMeterNilIsTheBrokerDefault(t *testing.T) {
	t.Setenv("HIVEX_PORTAL_TURNS_URL", config.Getenv("HIVEX_PORTAL_TURNS_URL"))
	t.Setenv("HIVEX_PORTAL_ACCOUNT_KEY", config.Getenv("HIVEX_PORTAL_ACCOUNT_KEY"))
	b := newTestBroker(t)
	id := b.TurnBegin("researcher", "", "")
	b.TurnSettle(id, "settled")
	if rec := b.TurnRecords()[0]; rec.State != TurnSettled {
		t.Fatalf("state = %q, want settled (nil meter must not break settle)", rec.State)
	}
}
