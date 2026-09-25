package team

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Northlatch-Labs-LLC/hivex/internal/config"
	"github.com/Northlatch-Labs-LLC/hivex/internal/gridframe"
	"github.com/Northlatch-Labs-LLC/hivex/internal/provider"
)

// The citizen provisioning primitive: POST /citizens provisions an office
// member through the office-members create path (kind zai, default model),
// mints sk-hivex-<hex24> that crosses the wire exactly once, books a
// cost-ledger row, and meters the citizen's turns under usage ByKind "zai".
// DELETE /citizens/{slug} revokes member + key.

var citizenKeyPattern = regexp.MustCompile(`^sk-hivex-[0-9a-f]{24}$`)

// newCitizenTestBroker returns a test broker whose gridframe store persists
// to a temp dir (never the operator's real ledgers) plus that dir.
func newCitizenTestBroker(t *testing.T) (*Broker, string) {
	t.Helper()
	b := newTestBroker(t)
	dir := t.TempDir()
	s := gridframe.NewStore()
	if err := s.PersistTo(dir); err != nil {
		t.Fatalf("gridframe PersistTo: %v", err)
	}
	b.gridframeMu.Lock()
	b.gridframeStore = s
	b.gridframeMu.Unlock()
	return b, dir
}

func postCitizen(t *testing.T, b *Broker, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/citizens", strings.NewReader(body))
	b.handleCitizens(rec, req)
	out := map[string]any{}
	// Error paths use http.Error (plain text); only JSON bodies decode.
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil && rec.Code < 400 {
		t.Fatalf("decode response: %v", err)
	}
	return rec, out
}

func deleteCitizen(t *testing.T, b *Broker, slug string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/citizens/"+slug, nil)
	req.SetPathValue("slug", slug)
	b.handleCitizensSubpath(rec, req)
	return rec
}

