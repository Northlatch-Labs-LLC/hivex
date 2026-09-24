package provider

import (
	"os"
	"path/filepath"
	"testing"
)

// The zai kind must be a first-class binding target: valid as a kind,
// resolvable for keys env-first then config, with the documented defaults.
func TestZaiKindIsBindable(t *testing.T) {
	if err := ValidateKind(KindZAI); err != nil {
		t.Fatalf("ValidateKind(zai): %v", err)
	}
	if _, ok := registry[KindZAI]; !ok {
		t.Fatal("zai entry must be registered at init")
	}
}

func TestResolveZaiAPIKeyEnvWinsOverConfig(t *testing.T) {
	t.Setenv("HIVEX_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	t.Setenv("HIVEX_ZAI_API_KEY", "env-key")
	if got := resolveOpenAICompatAPIKey(KindZAI); got != "env-key" {
		t.Fatalf("env key must win, got %q", got)
	}
}

func TestResolveZaiAPIKeyFromConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"zai_api_key":"cfg-key"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HIVEX_CONFIG_PATH", path)
	t.Setenv("HIVEX_ZAI_API_KEY", "")
	if got := resolveOpenAICompatAPIKey(KindZAI); got != "cfg-key" {
		t.Fatalf("config key must resolve, got %q", got)
	}
}
