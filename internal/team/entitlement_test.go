package team

// entitlement_test.go — pins the harness side of the portal entitlement
// contract: the exact token scheme (base64url(payload).base64url(HMAC)),
// fail-open caching posture, and the bearer-channel trust model.

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Northlatch-Labs-LLC/hivex/internal/config"
)

// signTestEntitlement builds a token exactly like the portal does
// (9router/src/lib/portal/entitlements.js): base64url(JSON payload) + "." +
// base64url(HMAC-SHA256(payloadB64url, key)).
func signTestEntitlement(payload map[string]any, key string) string {
	raw, _ := json.Marshal(payload)
	payloadB64 := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(payloadB64))
	return fmt.Sprintf("%s.%s", payloadB64, base64.RawURLEncoding.EncodeToString(mac.Sum(nil)))
}

func testPayload(expSec int64) map[string]any {
	return map[string]any{
		"account": "acc-1", "tier": "monthly", "turn_cap_monthly": nil,
		"provider_locked": true, "issued_at": time.Now().Unix(), "exp": expSec,
	}
}

// TestEntitlementTokenSchemePinnedToThePortal pins the wire format: a
// portal-issued token verifies, and every tampering vector fails.
func TestEntitlementTokenSchemePinnedToThePortal(t *testing.T) {
	const key = "test-signing-key"
	now := time.Now()

	token := signTestEntitlement(testPayload(now.Add(time.Hour).Unix()), key)
	got, err := verifyEntitlementToken(token, key, now)
	if err != nil {
		t.Fatalf("valid token must verify: %v", err)
	}
	if got.Tier != "monthly" || !got.ProviderLocked || !got.UncappedTurns() || got.Account != "acc-1" {
		t.Fatalf("payload mismatch: %+v", got)
	}

	if _, err := verifyEntitlementToken(token, "other-key", now); err == nil {
		t.Fatal("wrong signing key must fail")
	}

	// Tampered payload with the original signature.
	parts := split2(token)
	forged := base64.RawURLEncoding.EncodeToString([]byte(`{"account":"acc-1","tier":"business","exp":` + fmt.Sprint(now.Add(time.Hour).Unix()) + `}`))
	if _, err := verifyEntitlementToken(forged+"."+parts[1], key, now); err == nil {
		t.Fatal("tampered payload must fail signature check")
	}

	if _, err := verifyEntitlementToken(token, key, now.Add(2*time.Hour)); err == nil {
		t.Fatal("expired token must fail")
	}
	for _, bad := range []string{"", "no-dot", "a.b.c", parts[0] + ".not-base64!!", ". dangling"} {
		if _, err := verifyEntitlementToken(bad, key, now); err == nil {
			t.Fatalf("malformed token %q must fail", bad)
		}
	}
}

func split2(token string) []string {
	for i := len(token) - 1; i >= 0; i-- {
		if token[i] == '.' {
			return []string{token[:i], token[i+1:]}
		}
	}
	return nil
}

// fakeEntitlement is the injected client for cache/gate tests.
type fakeEntitlement struct {
	payload portalEntitlement
	err     error
	fetches int
}

func (f *fakeEntitlement) Fetch(context.Context) (portalEntitlement, error) {
	f.fetches++
	if f.err != nil {
		return portalEntitlement{}, f.err
	}
	return f.payload, nil
}

func withEntitlement(t *testing.T, f *fakeEntitlement) {
	t.Helper()
	setEntitlementClient(f)
	resetEntitlementCacheForTests()
	t.Cleanup(func() {
		setEntitlementClient(nil)
		resetEntitlementCacheForTests()
	})
}

