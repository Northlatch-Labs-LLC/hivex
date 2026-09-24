package team

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Northlatch-Labs-LLC/hivex/internal/config"
)

// The verify flow proves a key against the real endpoint shape, saves it
// only when the operator provided it fresh AND z.ai accepted it, and never
// echoes the key itself.
func TestZaiKeyVerifyFlow(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	t.Setenv("HIVEX_CONFIG_PATH", cfgPath)

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("x-api-key")
		if strings.HasSuffix(gotAuth, "-bad") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"type":"message","model":"GLM-5.3","content":[]}`))
	}))
	defer srv.Close()
	t.Setenv("HIVEX_ZAI_BASE_URL", srv.URL)

	var b Broker
	post := func(body string) map[string]any {
		rec := httptest.NewRecorder()
		b.handleZaiKeyVerify(rec, httptest.NewRequest(http.MethodPost, "/zai-key/verify", strings.NewReader(body)))
		var out map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return out
	}

	// Fresh valid key → ok + saved.
	out := post(`{"api_key":"zk-fresh-key-0123456789abcdef"}`)
	if out["ok"] != true || out["saved"] != true {
		t.Fatalf("valid fresh key: %+v", out)
	}
	if gotAuth != "zk-fresh-key-0123456789abcdef" {
		t.Fatalf("verify must authenticate with the provided key, got %q", gotAuth)
	}
	cfg, _ := config.Load()
	if cfg.ZaiAPIKey != "zk-fresh-key-0123456789abcdef" {
		t.Fatal("verified key must be saved")
	}
	if s, _ := out["fingerprint"].(string); !strings.Contains(s, "…") {
		t.Fatalf("fingerprint must be masked, got %q", s)
	}

	// Rejected key → not ok, not saved, key never echoed.
	before := cfg.ZaiAPIKey
	out = post(`{"api_key":"zk-bad"}`)
	if out["ok"] == true {
		t.Fatal("401 from z.ai must not verify")
	}
	cfg, _ = config.Load()
	if cfg.ZaiAPIKey != before {
		t.Fatal("rejected key must not be saved")
	}

	// Empty body → verifies the stored key, saves nothing.
	out = post(`{}`)
	if out["ok"] != true || out["saved"] != false {
		t.Fatalf("stored-key verify: %+v", out)
	}
}
