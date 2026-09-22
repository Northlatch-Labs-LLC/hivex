package team

// policy_engine_cel.go — data-driven policy rules (the openbot governance
// adaptation): operator-authored CEL expressions evaluated against the
// capability call BEFORE the class defaults, so policy is data instead of
// code. cel-go is Apache-2.0 (cel.dev/cel-go).
//
// Semantics (fail-closed everywhere):
//   - DENY before ALLOW: an active deny rule forces the human gate even when
//     an allow rule also matches. This engine's decision vocabulary is
//     allow|approve — a rule "deny" means "never auto-proceed; a human
//     decides" — the human's rejection IS the deny. Rules can only NARROW
//     access, never widen it.
//   - IRREVERSIBLE capabilities never consult rules (nor grants): a human
//     decides, always.
//   - Compile is fail-closed: an expression that does not parse or typecheck
//     is rejected at creation, never stored. A stored rule that somehow fails
//     to evaluate at runtime is inert (no match), never an allow.
//   - SECURITY: rule CRUD is broker-token gated and no MCP tool reaches it —
//     a bot can never mint its own policy.

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	cel "cel.dev/cel-go/cel"
)

// policyRule is one operator-authored rule: a CEL expression over the
// capability call plus its effect.
type policyRule struct {
	ID         string `json:"id"`
	Expression string `json:"expression"`
	Effect     string `json:"effect"` // "allow" | "deny"
	Note       string `json:"note,omitempty"`
	CreatedBy  string `json:"created_by"`
	CreatedAt  string `json:"created_at"`
	RevokedAt  string `json:"revoked_at,omitempty"`
}

const (
	policyRuleEffectAllow = "allow"
	policyRuleEffectDeny  = "deny"
)

// newPolicyRuleEnv builds the CEL environment rules evaluate against: the
// full capability call surface.
func newPolicyRuleEnv() (*cel.Env, error) {
	return cel.NewEnv(
		cel.Variable("agent", cel.StringType),
		cel.Variable("capability", cel.StringType),
		cel.Variable("scope", cel.StringType),
		cel.Variable("class", cel.StringType),
	)
}

var (
	policyRuleEnvOnce sync.Once
	policyRuleEnv     *cel.Env
	policyRuleEnvErr  error

	policyRuleProgMu    sync.RWMutex
	policyRuleProgCache = map[string]cel.Program{}
)

// policyRuleEnvInit resolves the shared CEL environment exactly once.
func policyRuleEnvInit() (*cel.Env, error) {
	policyRuleEnvOnce.Do(func() {
		policyRuleEnv, policyRuleEnvErr = newPolicyRuleEnv()
	})
	return policyRuleEnv, policyRuleEnvErr
}

// compilePolicyRule validates an expression against the environment
// (parse + type-check) and returns the compiled program. Errors name the
// problem so the operator surface can show it.
func compilePolicyRule(expression string) (cel.Program, error) {
	env, err := policyRuleEnvInit()
	if err != nil {
		return nil, fmt.Errorf("cel environment: %w", err)
	}
	ast, iss := env.Compile(expression)
	if iss != nil && iss.Err() != nil {
		return nil, fmt.Errorf("rule expression does not compile: %w", iss.Err())
	}
	if ast.OutputType() != cel.BoolType {
		return nil, fmt.Errorf("rule expression must evaluate to a bool, got %s", ast.OutputType())
	}
	prog, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("rule expression program: %w", err)
	}
	return prog, nil
}

// policyRuleProgram returns the compiled program for an expression,
// compiling and caching on first use. Runtime compile failures are inert
// (nil program → no match), never an allow.
func policyRuleProgram(expression string) cel.Program {
	policyRuleProgMu.RLock()
	prog, ok := policyRuleProgCache[expression]
	policyRuleProgMu.RUnlock()
	if ok {
		return prog
	}
	compiled, err := compilePolicyRule(expression)
	if err != nil {
		return nil
	}
	policyRuleProgMu.Lock()
	policyRuleProgCache[expression] = compiled
	policyRuleProgMu.Unlock()
	return compiled
}

// policyRuleMatches evaluates one rule's program against the call.
func policyRuleMatches(r policyRule, bot string, cap Capability, scope string) bool {
	if strings.TrimSpace(r.RevokedAt) != "" {
		return false
	}
	prog := policyRuleProgram(r.Expression)
	if prog == nil {
		return false // fail closed: an uncompilable rule is inert
	}
	out, _, err := prog.Eval(map[string]any{
		"agent":      policyGrantBotKey(bot),
		"capability": policyGrantCapabilityKey(cap.ID),
		"scope":      policyGrantScopeKey(scope),
		"class":      string(cap.Class),
	})
	if err != nil {
		return false
	}
	match, ok := out.Value().(bool)
	return ok && match
}

