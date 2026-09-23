package team

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Northlatch-Labs-LLC/hivex/internal/config"
	"github.com/Northlatch-Labs-LLC/hivex/internal/provider"
)

// handleCustomProviders manages user-defined OpenAI-compatible providers from
// Settings: GET lists, POST adds, PUT updates, DELETE removes, and
// action=test pings the endpoint's /models with the stored key.
func (b *Broker) handleCustomProviders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, _ := config.Load()
		writeJSON(w, http.StatusOK, map[string]any{"providers": cfg.CustomProviders})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (b *Broker) handleCustomProviderAdd(w http.ResponseWriter, r *http.Request) {
	cp, ok := decodeCustomProvider(w, r)
	if !ok {
		return
	}
	cfg, _ := config.Load()
	cfg.CustomProviders = append(cfg.CustomProviders, cp)
	if err := config.ValidateCustomProviders(cfg.CustomProviders); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	saveCustomProviders(w, cfg)
}

func (b *Broker) handleCustomProviderUpdate(w http.ResponseWriter, r *http.Request) {
	cp, ok := decodeCustomProvider(w, r)
	if !ok {
		return
	}
	cfg, _ := config.Load()
	for i := range cfg.CustomProviders {
		if cfg.CustomProviders[i].ID == cp.ID {
			cp.ID = cfg.CustomProviders[i].ID
			cfg.CustomProviders[i] = cp
			saveCustomProviders(w, cfg)
			return
		}
	}
	http.Error(w, "provider not found", http.StatusNotFound)
}

func (b *Broker) handleCustomProviderDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/custom-providers/delete/")
	cfg, _ := config.Load()
	kept := cfg.CustomProviders[:0]
	for _, cp := range cfg.CustomProviders {
		if cp.ID != id {
			kept = append(kept, cp)
		}
	}
	cfg.CustomProviders = kept
	saveCustomProviders(w, cfg)
}

func (b *Broker) handleCustomProviderTest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.BaseURL) == "" {
		http.Error(w, "base_url required", http.StatusBadRequest)
		return
	}
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, strings.TrimRight(body.BaseURL, "/")+"/models", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if k := strings.TrimSpace(body.APIKey); k != "" {
		req.Header.Set("Authorization", "Bearer "+k)
	}
	resp, err := client.Do(req)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	defer resp.Body.Close()
	writeJSON(w, http.StatusOK, map[string]any{"ok": resp.StatusCode < 500, "status": resp.StatusCode})
}

func decodeCustomProvider(w http.ResponseWriter, r *http.Request) (config.CustomProvider, bool) {
	var cp config.CustomProvider
	if err := json.NewDecoder(r.Body).Decode(&cp); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return cp, false
	}
	cp.ID = strings.TrimSpace(cp.ID)
	cp.Name = strings.TrimSpace(cp.Name)
	cp.BaseURL = strings.TrimSpace(cp.BaseURL)
	cp.Model = strings.TrimSpace(cp.Model)
	if err := config.ValidateCustomProvider(cp); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return cp, false
	}
	return cp, true
}

func saveCustomProviders(w http.ResponseWriter, cfg config.Config) {
	if err := config.Save(cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Re-register so a provider added from Settings is dispatchable on the
	// very next turn — no broker restart.
	provider.RegisterCustomProviders()
	writeJSON(w, http.StatusOK, map[string]any{"providers": cfg.CustomProviders})
}
