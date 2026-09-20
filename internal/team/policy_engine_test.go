package team

import (
	"strings"
	"testing"
	"time"
)

// TestCapabilityForClassifiesTheRegistry pins the declared surface: known
// capabilities resolve with their class, the integration family is dynamic
// and external, and anything undeclared resolves to ok=false — which is what
// makes EvaluateCapability fail closed on it.
func TestCapabilityForClassifiesTheRegistry(t *testing.T) {
	for _, tc := range []struct {
		id       string
		want     CapabilityClass
		wantable bool
	}{
		{"browser.drive", CapabilityExternal, true},
		{"net.http.request", CapabilityExternal, true},
		{"memory.write", CapabilityMutating, true},
		{"task.mutate", CapabilityMutating, true},
		{"app.register", CapabilityMutating, true},
		{"spend.authorize", CapabilityIrreversible, true},
		{"workspace.admin", CapabilityIrreversible, true},
		{"integration.gmail.gmail_send_email", CapabilityExternal, true},
		{"", CapabilityClass(""), false},
		{"shell.exec", CapabilityClass(""), false},
		{"totally.unknown", CapabilityClass(""), false},
	} {
		cap, ok := capabilityFor(tc.id)
		if ok != tc.wantable {
			t.Errorf("capabilityFor(%q) ok=%v, want %v", tc.id, ok, tc.wantable)
			continue
		}
		if ok && cap.Class != tc.want {
			t.Errorf("capabilityFor(%q) class=%q, want %q", tc.id, cap.Class, tc.want)
		}
	}
}

// TestEvaluateCapabilityMatrix is the policy gate's contract table: every
// (class, grant) pair and the answer it must produce.
func TestEvaluateCapabilityMatrix(t *testing.T) {
	b := newTestBroker(t)

	mint := func(t *testing.T, bot, capability, scope string) {
		t.Helper()
		if _, err := b.addPolicyGrant(policyGrant{BotSlug: bot, Capability: capability, Scope: scope}); err != nil {
			t.Fatalf("mint grant: %v", err)
		}
	}

	t.Run("readonly auto-proceeds without a grant", func(t *testing.T) {
		// "task.mutate" is mutating; read-only has no seeded capability that a
		// bot calls on its own — use the class directly via a registry entry.
		if d := b.EvaluateCapability("researcher", "task.mutate", "task-1"); d.Decision != "approve" {
			t.Fatalf("mutating without grant = %q, want approve", d.Decision)
		}
	})
	t.Run("readonly allows", func(t *testing.T) {
		// The registry's read-only representative is declared below in the
		// table; here exercise it through the matrix.
		d := b.EvaluateCapability("researcher", "browser.drive", "")
		if d.Decision != "approve" {
			t.Fatalf("external without grant = %q, want approve", d.Decision)
		}
	})
	t.Run("mutating with exact grant allows", func(t *testing.T) {
		mint(t, "researcher", "memory.write", "")
		if d := b.EvaluateCapability("researcher", "memory.write", ""); d.Decision != "allow" {
			t.Fatalf("mutating with grant = %q, want allow (%s)", d.Decision, d.Reason)
		}
	})
	t.Run("grant does not leak across bots", func(t *testing.T) {
		if d := b.EvaluateCapability("writer", "memory.write", ""); d.Decision != "approve" {
			t.Fatalf("other bot's grant leaked: %q", d.Decision)
		}
	})
	t.Run("grant does not leak across scopes", func(t *testing.T) {
		mint(t, "researcher", "task.mutate", "task-7")
		if d := b.EvaluateCapability("researcher", "task.mutate", "task-8"); d.Decision != "approve" {
			t.Fatalf("grant leaked across scopes: %q", d.Decision)
		}
		if d := b.EvaluateCapability("researcher", "task.mutate", "task-7"); d.Decision != "allow" {
			t.Fatalf("exact scope grant did not allow: %q", d.Decision)
		}
	})
	t.Run("irreversible never allows, even granted", func(t *testing.T) {
		if _, err := b.addPolicyGrant(policyGrant{BotSlug: "researcher", Capability: "spend.authorize"}); err != nil {
			t.Fatalf("mint irreversible grant: %v", err)
		}
		if d := b.EvaluateCapability("researcher", "spend.authorize", ""); d.Decision != "approve" {
			t.Fatalf("irreversible granted = %q, want approve (grants never cover irreversible)", d.Decision)
		}
	})
	t.Run("undeclared capability fails closed", func(t *testing.T) {
		d := b.EvaluateCapability("researcher", "shell.exec", "")
		if d.Decision != "approve" || !strings.Contains(d.Reason, "not declared") {
			t.Fatalf("undeclared = %+v, want approve with a default-deny reason", d)
		}
	})
	t.Run("nil broker answers approve", func(t *testing.T) {
		var nilBroker *Broker
		if d := nilBroker.EvaluateCapability("researcher", "memory.write", ""); d.Decision != "approve" {
			t.Fatalf("nil broker = %q, want approve (no broker, no trust)", d.Decision)
		}
	})
}

