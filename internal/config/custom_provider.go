package config

import (
	"fmt"
	"net/url"
	"strings"
)

// CustomProviderKindPrefix marks Kinds from user-defined entries (vs built-in
// mlx-lm/ollama/exo), so IDs double as provider Kinds without collisions.
const CustomProviderKindPrefix = "custom-"

// CustomProvider is one user-defined OpenAI-compatible provider managed from
// Settings — the API-key-carrying counterpart of ProviderEndpoint.
type CustomProvider struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
	APIKey  string `json:"api_key,omitempty"`
	Enabled bool   `json:"enabled"`
}

// isKindSlug reports a non-empty [a-z0-9-] run with no edge '-'.
func isKindSlug(s string) bool {
	if s == "" || s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

// ValidateCustomProvider checks one entry in isolation: required fields, ID
// shape (prefix + kind slug), and an http(s) base URL with a host. Uniqueness
// is checked by ValidateCustomProviders.
func ValidateCustomProvider(cp CustomProvider) error {
	id := strings.TrimSpace(cp.ID)
	slug, ok := strings.CutPrefix(id, CustomProviderKindPrefix)
	if !ok || !isKindSlug(slug) {
		return fmt.Errorf("custom provider id %q: want prefix %q + [a-z0-9-] slug", id, CustomProviderKindPrefix)
	}
	if strings.TrimSpace(cp.Name) == "" {
		return fmt.Errorf("custom provider %s: name is required", id)
	}
	if strings.TrimSpace(cp.Model) == "" {
		return fmt.Errorf("custom provider %s: model is required", id)
	}
	raw := strings.TrimSpace(cp.BaseURL)
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("custom provider base URL %q: %w", raw, err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("custom provider base URL %q: want http(s) URL with host", raw)
	}
	return nil
}

// ValidateCustomProviders validates every entry and rejects duplicate IDs,
// which would make provider-Kind resolution ambiguous.
func ValidateCustomProviders(cps []CustomProvider) error {
	seen := make(map[string]bool, len(cps))
	for _, cp := range cps {
		if err := ValidateCustomProvider(cp); err != nil {
			return err
		}
		if seen[cp.ID] {
			return fmt.Errorf("duplicate custom provider id %q", cp.ID)
		}
		seen[cp.ID] = true
	}
	return nil
}
