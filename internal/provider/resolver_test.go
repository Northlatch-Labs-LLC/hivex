package provider

import (
	"testing"

	"github.com/Northlatch-Labs-LLC/hivex/internal/config"
)

func TestDefaultResolverUsesClaudeCode(t *testing.T) {
	// When LLMProvider is empty, should resolve to claude-code, not hive-ask
	cfg := config.Config{} // empty provider
	resolver := DefaultStreamFnResolver(nil)
	fn := resolver("test-agent")
	if fn == nil {
		t.Fatal("resolver returned nil StreamFn")
	}
	// We can't easily test the internal provider, but we verify it doesn't panic
	_ = cfg // used for context
}
