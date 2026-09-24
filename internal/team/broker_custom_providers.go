package team

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Northlatch-Labs-LLC/hivex/internal/config"
	"github.com/Northlatch-Labs-LLC/hivex/internal/provider"
)

// customProvidersMu serializes every config Load→mutate→Save cycle in these
// handlers. Without it two concurrent adds race on the same config file and
// one silently drops the other's write.
var customProvidersMu sync.Mutex

// customProviderView is everything these handlers may put on the wire — all
// provider fields except the key itself. The key never leaves the process
// over HTTP; callers that need it authenticated send a blank api_key and the
// broker substitutes the stored one (update keeps the old key when the field
// is empty, test resolves it by provider id).
type customProviderView struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	Enabled   bool   `json:"enabled"`
	APIKeySet bool   `json:"api_key_set"`
}

func maskedProviders(cps []config.CustomProvider) []customProviderView {
	views := make([]customProviderView, 0, len(cps))
	for _, cp := range cps {
		views = append(views, customProviderView{
			ID: cp.ID, Name: cp.Name, BaseURL: cp.BaseURL,
			Model: cp.Model, Enabled: cp.Enabled, APIKeySet: cp.APIKey != "",
		})
	}
	return views
}

// handleCustomProviders manages user-defined OpenAI-compatible providers from
// Settings: GET lists, POST adds, PUT updates, DELETE removes, and
// action=test pings the endpoint's /models with the stored key.
func (b *Broker) handleCustomProviders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, _ := config.Load()
		writeJSON(w, http.StatusOK, map[string]any{"providers": maskedProviders(cfg.CustomProviders)})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (b *Broker) handleCustomProviderAdd(w http.ResponseWriter, r *http.Request) {
	cp, ok := decodeCustomProvider(w, r)
	if !ok {
		return
	}
	customProvidersMu.Lock()
	defer customProvidersMu.Unlock()
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
	customProvidersMu.Lock()
	defer customProvidersMu.Unlock()
	cfg, _ := config.Load()
	for i := range cfg.CustomProviders {
		if cfg.CustomProviders[i].ID == cp.ID {
			cp.ID = cfg.CustomProviders[i].ID
			// Blank key on update means "keep the stored one" — the wire
			// format no longer carries keys, so callers cannot round-trip
			// what they never received.
			if strings.TrimSpace(cp.APIKey) == "" {
				cp.APIKey = cfg.CustomProviders[i].APIKey
			}
			cfg.CustomProviders[i] = cp
			saveCustomProviders(w, cfg)
			return
		}
	}
	http.Error(w, "provider not found", http.StatusNotFound)
}

func (b *Broker) handleCustomProviderDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/custom-providers/delete/")
	customProvidersMu.Lock()
	defer customProvidersMu.Unlock()
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
		ID      string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.BaseURL) == "" {
		http.Error(w, "base_url required", http.StatusBadRequest)
		return
	}
	// The form never sees the stored key (responses mask it), so an empty
	// key plus a provider id means "test with what you have on file".
	key := strings.TrimSpace(body.APIKey)
	if key == "" && strings.TrimSpace(body.ID) != "" {
		if cfg, err := config.Load(); err == nil {
			for _, cp := range cfg.CustomProviders {
				if cp.ID == body.ID {
					key = strings.TrimSpace(cp.APIKey)
					break
				}
			}
		}
	}
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, strings.TrimRight(body.BaseURL, "/")+"/models", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
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
	writeJSON(w, http.StatusOK, map[string]any{"providers": maskedProviders(cfg.CustomProviders)})
}
