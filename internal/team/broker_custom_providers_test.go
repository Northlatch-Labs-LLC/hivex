package team

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Northlatch-Labs-LLC/hivex/internal/config"
)

// The custom-provider handlers only touch config + http, so a zero-value
// Broker is enough to drive them directly.
func TestCustomProviderHandlersRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HIVEX_CONFIG_PATH", filepath.Join(dir, "config.json"))
	var b Broker

	cp := map[string]any{"id": "custom-test", "name": "Test", "base_url": "https://api.test/v1", "model": "m1", "api_key": "k", "enabled": true}
	body, _ := json.Marshal(cp)
	rec := httptest.NewRecorder()
	b.handleCustomProviderAdd(rec, httptest.NewRequest(http.MethodPost, "/custom-providers/add", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("add: %d %s", rec.Code, rec.Body.String())
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.CustomProviders) != 1 || cfg.CustomProviders[0].ID != "custom-test" {
		t.Fatalf("want 1 custom-test provider, got %+v", cfg.CustomProviders)
	}

	rec = httptest.NewRecorder()
	b.handleCustomProviders(rec, httptest.NewRequest(http.MethodGet, "/custom-providers", nil))
	var out struct {
		Providers []config.CustomProvider `json:"providers"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Providers) != 1 || out.Providers[0].Name != "Test" {
		t.Fatalf("list: %+v", out.Providers)
	}

	rec = httptest.NewRecorder()
	b.handleCustomProviderDelete(rec, httptest.NewRequest(http.MethodDelete, "/custom-providers/delete/custom-test", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	cfg, _ = config.Load()
	if len(cfg.CustomProviders) != 0 {
		t.Fatalf("want empty after delete, got %+v", cfg.CustomProviders)
	}
}