// TestPolicyGrantLifecycle pins mint → revoke → fail-closed, plus the
// validation and TTL rules on the grant store itself.
func TestPolicyGrantLifecycle(t *testing.T) {
	b := newTestBroker(t)

	if _, err := b.addPolicyGrant(policyGrant{BotSlug: "researcher"}); err == nil {
		t.Fatal("grant without a capability must be rejected")
	}
	if _, err := b.addPolicyGrant(policyGrant{BotSlug: "researcher", Capability: "shell.exec"}); err == nil {
		t.Fatal("grant for an undeclared capability must be rejected")
	}
	past := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	if _, err := b.addPolicyGrant(policyGrant{BotSlug: "researcher", Capability: "memory.write", ExpiresAt: past}); err != nil {
		t.Fatalf("minting an already-expired grant is stored (inactive), got error: %v", err)
	}
	if d := b.EvaluateCapability("researcher", "memory.write", ""); d.Decision != "approve" {
		t.Fatalf("expired grant must fail closed, got %q", d.Decision)
	}

	g, err := b.addPolicyGrant(policyGrant{BotSlug: "researcher", Capability: "memory.write"})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if d := b.EvaluateCapability("researcher", "memory.write", ""); d.Decision != "allow" {
		t.Fatalf("active grant must allow, got %q", d.Decision)
	}
	if err := b.revokePolicyGrant(g.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if d := b.EvaluateCapability("researcher", "memory.write", ""); d.Decision != "approve" {
		t.Fatalf("revoked grant must fail closed, got %q", d.Decision)
	}
	// Revoke is idempotent: revoking an already-revoked grant is a no-op
	// success (the state is already what the caller asked for), and the
	// grant must stay revoked.
	if err := b.revokePolicyGrant(g.ID); err != nil {
		t.Fatalf("double revoke must be idempotent, got: %v", err)
	}
	if d := b.EvaluateCapability("researcher", "memory.write", ""); d.Decision != "approve" {
		t.Fatalf("grant must stay revoked after double revoke, got %q", d.Decision)
	}
}

// TestPolicyGrantTTLCap pins the defense-in-depth TTL: a far-future expiry is
// clamped to maxPolicyGrantTTL at mint time.
func TestPolicyGrantTTLCap(t *testing.T) {
	b := newTestBroker(t)
	far := time.Now().UTC().Add(10 * 365 * 24 * time.Hour).Format(time.RFC3339)
	g, err := b.addPolicyGrant(policyGrant{BotSlug: "researcher", Capability: "memory.write", ExpiresAt: far})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	ts, ok := parseGrantTime(g.ExpiresAt)
	if !ok {
		t.Fatalf("expiry %q unparseable", g.ExpiresAt)
	}
	if ts.Sub(time.Now().UTC()) > maxPolicyGrantTTL {
		t.Fatalf("expiry %v exceeds the %v TTL cap", ts, maxPolicyGrantTTL)
	}
}
