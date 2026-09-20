package team

// policy_engine_cel_test.go — pins the CEL data-driven policy layer: rules
// evaluated before the class defaults, deny before allow, irreversible never
// consulted, fail-closed compile, and persistence of the rule estate.

import (
	"strings"
	"testing"
)

func mustAddRule(t *testing.T, b *Broker, expression, effect string) policyRule {
	t.Helper()
	r, err := b.addPolicyRule(policyRule{Expression: expression, Effect: effect})
	if err != nil {
		t.Fatalf("addPolicyRule(%q, %s): %v", expression, effect, err)
	}
	return r
}

// TestPolicyRuleAllowBypassesGrantNeed pins the core adaptation value: an
// operator rule can let a declared mutating capability proceed without a
// standing grant, with the decision attributed to the rule.
func TestPolicyRuleAllowBypassesGrantNeed(t *testing.T) {
	b := newTestBroker(t)
	d := b.EvaluateCapability("researcher", "memory.write", "")
	if d.Decision != "approve" {
		t.Fatalf("precondition: memory.write needs a human without a rule, got %q", d.Decision)
	}
	mustAddRule(t, b, `agent == "researcher" && capability == "memory.write"`, policyRuleEffectAllow)
	d = b.EvaluateCapability("researcher", "memory.write", "")
	if d.Decision != "allow" || !strings.Contains(d.Reason, "policy rule") {
		t.Fatalf("rule must allow, got %q (%q)", d.Decision, d.Reason)
	}
	// Other bots are untouched by the rule.
	d = b.EvaluateCapability("writer", "memory.write", "")
	if d.Decision != "approve" {
		t.Fatalf("rule must not widen to other agents, got %q", d.Decision)
	}
}

// TestPolicyRuleDenyBeatsAllowAndGrants pins deny-before-allow and the rule
// precedence over standing grants: a deny forces the human gate even when a
// grant or an allow rule would have covered the call.
func TestPolicyRuleDenyBeatsAllowAndGrants(t *testing.T) {
	b := newTestBroker(t)
	if _, err := b.addPolicyGrant(policyGrant{BotSlug: "researcher", Capability: "net.http.request", Scope: "api.good.test"}); err != nil {
		t.Fatalf("grant: %v", err)
	}
	mustAddRule(t, b, `capability == "net.http.request" && scope.contains("api.evil.test")`, policyRuleEffectAllow)
	deny := mustAddRule(t, b, `scope.contains("api.evil.test")`, policyRuleEffectDeny)

	d := b.EvaluateCapability("researcher", "net.http.request", "api.evil.test")
	if d.Decision != "approve" || !strings.Contains(d.Reason, deny.ID) {
		t.Fatalf("deny rule must force the human gate despite grant+allow, got %q (%q)", d.Decision, d.Reason)
	}
	// The same capability outside the deny scope flows through the grant.
	d = b.EvaluateCapability("researcher", "net.http.request", "api.good.test")
	if d.Decision != "allow" {
		t.Fatalf("unaffected scope must proceed via grant, got %q (%q)", d.Decision, d.Reason)
	}
}

// TestPolicyRulesNeverTouchIrreversible pins the invariant: irreversible
// capabilities are human decisions ALWAYS — neither grants nor CEL rules
// (nor both) can pre-authorize them.
func TestPolicyRulesNeverTouchIrreversible(t *testing.T) {
	b := newTestBroker(t)
	mustAddRule(t, b, `capability == "spend.authorize"`, policyRuleEffectAllow)
	d := b.EvaluateCapability("researcher", "spend.authorize", "")
	if d.Decision != "approve" || !strings.Contains(d.Reason, "irreversible") {
		t.Fatalf("irreversible must stay human despite an allow rule, got %q (%q)", d.Decision, d.Reason)
	}
}