func TestCitizenProvisionFlow(t *testing.T) {
	cfgDir := t.TempDir()
	t.Setenv("HIVEX_CONFIG_PATH", filepath.Join(cfgDir, "config.json"))
	b, ledgerDir := newCitizenTestBroker(t)

	rec, out := postCitizen(t, b, `{"weir_handle":"Weir Tester"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("provision: got %d body %v", rec.Code, out)
	}
	slug, _ := out["slug"].(string)
	key, _ := out["key"].(string)
	provisionedAt, _ := out["provisioned_at"].(string)
	if slug != "weir-tester" {
		t.Fatalf("slug: got %q want weir-tester", slug)
	}
	if !citizenKeyPattern.MatchString(key) {
		t.Fatalf("key must be sk-hivex-<24 hex>, got %q", key)
	}
	if provisionedAt == "" {
		t.Fatal("provisioned_at required")
	}

	// Member provisioned with the zai binding + default model, via the
	// office-members create semantics (roster visible).
	var member *officeMember
	for _, m := range b.OfficeMembers() {
		if m.Slug == slug {
			cp := m
			member = &cp
		}
	}
	if member == nil {
		t.Fatal("citizen not in office members")
	}
	if member.Provider.Kind != provider.KindZAI {
		t.Fatalf("provider kind: got %q want %q", member.Provider.Kind, provider.KindZAI)
	}
	if member.Provider.Model != provider.ZaiDefaultModel() {
		t.Fatalf("provider model: got %q want default %q", member.Provider.Model, provider.ZaiDefaultModel())
	}

	// Key persisted in config (plain, like zai_api_key) with the timestamp.
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config load: %v", err)
	}
	if len(cfg.CitizenKeys) != 1 || cfg.CitizenKeys[0].Slug != slug || cfg.CitizenKeys[0].APIKey != key {
		t.Fatalf("citizen key not stored as issued: %+v", cfg.CitizenKeys)
	}
	if cfg.CitizenKeys[0].CreatedAt != provisionedAt {
		t.Fatalf("created_at: got %q want %q", cfg.CitizenKeys[0].CreatedAt, provisionedAt)
	}

	// Ledger row appended: item citizen-provision:<slug>, append-only CSV on
	// disk (the same store the /gridframe surface serves).
	raw, err := os.ReadFile(filepath.Join(ledgerDir, "cost-ledger.csv"))
	if err != nil {
		t.Fatalf("read cost ledger: %v", err)
	}
	if !strings.Contains(string(raw), "citizen-provision:"+slug) {
		t.Fatalf("cost ledger missing provision row:\n%s", raw)
	}

	// The key is never returned again: the members surface (the only citizen
	// read path besides config.json on disk) must not echo it.
	listRec := httptest.NewRecorder()
	b.handleOfficeMembers(listRec, httptest.NewRequest(http.MethodGet, "/office-members", nil))
	if strings.Contains(listRec.Body.String(), key) {
		t.Fatal("plaintext citizen key leaked through /office-members")
	}

	// Duplicate provision → conflict, no second key.
	rec, out = postCitizen(t, b, `{"weir_handle":"weir-tester"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate provision: got %d body %v", rec.Code, out)
	}
	cfg, _ = config.Load()
	if len(cfg.CitizenKeys) != 1 {
		t.Fatalf("duplicate provision must not mint a second key: %+v", cfg.CitizenKeys)
	}

	// Missing handle → 400 at the boundary (not laundered into "general").
	rec, _ = postCitizen(t, b, `{"weir_handle":"  "}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty weir_handle: got %d want 400", rec.Code)
	}

	// Revoke: member gone, key destroyed.
	if rec := deleteCitizen(t, b, slug); rec.Code != http.StatusOK {
		t.Fatalf("revoke: got %d body %s", rec.Code, rec.Body.String())
	}
	for _, m := range b.OfficeMembers() {
		if m.Slug == slug {
			t.Fatal("revoked citizen still in office members")
		}
	}
	cfg, err = config.Load()
	if err != nil {
		t.Fatalf("config load after revoke: %v", err)
	}
	if len(cfg.CitizenKeys) != 0 {
		t.Fatalf("revoked citizen key must be removed: %+v", cfg.CitizenKeys)
	}
	// Second revoke → 404 (idempotent on the key, but the member is gone).
	if rec := deleteCitizen(t, b, slug); rec.Code != http.StatusNotFound {
		t.Fatalf("revoke of unknown citizen: got %d want 404", rec.Code)
	}
}

// TestCitizensServeMuxWiring drives the exact route patterns registered in
// broker.go's StartOnPort through a real ServeMux: the {slug} wildcard must
// populate r.PathValue for the DELETE, and /api/citizens on the web port
// proxies to the broker's /citizens (broker_web_proxy strips /api), so the
// public shape is POST /api/citizens + DELETE /api/citizens/{slug}.
func TestCitizensServeMuxWiring(t *testing.T) {
	t.Setenv("HIVEX_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	b, _ := newCitizenTestBroker(t)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /citizens", b.handleCitizens)
	mux.HandleFunc("DELETE /citizens/{slug}", b.handleCitizensSubpath)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/citizens", strings.NewReader(`{"weir_handle":"weir-wired"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("mux POST: got %d body %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/citizens/weir-wired", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("mux DELETE: got %d body %s — {slug} pattern must populate PathValue", rec.Code, rec.Body.String())
	}
	if rec := deleteCitizen(t, b, "weir-wired"); rec.Code != http.StatusNotFound {
		t.Fatalf("already revoked through mux: got %d want 404", rec.Code)
	}
}

// TestCitizenUsageMetersUnderZai pins the metering half of the primitive: a
// usage event recorded against the citizen's member binding lands in
// b.usage.ByKind under "zai" — the revenue truth the Inference card renders.
func TestCitizenUsageMetersUnderZai(t *testing.T) {
	t.Setenv("HIVEX_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	b, _ := newCitizenTestBroker(t)

	rec, _ := postCitizen(t, b, `{"weir_handle":"weir-meter"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("provision: got %d", rec.Code)
	}

	// The same path every headless launcher uses to meter a turn.
	b.RecordBotUsage("weir-meter", "", provider.ClaudeUsage{
		InputTokens:  120,
		OutputTokens: 30,
		CostUSD:      0.02,
	})

	b.mu.Lock()
	kind := b.usage.ByKind["zai"]
	inherit := b.usage.ByKind["(inherit)"]
	unknown := b.usage.ByKind["(unknown)"]
	b.mu.Unlock()
	if kind.Requests != 1 || kind.InputTokens != 120 || kind.OutputTokens != 30 || kind.TotalTokens != 150 {
		t.Fatalf("ByKind[zai] = %+v — citizen usage must bill under its zai binding", kind)
	}
	if kind.CostUsd < 0.0199 || kind.CostUsd > 0.0201 {
		t.Fatalf("ByKind[zai].CostUsd = %v want 0.02", kind.CostUsd)
	}
	if inherit.Requests != 0 || unknown.Requests != 0 {
		t.Fatalf("usage leaked outside the zai bucket: inherit=%+v unknown=%+v", inherit, unknown)
	}
}

// TestCitizenProvisionWithoutLedgerStore: when the broker was never launched
// (no gridframe store armed), provisioning fails loudly with 503 instead of
// silently booking nothing — and the half-provisioned citizen is revocable.
func TestCitizenProvisionWithoutLedgerStore(t *testing.T) {
	t.Setenv("HIVEX_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	b := newTestBroker(t)

	rec, out := postCitizen(t, b, `{"weir_handle":"weir-noledder"}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("provision without ledger store: got %d body %v", rec.Code, out)
	}
	if _, leaked := out["key"]; leaked {
		t.Fatal("503 must not hand over the inference key")
	}
	if rec := deleteCitizen(t, b, "weir-noledder"); rec.Code != http.StatusOK {
		t.Fatalf("cleanup revoke: got %d body %s", rec.Code, rec.Body.String())
	}
}
