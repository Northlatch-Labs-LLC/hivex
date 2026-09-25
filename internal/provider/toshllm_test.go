package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Northlatch-Labs-LLC/hivex/internal/bot"
)

// The ToshLLM engine on :8080 speaks OpenAI chat completions but lists
// models in its own shape with full-path ids. A kind with no pinned model
// must resolve the engine's loaded model and stream a turn through it.
func TestToshllmAutoResolvesLoadedModel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HIVEX_CONFIG_PATH", path)

	var gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/models":
			_, _ = w.Write([]byte(`{"models":[{"model":"/models/qwen2.5-coder-7b.gguf","name":"qwen2.5-coder"}]}`))
		case r.URL.Path == "/v1/chat/completions":
			var body struct {
				Model string `json:"model"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			gotModel = body.Model
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"metal \"}}]}\n\n"))
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ready\"}}]}\n\n"))
			_, _ = w.Write([]byte("data: {\"choices\":[{\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2}}\n\n"))
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
		}
	}))
	defer srv.Close()
	t.Setenv("HIVEX_TOSHLLM_BASE_URL", srv.URL+"/v1")

	if err := ValidateKind(KindToshllm); err != nil {
		t.Fatalf("ValidateKind: %v", err)
	}
	if !IsHeadlessHTTPKind(KindToshllm) {
		t.Fatal("toshllm must be a headless HTTP runtime")
	}
	ch := NewStreamFnFor(context.Background(), KindToshllm, "", "local")(
		[]bot.Message{{Role: "user", Content: "status?"}}, nil)
	var text string
	for c := range ch {
		if c.Type == "error" {
			t.Fatalf("stream error: %s", c.Content)
		}
		if c.Type == "text" {
			text += c.Content
		}
	}
	if !strings.Contains(text, "metal ready") {
		t.Fatalf("text = %q", text)
	}
	if gotModel != "/models/qwen2.5-coder-7b.gguf" {
		t.Fatalf("auto-resolved model must be the engine's loaded model, got %q", gotModel)
	}
}
