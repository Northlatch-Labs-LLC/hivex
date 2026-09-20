package team

// policy_engine.go — default-deny capability grants: the general policy gate
// for EVERY tool surface a bot can drive, not just external integrations.
//
// The shipped model (broker_action_grants.go + deterministic-integrations.md)
// proved the semantics on integrations: a standing grant is EXACT-MATCH
// (bot, capability, scope), fails closed on expiry/revocation/malformed data,
// never widens with wildcards, and carries a TTL cap. This engine
// generalizes that model to the whole capability registry — browser driving,
// HTTP egress, app registration, task mutation, memory writes, spend — and
// adds the classification layer the old gate lacked:
//
//   readonly      auto-proceed (audited) — reading can't burn anything.
//   mutating      grant → proceed; no grant → human approval.
//   external      same as mutating (the old integration behavior), because
//                 data leaves the machine.
//   irreversible  ALWAYS a human decision — a grant can never pre-authorize
//                 spend, or workspace admin, or anything we cannot undo.
//
// DEFAULT-DENY: a capability that is not declared in the registry is treated
// as needing human approval — never silently allowed. Declaring a new
// capability is an explicit, reviewed act. The evaluation result is audited
// into the broker's decision log either way, so every auto-proceed is
// inspectable after the fact.
//
// SECURITY (host-trust model — same as grants): grant CRUD is broker-token
// gated and NO MCP tool reaches /policy/grants, so a bot cannot mint its own
// permissions. Grants are created by the human-driven approval surfaces and
// revoked from the same apps.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// CapabilityClass is the risk tier a capability belongs to. It decides the
// default posture when no grant covers the call.
type CapabilityClass string

const (
	CapabilityReadOnly     CapabilityClass = "readonly"
	CapabilityMutating     CapabilityClass = "mutating"
	CapabilityExternal     CapabilityClass = "external"
	CapabilityIrreversible CapabilityClass = "irreversible"
)

// Capability is one declared, classifiable thing a bot can ask the harness to
// do. The registry is the single place a capability must be declared before
// the policy engine will classify it as anything safer than "ask a human".
type Capability struct {
	ID          string          `json:"id"`
	Class       CapabilityClass `json:"class"`
	Description string          `json:"description"`
}

// capabilityRegistry declares the harness's tool surface. The integration
// family is dynamic (integration.<platform>.<action_id>) and classified
// external; everything else must appear here.
var capabilityRegistry = map[string]Capability{
	"browser.drive":    {ID: "browser.drive", Class: CapabilityExternal, Description: "drive the shared browser (cua) toward a goal"},
	"net.http.request": {ID: "net.http.request", Class: CapabilityExternal, Description: "outbound HTTP egress to any endpoint"},
	"memory.write":     {ID: "memory.write", Class: CapabilityMutating, Description: "write to the organizational memory (wiki, notebook, learnings)"},
	"app.register":     {ID: "app.register", Class: CapabilityMutating, Description: "register or publish a workspace app"},
	"app.edit":         {ID: "app.edit", Class: CapabilityMutating, Description: "edit an existing workspace app"},
	"task.mutate":      {ID: "task.mutate", Class: CapabilityMutating, Description: "create, assign, or transition a team task"},
	"spend.authorize":  {ID: "spend.authorize", Class: CapabilityIrreversible, Description: "authorize payment or commit spend"},
	"workspace.admin":  {ID: "workspace.admin", Class: CapabilityIrreversible, Description: "administer workspaces, members, or credentials"},
}

// capabilityIntegrationPrefix is the dynamic family: any capability starting
// with it names a concrete external integration action
// (integration.gmail.gmail_send_email).
const capabilityIntegrationPrefix = "integration."

// capabilityFor resolves a capability id to its declaration. Integration
// capabilities are classified external without a registry entry; anything
// else unknown returns ok=false so Evaluate can fail closed.
func capabilityFor(id string) (Capability, bool) {
	key := strings.ToLower(strings.TrimSpace(id))
	if key == "" {
		return Capability{}, false
	}
	if strings.HasPrefix(key, capabilityIntegrationPrefix) {
		return Capability{ID: key, Class: CapabilityExternal, Description: "external integration action"}, true
	}
	c, ok := capabilityRegistry[key]
	return c, ok
}

// policyGrant is a standing, human-issued approval for exactly one
// (bot, capability, scope). Mirrors actionGrant semantics: exact match only,
// no wildcards, TTL-capped, fail-closed on malformed data.
type policyGrant struct {
	ID         string `json:"id"`
	BotSlug    string `json:"agent_slug"`
	Capability string `json:"capability"`
	Scope      string `json:"scope,omitempty"`
	GrantedBy  string `json:"granted_by"`
	GrantedAt  string `json:"granted_at"`
	ExpiresAt  string `json:"expires_at,omitempty"`
	RevokedAt  string `json:"revoked_at,omitempty"`
}

