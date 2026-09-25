package team

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Northlatch-Labs-LLC/hivex/internal/provider"
)

// newTurnsLiveTest mounts the /turns/live route exactly as the broker does
// (requireAuth-wrapped) and returns the server and broker, so the tests
// exercise the same wire the web office polls.
func newTurnsLiveTest(t *testing.T) (*httptest.Server, *Broker) {
	t.Helper()
	b := newTestBroker(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/turns/live", b.requireAuth(b.handleTurnsLive))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, b
}

// getTurnsLive decodes one authenticated GET /turns/live response.
func getTurnsLive(t *testing.T, ts *httptest.Server, token string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/turns/live", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp.StatusCode, body
}

// turnLiveEntry is one entry of the /turns/live payload, decoded with types
// intact (json.Unmarshal into any would turn numbers into float64).
func turnLiveEntry(t *testing.T, body map[string]any, agent string) map[string]any {
	t.Helper()
	raw, ok := body["turns"]
	if !ok {
		t.Fatalf("response has no turns key: %v", body)
	}
	list, ok := raw.([]any)
	if !ok {
		t.Fatalf("turns is %T, want a list", raw)
	}
	for _, item := range list {
		entry, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("turns entry is %T, want an object", item)
		}
		if entry["agent"] == agent {
			return entry
		}
	}
	t.Fatalf("no turns entry for agent %q in %v", agent, body)
	return nil
}

// TestTurnsLiveRequiresAuth pins the auth contract: without the bearer
// token the endpoint answers 401 and never leaks turn state.
func TestTurnsLiveRequiresAuth(t *testing.T) {
	ts, _ := newTurnsLiveTest(t)
	status, body := getTurnsLive(t, ts, "")
	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 without a token", status)
	}
	if body["error"] != "unauthorized" {
		t.Fatalf("body = %v, want the unauthorized error contract", body)
	}
}

// TestTurnsLiveReturnsLatestTurnPerAgent pins the live-work contract: one
// entry per agent (the LATEST turn, not the whole journal), with the engine
// state, the member binding's model, and the stamped usage.
func TestTurnsLiveReturnsLatestTurnPerAgent(t *testing.T) {
	ts, b := newTurnsLiveTest(t)

	// scout gets a member row and a bound model, so the model enrichment
	// has a real binding to read. In-package fixture append, mirroring
	// seedTestTeamRoom's direct-state style.
	b.mu.Lock()
	b.members = append(b.members, officeMember{Slug: "scout", Name: "Scout"})
	b.mu.Unlock()
	if err := b.SetMemberProvider("scout", provider.ProviderBinding{Kind: provider.KindZAI, Model: "glm-5.3"}); err != nil {
		t.Fatalf("set member provider: %v", err)
	}

	// scout's settled turn is superseded by a live dispatch turn — the
	// endpoint must report the dispatch one, proving latest-per-agent.
	settled := b.TurnBegin("scout", "task-old", testTeamRoom)
	b.TurnSettle(settled, "ledger recorded")
	live := b.TurnBegin("scout", "task-live", testTeamRoom)
	b.TurnTransition(live, TurnDispatch, "runner picked up")
	b.StampTurnUsage(live, provider.ClaudeUsage{InputTokens: 1200, OutputTokens: 300})
	// A second agent mid-verification proves the surface covers the roster,
	// not just one bot.
	cos := b.TurnBegin("cos", "task-cos", testTeamRoom)
	b.TurnTransition(cos, TurnVerifying, "gate running")

	status, body := getTurnsLive(t, ts, b.Token())
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	rawList, ok := body["turns"].([]any)
	if !ok {
		t.Fatalf("turns is %T, want a list", body["turns"])
	}
	if len(rawList) != 2 {
		t.Fatalf("turns has %d entries, want exactly one per agent (2)", len(rawList))
	}

	scout := turnLiveEntry(t, body, "scout")
	if scout["state"] != string(TurnDispatch) {
		t.Errorf("scout state = %v, want the latest (dispatch) turn", scout["state"])
	}
	if scout["task_id"] != "task-live" {
		t.Errorf("scout task_id = %v, want task-live", scout["task_id"])
	}
	if scout["model"] != "glm-5.3" {
		t.Errorf("scout model = %v, want the member binding's model", scout["model"])
	}
	usage, ok := scout["usage"].(map[string]any)
	if !ok {
		t.Fatalf("scout usage = %T, want the stamped usage object", scout["usage"])
	}
	if usage["input_tokens"] != float64(1200) || usage["output_tokens"] != float64(300) {
		t.Errorf("scout usage = %v, want the stamped 1200/300 tokens", usage)
	}
	if scout["started_at"] == "" || scout["updated_at"] == "" {
		t.Errorf("scout timestamps missing: %v", scout)
	}

	cosEntry := turnLiveEntry(t, body, "cos")
	if cosEntry["state"] != string(TurnVerifying) {
		t.Errorf("cos state = %v, want verifying", cosEntry["state"])
	}
	// cos has no member binding on this fixture; the model key must be
	// absent, not a stub.
	if _, has := cosEntry["model"]; has {
		t.Errorf("cos model = %v, want absent (no binding)", cosEntry["model"])
	}
}

// TestTurnsLiveEmptyJournal pins the empty-office shape: no turns recorded
// yet means an empty list, not null — the web polling hook treats a stable
// array shape as its contract.
func TestTurnsLiveEmptyJournal(t *testing.T) {
	ts, b := newTurnsLiveTest(t)
	status, body := getTurnsLive(t, ts, b.Token())
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	list, ok := body["turns"].([]any)
	if !ok {
		t.Fatalf("turns is %T, want a list", body["turns"])
	}
	if len(list) != 0 {
		t.Fatalf("turns has %d entries, want an empty list", len(list))
	}
}
