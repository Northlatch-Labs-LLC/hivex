package team

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Northlatch-Labs-LLC/hivex/internal/config"
	"github.com/Northlatch-Labs-LLC/hivex/internal/provider"
)

// handleZaiKeyVerify makes a real 1-token call to the z.ai Anthropic
// endpoint with the provided key (or the stored one when the request is
// empty) so the operator can prove which key is authoritative before
// trusting it with the company's inference. A successful verify of a
// freshly provided key saves it as zai_api_key — write-only over HTTP,
// surfaced afterwards only as a fingerprint.
func (b *Broker) handleZaiKeyVerify(w http.ResponseWriter, r *http.Request) {
	var body struct {
		APIKey string `json:"api_key"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	key := strings.TrimSpace(body.APIKey)
	fresh := key != ""
	if !fresh {
		key = config.ResolveZaiAPIKey()
	}
	if key == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": false, "error": "no key provided and none stored — paste your Z.ai key first",
		})
		return
	}

	base := strings.TrimSpace(os.Getenv("HIVEX_ZAI_BASE_URL"))
	if base == "" {
		base = provider.ZaiDefaultBaseURL()
	}
	payload, _ := json.Marshal(map[string]any{
		"model":      provider.ZaiDefaultModel(),
		"max_tokens": 16,
		"messages":   []map[string]string{{"role": "user", "content": "key check"}},
	})
	start := time.Now()
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost,
		strings.TrimRight(base, "/")+"/v1/messages", bytes.NewReader(payload))
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", key)
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("anthropic-version", "2023-06-01")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	latencyMs := time.Since(start).Milliseconds()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": false, "error": err.Error(), "latency_ms": latencyMs,
		})
		return
	}
	defer resp.Body.Close()

	out := map[string]any{
		"ok":          resp.StatusCode >= 200 && resp.StatusCode < 300,
		"status":      resp.StatusCode,
		"latency_ms":  latencyMs,
		"model":       provider.ZaiDefaultModel(),
		"fingerprint": config.KeyFingerprint(key),
		"saved":       false,
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		out["error"] = "z.ai rejected this key (invalid or revoked)"
	case resp.StatusCode == http.StatusPaymentRequired:
		out["error"] = "key accepted but the plan has no quota left"
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		out["error"] = "unexpected response from z.ai"
	}
	if !fresh || out["ok"] != true {
		writeJSON(w, http.StatusOK, out)
		return
	}

	customProvidersMu.Lock()
	defer customProvidersMu.Unlock()
	cfg, err := config.Load()
	if err != nil {
		out["error"] = "verified, but saving failed: " + err.Error()
		writeJSON(w, http.StatusOK, out)
		return
	}
	cfg.ZaiAPIKey = key
	if err := config.Save(cfg); err != nil {
		out["error"] = "verified, but saving failed: " + err.Error()
		writeJSON(w, http.StatusOK, out)
		return
	}
	out["saved"] = true
	writeJSON(w, http.StatusOK, out)
}
