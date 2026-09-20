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

// TestPortalTurnMeterCapturesTheBudgetResponse pins the Free-cap gate's
// data source: the turns endpoint's authoritative {success, used, cap,
// capped} answer is captured from a real HTTP round trip and arms the
// pre-turn gate; a non-capped answer re-opens it.
func TestPortalTurnMeterCapturesTheBudgetResponse(t *testing.T) {
	resetPortalTurnBudgetForTests()
	t.Cleanup(resetPortalTurnBudgetForTests)

	capped := make(chan bool, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "used": 1000, "cap": 1000, "capped": true, "remaining": 0})
		capped <- true
	}))
	defer srv.Close()

	t.Setenv("HIVEX_PORTAL_TURNS_URL", srv.URL)
	t.Setenv("HIVEX_PORTAL_ACCOUNT_KEY", "sk-test-account-key")
	m := newPortalTurnMeterFromEnv()

	b := newTestBroker(t)
	b.SetTurnMeter(m)
	id := b.TurnBegin("researcher", "task-cap", "general")
	b.TurnSettle(id, "settled")

	select {
	case <-capped:
	case <-time.After(5 * time.Second):
		t.Fatal("meter report never reached the endpoint")
	}
	// The capture happens on the meter's goroutine; poll briefly for it.
	for i := 0; i < 50 && !turnGateBlockedByPortal(); i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if !turnGateBlockedByPortal() {
		t.Fatal("capped budget answer must arm the pre-turn gate")
	}

	// An uncapped answer re-opens the gate.
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "used": 5, "cap": 1000, "capped": false, "remaining": 995})
	}))
	defer srv2.Close()
	m2 := &portalTurnMeter{url: srv2.URL, apiKey: "sk-test-account-key", client: &http.Client{Timeout: portalTurnTimeout}}
	m2.MeterTurn("researcher", "task-cap", false)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && turnGateBlockedByPortal() {
		time.Sleep(10 * time.Millisecond)
	}
	if turnGateBlockedByPortal() {
		t.Fatal("uncapped budget answer must re-open the gate")
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