// maxPolicyGrantTTL caps standing grants — defense in depth so a forgotten
// grant cannot bypass approval forever. Same window as action grants.
const maxPolicyGrantTTL = 30 * 24 * time.Hour

func policyGrantBotKey(bot string) string      { return strings.ToLower(strings.TrimSpace(bot)) }
func policyGrantCapabilityKey(c string) string { return strings.ToLower(strings.TrimSpace(c)) }
func policyGrantScopeKey(s string) string      { return strings.ToLower(strings.TrimSpace(s)) }

// policyGrantActive reports whether a grant currently authorizes its
// capability: not revoked, not expired, and carrying a parseable expiry when
// one is present. Unparseable expiry fails closed — a malformed grant never
// silently authorizes anything (same rule as actionGrantActive).
func policyGrantActive(g policyGrant, now time.Time) bool {
	if strings.TrimSpace(g.RevokedAt) != "" {
		return false
	}
	if exp := strings.TrimSpace(g.ExpiresAt); exp != "" {
		ts, ok := parseGrantTime(exp)
		if !ok || !now.Before(ts) {
			return false
		}
	}
	return true
}

// PolicyDecision is the policy gate's answer: allow (proceed), or
// approve (a human must decide). There is no "deny" state at this layer —
// the human's rejection IS the deny.
type PolicyDecision struct {
	Decision string `json:"decision"` // "allow" | "approve"
	Reason   string `json:"reason"`
}

// EvaluateCapability classifies a capability and checks grants. Locks b.mu.
//
// Fail-closed order of operations: unknown capabilities and irreversible
// classes never consult grants; mutating/external consult EXACT-match grants
// only. A nil broker answers "approve" — without a broker there is no
// persisted grant estate to trust, so the human gate is the only safe answer.
func (b *Broker) EvaluateCapability(bot, capability, scope string) PolicyDecision {
	if b == nil {
		return PolicyDecision{Decision: "approve", Reason: "no broker attached; human approval required"}
	}
	cap, known := capabilityFor(capability)
	if !known {
		return PolicyDecision{Decision: "approve", Reason: fmt.Sprintf("capability %q is not declared in the registry; default deny requires a human decision", capability)}
	}
	now := time.Now().UTC()
	// CEL policy rules (policy_engine_cel.go): data-driven policy evaluated
	// before the class defaults, deny before allow. Irreversible capabilities
	// never consult rules — a human decides, always.
	if cap.Class != CapabilityIrreversible {
		if d, matched := b.evaluatePolicyRules(bot, cap, scope); matched {
			return d
		}
	}
	switch cap.Class {
	case CapabilityReadOnly:
		return PolicyDecision{Decision: "allow", Reason: fmt.Sprintf("read-only capability %q auto-proceeds", cap.ID)}
	case CapabilityIrreversible:
		return PolicyDecision{Decision: "approve", Reason: fmt.Sprintf("capability %q is irreversible; a grant can never pre-authorize it", cap.ID)}
	default: // mutating, external
		if b.hasActivePolicyGrant(bot, capability, scope, now) {
			return PolicyDecision{Decision: "allow", Reason: fmt.Sprintf("active grant covers %q for this bot", cap.ID)}
		}
		return PolicyDecision{Decision: "approve", Reason: fmt.Sprintf("capability %q (%s) has no active grant for this bot", cap.ID, cap.Class)}
	}
}

