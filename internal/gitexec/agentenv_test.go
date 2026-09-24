package gitexec

import (
	"strings"
	"testing"
)

func TestAgentEnvAllowslistDropsMachineCredentials(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "sk-or-victim")
	t.Setenv("CLOUDFLARE_API_TOKEN", "cfat-victim")
	t.Setenv("FIGMA_TOKEN", "figd-victim")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-victim")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "aws-victim")
	t.Setenv("HIVEX_BROKER_TOKEN", "keep-me")
	t.Setenv("LC_CTYPE", "UTF-8")
	env := AgentEnv()
	joined := strings.Join(env, "\n")
	for _, leak := range []string{"sk-or-victim", "cfat-victim", "figd-victim", "sk-ant-victim", "aws-victim"} {
		if strings.Contains(joined, leak) {
			t.Fatalf("agent env leaked %q", leak)
		}
	}
	if !strings.Contains(joined, "HIVEX_BROKER_TOKEN=keep-me") {
		t.Fatal("HIVEX_* must pass through")
	}
	if !strings.Contains(joined, "LC_CTYPE=UTF-8") {
		t.Fatal("LC_* must pass through")
	}
	hasPath := false
	for _, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			hasPath = true
		}
	}
	if !hasPath {
		t.Fatal("PATH must survive")
	}
}
