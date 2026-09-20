package team

// turn_meter_portal.go — the production TurnMeter: reports every settled
// turn to the HiveAPI portal's metering endpoint (POST /api/portal/turns),
// which is where the Free tier's agentic cap is enforced. Configure with:
//
//	HIVEX_PORTAL_TURNS_URL=http://localhost:8080/api/portal/turns
//	HIVEX_PORTAL_ACCOUNT_KEY=sk-…   (one of the account's gateway keys)
//
// Contract (portal/turns.js): the endpoint accepts the bot slug, an
// optional task id, and the failed flag; auth is a portal session OR one of
// the account's gateway keys as Bearer. Fire-and-forget by design: the
// settle path must never block or fail a turn on metering — a metering
// outage degrades to under-counting, and the portal's cap is enforced on
// what was reported, never invented.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Northlatch-Labs-LLC/hivex/internal/config"
)

// portalTurnTimeout bounds a meter report. The report runs on the settle
// path, so it must be fast; the portal is loopback-adjacent on Northlatch
// infra, and a slow report is dropped rather than queued.
const portalTurnTimeout = 2 * time.Second

// portalTurnMeter implements TurnMeter against the portal endpoint.
type portalTurnMeter struct {
	url    string
	apiKey string
	client *http.Client
}

// newPortalTurnMeterFromEnv builds the meter from the env contract. Both
// vars must be set; otherwise nil (metering disabled — the broker keeps its
// nil-safe default).
func newPortalTurnMeterFromEnv() TurnMeter {
	url := strings.TrimSpace(config.Getenv("HIVEX_PORTAL_TURNS_URL"))
	key := strings.TrimSpace(config.Getenv("HIVEX_PORTAL_ACCOUNT_KEY"))
	if url == "" || key == "" {
		return nil
	}
	return &portalTurnMeter{
		url:    strings.TrimRight(url, "/"),
		apiKey: key,
		client: &http.Client{Timeout: portalTurnTimeout},
	}
}

// MeterTurn reports one settled turn. Fire-and-forget: errors are the
// meter's problem, never the turn's.
func (m *portalTurnMeter) MeterTurn(bot, taskID string, failed bool) {
	if m == nil || m.url == "" {
		return
	}
	// The POST runs on a fresh goroutine so the settle path never waits on
	// the portal — the TurnEngine contract ("never block the settle path")
	// is honored structurally, not by timeout alone.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), portalTurnTimeout)
		defer cancel()
		body := map[string]any{
			"agent_slug": bot,
			"task_id":    taskID,
			"failed":     failed,
		}
		raw, err := json.Marshal(body)
		if err != nil {
			return
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.url, bytes.NewReader(raw))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+m.apiKey)
		res, err := m.client.Do(req)
		if err != nil {
			return
		}
		// The endpoint answers with the authoritative budget
		// {success, used, cap, capped, remaining} — capture it for the
		// pre-turn gate. Parse failures degrade to the previous posture
		// (under-counting, fail-open gate).
		respRaw, respErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		_ = res.Body.Close()
		if respErr != nil {
			return
		}
		var report struct {
			Success bool   `json:"success"`
			Used    int64  `json:"used"`
			Cap     *int64 `json:"cap"`
			Capped  bool   `json:"capped"`
		}
		if json.Unmarshal(respRaw, &report) == nil && report.Success {
			recordPortalTurnBudget(&portalTurnBudget{
				Used:   report.Used,
				Cap:    report.Cap,
				Capped: report.Capped,
				At:     time.Now(),
			})
		}
	}()
}

// portalTurnBudget is the cap state the turns endpoint returns with every
// meter report: {success, used, cap, capped, remaining}. The harness uses the
// authoritative capped flag as the pre-turn gate.
type portalTurnBudget struct {
	Used   int64  `json:"used"`
	Cap    *int64 `json:"cap"`
	Capped bool   `json:"capped"`
	At     time.Time
}

var (
	portalBudgetMu sync.Mutex
	portalBudget   *portalTurnBudget
)

func recordPortalTurnBudget(b *portalTurnBudget) {
	portalBudgetMu.Lock()
	defer portalBudgetMu.Unlock()
	portalBudget = b
}

// lastPortalTurnBudget returns the most recent meter-report budget, if any.
func lastPortalTurnBudget() *portalTurnBudget {
	portalBudgetMu.Lock()
	defer portalBudgetMu.Unlock()
	if portalBudget == nil {
		return nil
	}
	out := *portalBudget
	return &out
}

func resetPortalTurnBudgetForTests() {
	portalBudgetMu.Lock()
	defer portalBudgetMu.Unlock()
	portalBudget = nil
}

// installPortalTurnMeter wires the production meter onto the broker when the
// env contract is configured. Called once at broker startup; idempotent and
// nil-safe so every entry point (office, headless, tests) can call it.
func installPortalTurnMeter(b *Broker) {
	if b == nil {
		return
	}
	if m := newPortalTurnMeterFromEnv(); m != nil {
		b.SetTurnMeter(m)
	}
}
