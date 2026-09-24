package team

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Northlatch-Labs-LLC/hivex/internal/config"
	"github.com/Northlatch-Labs-LLC/hivex/internal/provider"
)

// handleProviderModels lists the models a provider kind actually serves, so
// per-agent model pickers offer reality instead of a drifting hardcoded
// catalog. zai queries the GLM Coding Plan's OpenAI-shaped /models; custom
// providers query their own base. 10s timeout, best-effort: errors return
// an empty list, never a 500 — a picker must still open.
func (b *Broker) handleProviderModels(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	out := map[string]any{"kind": kind, "models": []string{}}
	if kind == "" {
		writeJSON(w, http.StatusOK, out)
		return
	}

	var base, key string
	switch {
	case kind == provider.KindZAI:
		base = provider.ZaiDefaultBaseURL()
		// The coding-plan catalog is OpenAI-shaped; the Anthropic base
		// serves /v1/messages, so swap the suffix for model listing.
		// HIVEX_ZAI_MODELS_BASE_URL overrides for exotic plans.
		base = strings.Replace(base, "/api/anthropic", "/api/coding/paas/v4", 1)
		if v := strings.TrimSpace(os.Getenv("HIVEX_ZAI_MODELS_BASE_URL")); v != "" {
			base = v
		}
		key = config.ResolveZaiAPIKey()
	case strings.HasPrefix(kind, config.CustomProviderKindPrefix):
		if cp, err := config.FindCustomProviderByKey(kind); err == nil {
			base, key = cp.BaseURL, cp.APIKey
		}
	default:
		writeJSON(w, http.StatusOK, out)
		return
	}
	if strings.TrimSpace(base) == "" {
		writeJSON(w, http.StatusOK, out)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet,
		strings.TrimRight(base, "/")+"/models", nil)
	if err != nil {
		writeJSON(w, http.StatusOK, out)
		return
	}
	if strings.TrimSpace(key) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(key))
	}
	resp, err := client.Do(req)
	if err != nil {
		writeJSON(w, http.StatusOK, out)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeJSON(w, http.StatusOK, out)
		return
	}
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if json.Unmarshal(raw, &body) != nil {
		writeJSON(w, http.StatusOK, out)
		return
	}
	models := make([]string, 0, len(body.Data))
	for _, m := range body.Data {
		if id := strings.TrimSpace(m.ID); id != "" {
			models = append(models, id)
		}
	}
	out["models"] = models
	writeJSON(w, http.StatusOK, out)
}