// TestCurrentEntitlementFailsOpen pins the posture: no portal linkage →
// standalone (nil); a fetch outage keeps the last known entitlement (stale
// beats none) and never errors into the turn path.
func TestCurrentEntitlementFailsOpen(t *testing.T) {
	t.Setenv("HIVEX_PORTAL_ACCOUNT_KEY", "")
	t.Setenv("HIVEX_PORTAL_ENTITLEMENT_URL", "")
	if e := currentEntitlement(context.Background()); e != nil {
		t.Fatalf("no portal linkage must be standalone, got %+v", e)
	}

	t.Setenv("HIVEX_PORTAL_ACCOUNT_KEY", "acct-key")
	t.Setenv("HIVEX_PORTAL_ENTITLEMENT_URL", "http://portal.test/api/portal/entitlement")

	f := &fakeEntitlement{payload: portalEntitlement{Account: "a", Tier: "free"}}
	withEntitlement(t, f)
	if e := currentEntitlement(context.Background()); e == nil || e.Tier != "free" {
		t.Fatalf("fetched entitlement must be cached, got %+v", e)
	}
	if f.fetches != 1 {
		t.Fatalf("cache must serve without refetch, fetches=%d", f.fetches)
	}

	// Outage: keep the stale value.
	f.err = errors.New("portal down")
	if e := currentEntitlement(context.Background()); e == nil || e.Tier != "free" {
		t.Fatalf("outage must keep the stale entitlement, got %+v", e)
	}
}

// TestPortalTurnGateBlocksOnCappedBudget pins the Free-cap enforcement
// loop: the meter report's authoritative capped flag arms the pre-turn
// gate; an uncapped (or absent) report leaves it open (fail-open).
func TestPortalTurnGateBlocksOnCappedBudget(t *testing.T) {
	resetPortalTurnBudgetForTests()
	t.Cleanup(resetPortalTurnBudgetForTests)

	if turnGateBlockedByPortal() {
		t.Fatal("no budget report yet must leave the gate open (fail-open)")
	}

	cap := int64(1000)
	recordPortalTurnBudget(&portalTurnBudget{Used: 1000, Cap: &cap, Capped: true, At: time.Now()})
	if !turnGateBlockedByPortal() {
		t.Fatal("capped report must arm the gate")
	}

	recordPortalTurnBudget(&portalTurnBudget{Used: 5, Cap: &cap, Capped: false, At: time.Now()})
	if turnGateBlockedByPortal() {
		t.Fatal("uncapped report must re-open the gate")
	}
}

// TestTurnCapNoticePostsOncePerCooldown pins the upgrade prompt: the office
// feed gets exactly one cap notice per cooldown window.
func TestTurnCapNoticePostsOncePerCooldown(t *testing.T) {
	resetTurnCapNoticeForTests()
	t.Cleanup(resetTurnCapNoticeForTests)

	b := newTestBroker(t)
	before := len(b.messages)
	postTurnCapGateNotice(b, "researcher")
	if got := len(b.messages); got != before+1 {
		t.Fatalf("first notice must post exactly one office message, before=%d got=%d", before, got)
	}
	postTurnCapGateNotice(b, "researcher")
	if got := len(b.messages); got != before+1 {
		t.Fatalf("second notice within the cooldown must not spam, got %d messages", got)
	}
}

// TestHiveAPIKindIsCompatDispatched pins the dispatch routing: the Hivex
// Gateway kind routes through the OpenAI-compat runner.
func TestHiveAPIKindIsCompatDispatched(t *testing.T) {
	if !isOpenAICompatKind("hiveapi") {
		t.Fatal("hiveapi must dispatch through the OpenAI-compat runner")
	}
}

// TestEntitlementTierHelpers pins the tier predicates the gate consumes.
func TestEntitlementTierHelpers(t *testing.T) {
	cap := int64(1000)
	free := portalEntitlement{Tier: "free", TurnCapMonthly: &cap}
	monthly := portalEntitlement{Tier: "monthly"}
	if free.TierMonthly() || free.UncappedTurns() {
		t.Fatalf("free tier helpers: capped free must not be monthly/uncapped, got %+v", free)
	}
	if !monthly.TierMonthly() || !monthly.UncappedTurns() {
		t.Fatalf("monthly tier helpers: %+v", monthly)
	}
	if config.MemoryBackendCognee == "" {
		t.Fatal("sanity: cognee backend constant must exist")
	}
}