// TestPolicyRuleCompileFailsClosed pins the fail-closed compile: unparseable
// expressions and non-boolean results are rejected at creation, never stored.
func TestPolicyRuleCompileFailsClosed(t *testing.T) {
	b := newTestBroker(t)
	for _, bad := range []string{"", "agent ==", "agent == 1 && 2", `capability.startsWith("x"`, "1 + 1"} {
		if _, err := b.addPolicyRule(policyRule{Expression: bad, Effect: policyRuleEffectAllow}); err == nil {
			t.Fatalf("expression %q must be rejected", bad)
		}
	}
	if _, err := b.addPolicyRule(policyRule{Expression: `agent`, Effect: policyRuleEffectAllow}); err == nil {
		t.Fatal("non-bool expression must be rejected")
	}
	if _, err := b.addPolicyRule(policyRule{Expression: "true", Effect: "maybe"}); err == nil {
		t.Fatal("unknown effect must be rejected")
	}
}

// TestPolicyRuleRevokeMakesItInert pins the revocation path: a revoked rule
// no longer matches, restoring the default posture.
func TestPolicyRuleRevokeMakesItInert(t *testing.T) {
	b := newTestBroker(t)
	r := mustAddRule(t, b, `capability == "memory.write"`, policyRuleEffectDeny)
	if d := b.EvaluateCapability("researcher", "memory.write", ""); d.Decision == "allow" && strings.Contains(d.Reason, "policy rule") {
		t.Fatalf("deny rule must gate first, got %+v", d)
	}
	if err := b.revokePolicyRule(r.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	d := b.EvaluateCapability("researcher", "memory.write", "")
	if d.Decision != "approve" || !strings.Contains(d.Reason, "no active grant") {
		t.Fatalf("revoked rule must fall through to the default grant posture, got %q (%q)", d.Decision, d.Reason)
	}
}

// TestPolicyRuleDenyNarrowsOnlyMatchingAgents pins scoping: a deny matching
// one agent gates exactly that agent; others fall through to the default
// posture untouched — rules only narrow, never widen.
func TestPolicyRuleDenyNarrowsOnlyMatchingAgents(t *testing.T) {
	b := newTestBroker(t)
	mustAddRule(t, b, `agent == "intern"`, policyRuleEffectDeny)
	if got := b.EvaluateCapability("intern", "memory.write", "").Decision; got != "approve" {
		t.Fatalf("deny on intern must gate memory.write, got %q", got)
	}
	d := b.EvaluateCapability("researcher", "memory.write", "")
	if d.Decision != "approve" || !strings.Contains(d.Reason, "no active grant") {
		t.Fatalf("non-matching agent unaffected, got %q (%q)", d.Decision, d.Reason)
	}
}

// TestAuditProvenance pins the openbot audit-schema adaptation: policy gate
// answers audit with initiator provenance (routine + the bot slug), and
// RecordDecisionAs carries explicit person/deployment provenance.
func TestAuditProvenance(t *testing.T) {
	b := newTestBroker(t)
	// A policy gate answer audits as a routine initiated by the bot.
	b.auditPolicyDecision("researcher", "memory.write", "", PolicyDecision{Decision: "approve", Reason: "no active grant"})
	rec := b.decisions[len(b.decisions)-1]
	if rec.InitiatorKind != "routine" || rec.ActorID != "researcher" {
		t.Fatalf("policy decision provenance = (%q, %q), want (routine, researcher)", rec.InitiatorKind, rec.ActorID)
	}
	// An explicitly-attributed decision carries its own provenance.
	if _, err := b.RecordDecisionAs("approval", "general", "human approved", "", "owner-1", "person", "user-7", nil, true, false); err != nil {
		t.Fatalf("RecordDecisionAs: %v", err)
	}
	rec = b.decisions[len(b.decisions)-1]
	if rec.InitiatorKind != "person" || rec.ActorID != "user-7" {
		t.Fatalf("explicit provenance = (%q, %q), want (person, user-7)", rec.InitiatorKind, rec.ActorID)
	}
}
