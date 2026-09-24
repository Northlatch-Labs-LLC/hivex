package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Northlatch-Labs-LLC/hivex/internal/bot"
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

// Headless-HTTP routing must be registry-derived via the Transport tag: zai
// speaks the Anthropic messages protocol, the local runtimes speak
// OpenAI-compat, CLI kinds are not headless HTTP at all. A hand-maintained
// list in the turn dispatcher previously fell compat-bound bots to the
// claude runner.
func TestZaiRoutesToHeadlessRunner(t *testing.T) {
	e, ok := registry[KindZAI]
	if !ok || e.Transport != TransportAnthropic {
		t.Fatalf("zai entry must carry Transport=anthropic, got %+v", e)
	}
	for _, kind := range []string{KindZAI, KindHiveAPI, KindOllama, KindMLXLM, KindExo} {
		if !IsHeadlessHTTPKind(kind) {
			t.Errorf("IsHeadlessHTTPKind(%q) = false, want true", kind)
		}
	}
	for _, kind := range []string{KindClaudeCode, KindCodex, KindOpencode, KindZAICode} {
		if IsHeadlessHTTPKind(kind) {
			t.Errorf("IsHeadlessHTTPKind(%q) = true, want false", kind)
		}
	}
}

// Z.ai Code is a CLI-class kind: validated, registered, and NOT routed to
// the headless compat runner — it drives the claude engine instead.
func TestZAICodeIsACLIRuntime(t *testing.T) {
	if err := ValidateKind(KindZAICode); err != nil {
		t.Fatalf("ValidateKind(zai-code): %v", err)
	}
	if IsHeadlessHTTPKind(KindZAICode) {
		t.Fatal("zai-code is a CLI runtime, not a headless HTTP transport")
	}
}

// The zai stream goes through NewStreamFnFor (the transport dispatcher) and
// speaks the Anthropic Messages protocol: POST {base}/v1/messages with
// x-api-key, SSE text deltas, a trailing usage chunk.
func TestZaiAnthropicMessagesStream(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"zai_api_key":"zk-stream"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HIVEX_CONFIG_PATH", path)
	t.Setenv("HIVEX_ZAI_API_KEY", "")

	var gotPath, gotKey, gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotKey = r.URL.Path, r.Header.Get("x-api-key")
		var body struct{ Model string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotModel = body.Model
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"message_start\",\"usage\":{\"input_tokens\":12}}\n\n" +
			"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello \"}}\n\n" +
			"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"GLM\"}}\n\n" +
			"data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":7}}\n\n" +
			"data: {\"type\":\"message_stop\"}\n\n"))
	}))
	defer srv.Close()
	t.Setenv("HIVEX_ZAI_BASE_URL", srv.URL)

	prev := httpClientForOpenAICompat
	httpClientForOpenAICompat = func() *http.Client { return srv.Client() }
	t.Cleanup(func() { httpClientForOpenAICompat = prev })

	msgs := []bot.Message{
		{Role: "system", Content: "You are cos."},
		{Role: "user", Content: "provider check"},
	}
	ch := NewStreamFnFor(context.Background(), KindZAI, "GLM-5.3", "cos")(msgs, nil)
	var text string
	var usage *bot.StreamChunk
	for c := range ch {
		switch c.Type {
		case "text":
			text += c.Content
		case "usage":
			u := c
			usage = &u
		case "error":
			t.Fatalf("stream error: %s", c.Content)
		}
	}
	if text != "Hello GLM" {
		t.Fatalf("text = %q", text)
	}
	if gotPath != "/v1/messages" {
		t.Fatalf("endpoint path = %q, want /v1/messages", gotPath)
	}
	if gotKey != "zk-stream" {
		t.Fatalf("x-api-key = %q", gotKey)
	}
	if gotModel != "GLM-5.3" {
		t.Fatalf("model = %q (binding override must win)", gotModel)
	}
	if usage == nil || usage.InputTokens != 12 || usage.OutputTokens != 7 {
		t.Fatalf("usage = %+v", usage)
	}
}
