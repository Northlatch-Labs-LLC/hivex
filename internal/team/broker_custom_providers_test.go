package team

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
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

// The wire format never carries provider keys: list/add/update/delete
// responses replace the key with api_key_set, update keeps the stored key
// when the request omits it, and test authenticates by provider id.
func TestCustomProvidersWireNeverCarriesKeys(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HIVEX_CONFIG_PATH", filepath.Join(dir, "config.json"))
	var b Broker

	add := func(rec *httptest.ResponseRecorder) {
		body, _ := json.Marshal(map[string]any{"id": "custom-sec", "name": "Sec", "base_url": "https://api.sec/v1", "model": "m", "api_key": "sk-live-never-on-wire", "enabled": true})
		b.handleCustomProviderAdd(rec, httptest.NewRequest(http.MethodPost, "/custom-providers/add", bytes.NewReader(body)))
	}
	rec := httptest.NewRecorder()
	add(rec)
	if rec.Code != http.StatusOK {
		t.Fatalf("add: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "sk-live-never-on-wire") {
		t.Fatal("add response leaked the api key")
	}
	if !strings.Contains(rec.Body.String(), `"api_key_set":true`) {
		t.Fatalf("add response should flag api_key_set: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	b.handleCustomProviders(rec, httptest.NewRequest(http.MethodGet, "/custom-providers", nil))
	if strings.Contains(rec.Body.String(), "sk-live-never-on-wire") {
		t.Fatal("list response leaked the api key")
	}

	// Update without a key keeps the stored one.
	body, _ := json.Marshal(map[string]any{"id": "custom-sec", "name": "Sec2", "base_url": "https://api.sec/v1", "model": "m2", "enabled": true})
	rec = httptest.NewRecorder()
	b.handleCustomProviderUpdate(rec, httptest.NewRequest(http.MethodPut, "/custom-providers/update", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "sk-live-never-on-wire") {
		t.Fatal("update response leaked the api key")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CustomProviders[0].APIKey != "sk-live-never-on-wire" {
		t.Fatalf("blank-key update must preserve the stored key, got %q", cfg.CustomProviders[0].APIKey)
	}
	if cfg.CustomProviders[0].Model != "m2" {
		t.Fatalf("update should apply the non-key fields, got %+v", cfg.CustomProviders[0])
	}
}

// An empty-key test call with a provider id must authenticate with the
// stored key — the form can never send what it never received.
func TestCustomProviderTestUsesStoredKeyByID(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HIVEX_CONFIG_PATH", filepath.Join(dir, "config.json"))
	var b Broker

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	body, _ := json.Marshal(map[string]any{"id": "custom-sec", "name": "Sec", "base_url": srv.URL, "model": "m", "api_key": "sk-stored", "enabled": true})
	rec := httptest.NewRecorder()
	b.handleCustomProviderAdd(rec, httptest.NewRequest(http.MethodPost, "/custom-providers/add", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("add: %d", rec.Code)
	}

	testBody, _ := json.Marshal(map[string]any{"base_url": srv.URL, "id": "custom-sec"})
	rec = httptest.NewRecorder()
	b.handleCustomProviderTest(rec, httptest.NewRequest(http.MethodPost, "/custom-providers/test", bytes.NewReader(testBody)))
	if rec.Code != http.StatusOK {
		t.Fatalf("test: %d %s", rec.Code, rec.Body.String())
	}
	if gotAuth != "Bearer sk-stored" {
		t.Fatalf("test must use the stored key via id, got Authorization %q", gotAuth)
	}
}