// hasActivePolicyGrant reports whether a non-revoked, non-expired grant
// covers EXACTLY this (bot, capability, scope). Exact match on every part —
// no wildcards, so a grant never widens beyond what the human saw. Locks b.mu.
func (b *Broker) hasActivePolicyGrant(bot, capability, scope string, now time.Time) bool {
	a := policyGrantBotKey(bot)
	c := policyGrantCapabilityKey(capability)
	if a == "" || c == "" {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := range b.policyGrants {
		g := b.policyGrants[i]
		if policyGrantBotKey(g.BotSlug) == a &&
			policyGrantCapabilityKey(g.Capability) == c &&
			policyGrantScopeKey(g.Scope) == policyGrantScopeKey(scope) &&
			policyGrantActive(g, now) {
			return true
		}
	}
	return false
}

// addPolicyGrant mints a standing grant, TTL-capping the expiry. Locks b.mu
// and persists broker state on success.
func (b *Broker) addPolicyGrant(g policyGrant) (policyGrant, error) {
	g.BotSlug = policyGrantBotKey(g.BotSlug)
	g.Capability = policyGrantCapabilityKey(g.Capability)
	g.Scope = policyGrantScopeKey(g.Scope)
	if g.BotSlug == "" || g.Capability == "" {
		return g, fmt.Errorf("policy grant needs agent_slug and capability")
	}
	if _, known := capabilityFor(g.Capability); !known {
		return g, fmt.Errorf("capability %q is not declared in the registry", g.Capability)
	}
	now := time.Now().UTC()
	g.GrantedAt = now.Format(time.RFC3339)
	g.ID = fmt.Sprintf("pg-%s-%s-%d", g.BotSlug, strings.ReplaceAll(g.Capability, ".", "-"), now.UnixNano())
	if exp := strings.TrimSpace(g.ExpiresAt); exp != "" {
		ts, ok := parseGrantTime(exp)
		if !ok {
			return g, fmt.Errorf("expires_at %q is not a valid RFC3339 timestamp", exp)
		}
		if ts.After(now.Add(maxPolicyGrantTTL)) {
			g.ExpiresAt = now.Add(maxPolicyGrantTTL).Format(time.RFC3339)
		}
	}
	b.mu.Lock()
	b.policyGrants = append(b.policyGrants, g)
	err := b.saveLocked()
	b.mu.Unlock()
	if err != nil {
		return g, fmt.Errorf("persist policy grants: %w", err)
	}
	return g, nil
}

// revokePolicyGrant marks a grant revoked by id, failing closed on a miss.
// Locks b.mu and persists on success.
func (b *Broker) revokePolicyGrant(id string) error {
	b.mu.Lock()
	found := false
	for i := range b.policyGrants {
		if b.policyGrants[i].ID == id {
			b.policyGrants[i].RevokedAt = time.Now().UTC().Format(time.RFC3339)
			found = true
			break
		}
	}
	var err error
	if found {
		err = b.saveLocked()
	}
	b.mu.Unlock()
	if !found {
		return fmt.Errorf("no policy grant %q", id)
	}
	return err
}

// handlePolicyResolve answers the gate's question for one capability call.
// Broker-token gated like /integrations/resolve; the teammcp action gate
// consults it before any tool executes.
func (b *Broker) handlePolicyResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		BotSlug    string `json:"agent_slug"`
		Capability string `json:"capability"`
		Scope      string `json:"scope"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	decision := b.EvaluateCapability(body.BotSlug, body.Capability, body.Scope)
	b.auditPolicyDecision(body.BotSlug, body.Capability, body.Scope, decision)
	writeJSON(w, http.StatusOK, decision)
}

// handlePolicyGrants lists active grants (add ?all=true for the full estate)
// and mints/revokes them. SECURITY: no MCP tool reaches this endpoint —
// bots act only through the fixed teammcp surface, so a bot cannot grant
// itself capabilities.
func (b *Broker) handlePolicyGrants(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		includeAll := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("all")), "true")
		now := time.Now().UTC()
		b.mu.Lock()
		out := make([]policyGrant, 0, len(b.policyGrants))
		for _, g := range b.policyGrants {
			if includeAll || policyGrantActive(g, now) {
				out = append(out, g)
			}
		}
		b.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"grants": out})
	case http.MethodPost:
		var body struct {
			Action     string `json:"action"`
			ID         string `json:"id"`
			BotSlug    string `json:"agent_slug"`
			Capability string `json:"capability"`
			Scope      string `json:"scope"`
			ExpiresAt  string `json:"expires_at"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		switch strings.ToLower(strings.TrimSpace(body.Action)) {
		case "create":
			g, err := b.addPolicyGrant(policyGrant{
				BotSlug:    body.BotSlug,
				Capability: body.Capability,
				Scope:      body.Scope,
				GrantedBy:  "operator",
				ExpiresAt:  body.ExpiresAt,
			})
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"grant": g})
		case "revoke":
			if err := b.revokePolicyGrant(strings.TrimSpace(body.ID)); err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"revoked": body.ID})
		default:
			http.Error(w, "action must be create or revoke", http.StatusBadRequest)
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// clonePolicyGrants copies the grant estate for a persistence snapshot.
func clonePolicyGrants(in []policyGrant) []policyGrant {
	if len(in) == 0 {
		return nil
	}
	out := make([]policyGrant, len(in))
	copy(out, in)
	return out
}

// auditPolicyDecision records every gate answer — including auto-allows —
// so the human can audit exactly what proceeded without them. Locks b.mu.
func (b *Broker) auditPolicyDecision(bot, capability, scope string, d PolicyDecision) {
	if b == nil {
		return
	}
	entry := fmt.Sprintf("policy: %s %q scope=%q -> %s (%s)", policyGrantBotKey(bot), policyGrantCapabilityKey(capability), policyGrantScopeKey(scope), d.Decision, d.Reason)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.decisions = append(b.decisions, officeDecisionRecord{
		ID:            fmt.Sprintf("pol-%d", time.Now().UnixNano()),
		Kind:          "policy",
		Summary:       fmt.Sprintf("policy gate: %s", d.Decision),
		Reason:        entry,
		Owner:         policyGrantBotKey(bot),
		InitiatorKind: "routine",
		ActorID:       policyGrantBotKey(bot),
	})
}
