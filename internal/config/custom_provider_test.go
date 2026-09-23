package config

import (
	"encoding/json"
	"testing"
)

func validCustomProvider() CustomProvider {
	return CustomProvider{
		ID:      "custom-groq-fast",
		Name:    "Groq Fast",
		BaseURL: "http://127.0.0.1:9000/v1",
		Model:   "llama-3",
		APIKey:  "sk-test",
		Enabled: true,
	}
}

// TestValidateCustomProvider_AcceptsWellFormedEntry verifies the happy path.
func TestValidateCustomProvider_AcceptsWellFormedEntry(t *testing.T) {
	if err := ValidateCustomProvider(validCustomProvider()); err != nil {
		t.Fatalf("well-formed provider rejected: %v", err)
	}
}

// TestValidateCustomProvider_RejectsBadEntries is a table over each
// rejection the validator is responsible for.
func TestValidateCustomProvider_RejectsBadEntries(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*CustomProvider)
	}{
		{"missing prefix", func(cp *CustomProvider) { cp.ID = "groq-fast" }},
		{"empty slug", func(cp *CustomProvider) { cp.ID = "custom-" }},
		{"edge dash slug", func(cp *CustomProvider) { cp.ID = "custom-groq-" }},
		{"uppercase slug", func(cp *CustomProvider) { cp.ID = "custom-Groq" }},
		{"slug with space", func(cp *CustomProvider) { cp.ID = "custom-groq fast" }},
		{"empty name", func(cp *CustomProvider) { cp.Name = "" }},
		{"whitespace name", func(cp *CustomProvider) { cp.Name = "   " }},
		{"empty model", func(cp *CustomProvider) { cp.Model = "" }},
		{"non-http scheme", func(cp *CustomProvider) { cp.BaseURL = "ftp://host/v1" }},
		{"missing host", func(cp *CustomProvider) { cp.BaseURL = "http:///v1" }},
		{"unparseable url", func(cp *CustomProvider) { cp.BaseURL = "http://[::1" }},
		{"empty base url", func(cp *CustomProvider) { cp.BaseURL = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cp := validCustomProvider()
			tc.mut(&cp)
			if err := ValidateCustomProvider(cp); err == nil {
				t.Errorf("invalid entry accepted: %+v", cp)
			}
		})
	}
}

// TestValidateCustomProviders_RejectsDuplicateIDs verifies the list-level
// uniqueness check.
func TestValidateCustomProviders_RejectsDuplicateIDs(t *testing.T) {
	cps := []CustomProvider{validCustomProvider(), validCustomProvider()}
	if err := ValidateCustomProviders(cps); err == nil {
		t.Error("duplicate IDs accepted")
	}
	cps[1].ID = "custom-second"
	if err := ValidateCustomProviders(cps); err != nil {
		t.Errorf("distinct IDs rejected: %v", err)
	}
	if err := ValidateCustomProviders(nil); err != nil {
		t.Errorf("nil list rejected: %v", err)
	}
}

// TestCustomProviderJSONTags pins the wire shape: snake_case keys, api_key
// omitted when empty.
func TestCustomProviderJSONTags(t *testing.T) {
	b, err := json.Marshal(validCustomProvider())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"id":"custom-groq-fast","name":"Groq Fast","base_url":"http://127.0.0.1:9000/v1","model":"llama-3","api_key":"sk-test","enabled":true}`
	if string(b) != want {
		t.Errorf("marshal = %s, want %s", b, want)
	}
	noKey := validCustomProvider()
	noKey.APIKey = ""
	b, _ = json.Marshal(noKey)
	if want := `{"id":"custom-groq-fast","name":"Groq Fast","base_url":"http://127.0.0.1:9000/v1","model":"llama-3","enabled":true}`; string(b) != want {
		t.Errorf("marshal without key = %s, want %s", b, want)
	}
}