// evaluatePolicyRules runs the active rules against the call, deny before
// allow. Called by EvaluateCapability AFTER registry classification and
// BEFORE the class defaults; irreversible classes never reach it. Locks b.mu.
// The second return is false when no rule matched (fall through to defaults).
func (b *Broker) evaluatePolicyRules(bot string, cap Capability, scope string) (PolicyDecision, bool) {
	if b == nil {
		return PolicyDecision{}, false
	}
	b.mu.Lock()
	rules := make([]policyRule, 0, len(b.policyRules))
	for _, r := range b.policyRules {
		if strings.TrimSpace(r.RevokedAt) == "" {
			rules = append(rules, r)
		}
	}
	b.mu.Unlock()

	// Deny before allow: one pass for deny, one for allow.
	for _, r := range rules {
		if r.Effect == policyRuleEffectDeny && policyRuleMatches(r, bot, cap, scope) {
			return PolicyDecision{
				Decision: "approve",
				Reason:   fmt.Sprintf("policy rule %s denies %q for this call — a human must decide", r.ID, cap.ID),
			}, true
		}
	}
	for _, r := range rules {
		if r.Effect == policyRuleEffectAllow && policyRuleMatches(r, bot, cap, scope) {
			return PolicyDecision{
				Decision: "allow",
				Reason:   fmt.Sprintf("policy rule %s allows %q for this call", r.ID, cap.ID),
			}, true
		}
	}
	return PolicyDecision{}, false
}

// addPolicyRule mints a rule after a fail-closed compile check. Locks b.mu
// and persists broker state on success.
func (b *Broker) addPolicyRule(r policyRule) (policyRule, error) {
	r.Expression = strings.TrimSpace(r.Expression)
	r.Effect = strings.ToLower(strings.TrimSpace(r.Effect))
	r.Note = strings.TrimSpace(r.Note)
	if r.Expression == "" {
		return r, fmt.Errorf("policy rule needs an expression")
	}
	if r.Effect != policyRuleEffectAllow && r.Effect != policyRuleEffectDeny {
		return r, fmt.Errorf("policy rule effect must be allow or deny")
	}
	if _, err := compilePolicyRule(r.Expression); err != nil {
		return r, err
	}
	now := time.Now().UTC()
	r.ID = fmt.Sprintf("pr-%d", now.UnixNano())
	r.CreatedAt = now.Format(time.RFC3339)
	b.mu.Lock()
	b.policyRules = append(b.policyRules, r)
	err := b.saveLocked()
	b.mu.Unlock()
	if err != nil {
		return r, fmt.Errorf("persist policy rules: %w", err)
	}
	return r, nil
}

// revokePolicyRule marks a rule revoked by id, failing closed on a miss.
func (b *Broker) revokePolicyRule(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := range b.policyRules {
		if b.policyRules[i].ID == id {
			if strings.TrimSpace(b.policyRules[i].RevokedAt) == "" {
				b.policyRules[i].RevokedAt = time.Now().UTC().Format(time.RFC3339)
				if err := b.saveLocked(); err != nil {
					return fmt.Errorf("persist policy rules: %w", err)
				}
			}
			return nil
		}
	}
	return fmt.Errorf("no policy rule %q", id)
}

// clonePolicyRules copies the rule estate for a persistence snapshot.
func clonePolicyRules(in []policyRule) []policyRule {
	if len(in) == 0 {
		return nil
	}
	out := make([]policyRule, len(in))
	copy(out, in)
	return out
}

// handlePolicyRules is the operator CRUD surface for CEL policy rules:
// GET lists (all rules), POST {action: create|revoke}. Broker-token gated;
// no MCP tool reaches it. Compile errors surface as 400 with the CEL
// diagnostics so the operator sees exactly what is wrong.
func (b *Broker) handlePolicyRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		b.mu.Lock()
		out := append([]policyRule(nil), b.policyRules...)
		b.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"rules": out}); err != nil {
			log.Printf("broker: encode policy rules response: %v", err)
		}
	case http.MethodPost:
		var body struct {
			Action     string `json:"action"`
			ID         string `json:"id"`
			Expression string `json:"expression"`
			Effect     string `json:"effect"`
			Note       string `json:"note"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		switch strings.ToLower(strings.TrimSpace(body.Action)) {
		case "create":
			rule, err := b.addPolicyRule(policyRule{
				Expression: body.Expression,
				Effect:     body.Effect,
				Note:       body.Note,
				CreatedBy:  "operator",
			})
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"rule": rule})
		case "revoke":
			if err := b.revokePolicyRule(strings.TrimSpace(body.ID)); err != nil {
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
