package team

import (
	"testing"

	"github.com/Northlatch-Labs-LLC/hivex/internal/brokeraddr"
)

// nameWithPortSuffix decides the tmux socket and session names at package
// init based on the broker port. When two hivebot instances ran on the same
// machine they used to share "hivex" and "hivex-team" and race each other's
// kill-session / new-session / split-window calls, which surfaced as
// "spawn first bot: exit status 1" when the server was torn down
// mid-launch. These tests pin the rule so the isolation can't regress
// silently.

func TestNameWithPortSuffixDefaultPort(t *testing.T) {
	if got := nameWithPortSuffixForPort("hivex", brokeraddr.DefaultPort); got != "hivex" {
		t.Fatalf("default port should not suffix: got %q, want %q", got, "hivex")
	}
	if got := nameWithPortSuffixForPort("hivex-team", brokeraddr.DefaultPort); got != "hivex-team" {
		t.Fatalf("default port should not suffix session: got %q, want %q", got, "hivex-team")
	}
}

func TestNameWithPortSuffixNonDefault(t *testing.T) {
	cases := []struct {
		base string
		port int
		want string
	}{
		{"hivex", 7899, "hivex-7899"},
		{"hivex-team", 7899, "hivex-team-7899"},
		{"hivex", 8080, "hivex-8080"},
	}
	for _, tc := range cases {
		if got := nameWithPortSuffixForPort(tc.base, tc.port); got != tc.want {
			t.Fatalf("port %d base %q: got %q, want %q", tc.port, tc.base, got, tc.want)
		}
	}
}

func TestNameWithPortSuffixInvalidPortFallsBack(t *testing.T) {
	if got := nameWithPortSuffixForPort("hivex", 0); got != "hivex" {
		t.Fatalf("zero port should fall back: got %q", got)
	}
	if got := nameWithPortSuffixForPort("hivex", -1); got != "hivex" {
		t.Fatalf("negative port should fall back: got %q", got)
	}
}

// TestPackageLevelNamesHonorBaseNames guards against someone inadvertently
// changing the base constants in a way that leaks the port suffix into
// external consumers that hardcode "hivex-team".
func TestPackageLevelNamesHonorBaseNames(t *testing.T) {
	if baseSessionName != "hivex-team" {
		t.Fatalf("baseSessionName drifted: got %q", baseSessionName)
	}
	if baseTmuxSocketName != "hivex" {
		t.Fatalf("baseTmuxSocketName drifted: got %q", baseTmuxSocketName)
	}
}
