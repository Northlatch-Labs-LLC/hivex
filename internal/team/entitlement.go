package team

// entitlement.go — the harness side of the portal's signed entitlement
// contract: the Hivex Harness proves an account's tier, turn cap, and
// provider lock without touching the portal database.
//
// Contract (9router/src/lib/portal/entitlements.js, verified live):
//   token  = base64url(JSON payload) + "." + base64url(HMAC-SHA256(payload))
//   payload = { account, tier, turn_cap_monthly, provider_locked,
//               issued_at, exp }   // exp in SECONDS
//
// Env: HIVEX_PORTAL_ENTITLEMENT_URL (the harness fetches with Bearer
// HIVEX_PORTAL_ACCOUNT_KEY — the same key the turn meter posts with) and the
// optional HIVEX_ENTITLEMENT_SIGNING_KEY. With the signing key set, every
// fetched token is verified (HMAC + expiry) and only the VERIFIED payload is
// trusted. Without it, the bearer-authed local channel carries the payload
// (documented posture: set the shared key for full verification).

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Northlatch-Labs-LLC/hivex/internal/config"
)

// portalEntitlement is the verified payload the harness enforces.
type portalEntitlement struct {
	Account        string `json:"account"`
	Tier           string `json:"tier"`
	TurnCapMonthly *int64 `json:"turn_cap_monthly"` // nil = uncapped
	ProviderLocked bool   `json:"provider_locked"`
	IssuedAt       int64  `json:"issued_at"`
	Exp            int64  `json:"exp"`
}

// entitlementFetchTimeout: reads run on the gate path before a turn starts;
// an unreachable portal must not stall the office — fail and let the meter
// posture (or absence of portal linkage) decide.
const entitlementFetchTimeout = 3 * time.Second

// entitlementClient is the narrow surface the gate depends on; tests
// inject a fake, production talks to the portal.
type entitlementClient interface {
	Fetch(ctx context.Context) (portalEntitlement, error)
}

func entitlementURL() string {
	return strings.TrimSpace(config.Getenv("HIVEX_PORTAL_ENTITLEMENT_URL"))
}

func entitlementAccountKey() string {
	return strings.TrimSpace(config.Getenv("HIVEX_PORTAL_ACCOUNT_KEY"))
}

func entitlementSigningKey() string {
	return strings.TrimSpace(config.Getenv("HIVEX_ENTITLEMENT_SIGNING_KEY"))
}

// entitlementHTTPClient fetches over the bearer-authed channel and verifies
// the token when a signing key is configured.
type entitlementHTTPClient struct{ http *http.Client }

func newEntitlementHTTPClient() *entitlementHTTPClient {
	return &entitlementHTTPClient{http: &http.Client{}}
}

// verifyEntitlementToken checks the portal's token scheme: split on ".",
// timing-safe HMAC-SHA256 compare over the payload segment, JSON payload,
// expiry in seconds. Returns the payload or an error naming the failure.
func verifyEntitlementToken(token, key string, now time.Time) (portalEntitlement, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return portalEntitlement{}, fmt.Errorf("entitlement token: malformed")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return portalEntitlement{}, fmt.Errorf("entitlement token: bad signature encoding: %w", err)
	}
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(parts[0]))
	if !hmac.Equal(mac.Sum(nil), sig) {
		return portalEntitlement{}, fmt.Errorf("entitlement token: signature mismatch")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return portalEntitlement{}, fmt.Errorf("entitlement token: bad payload encoding: %w", err)
	}
	var out portalEntitlement
	if err := json.Unmarshal(raw, &out); err != nil {
		return portalEntitlement{}, fmt.Errorf("entitlement token: unparseable payload: %w", err)
	}
	if out.Exp > 0 && now.Unix() > out.Exp {
		return portalEntitlement{}, fmt.Errorf("entitlement token: expired")
	}
	return out, nil
}

// Fetch pulls the signed entitlement over the bearer channel. With a signing
// key configured, only the VERIFIED token payload is returned; without one,
// the bearer-authed payload is trusted (documented local-channel posture).
func (c *entitlementHTTPClient) Fetch(ctx context.Context) (portalEntitlement, error) {
	url := entitlementURL()
	key := entitlementAccountKey()
	if url == "" || key == "" {
		return portalEntitlement{}, fmt.Errorf("HIVEX_PORTAL_ENTITLEMENT_URL and HIVEX_PORTAL_ACCOUNT_KEY are not set")
	}
	ctx, cancel := context.WithTimeout(ctx, entitlementFetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return portalEntitlement{}, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	res, err := c.http.Do(req)
	if err != nil {
		return portalEntitlement{}, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return portalEntitlement{}, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return portalEntitlement{}, fmt.Errorf("entitlement fetch: status %d: %s", res.StatusCode, truncate(string(raw), 200))
	}
	var body struct {
		Token       string            `json:"token"`
		Entitlement portalEntitlement `json:"entitlement"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return portalEntitlement{}, fmt.Errorf("entitlement fetch: unparseable response: %w", err)
	}
	if signingKey := entitlementSigningKey(); signingKey != "" {
		return verifyEntitlementToken(body.Token, signingKey, time.Now())
	}
	if body.Entitlement.Tier == "" {
		return portalEntitlement{}, fmt.Errorf("entitlement fetch: response carries no entitlement")
	}
	return body.Entitlement, nil
}

// entitlementClient injection (same pattern as the memory backends).
var (
	entitlementClientMu sync.RWMutex
	entitlementClientV  entitlementClient
)

func setEntitlementClient(c entitlementClient) {
	entitlementClientMu.Lock()
	defer entitlementClientMu.Unlock()
	entitlementClientV = c
}

func resolveEntitlementClient() entitlementClient {
	entitlementClientMu.RLock()
	defer entitlementClientMu.RUnlock()
	if entitlementClientV != nil {
		return entitlementClientV
	}
	return newEntitlementHTTPClient()
}

// entitlementCacheTTL bounds how long a fetched entitlement is trusted
// locally between portal round-trips.
const entitlementCacheTTL = 5 * time.Minute

var (
	entitlementCacheMu   sync.Mutex
	entitlementCachedAt  time.Time
	entitlementCachedVal *portalEntitlement
)

// currentEntitlement returns the cached entitlement, refreshing it when the
// TTL elapsed. Fail-open: an unreachable portal yields nil — the office
// keeps running standalone (the turn meter still records; the gate pauses
// only on a positive capped/locked signal, never on a fetch error).
func currentEntitlement(ctx context.Context) *portalEntitlement {
	if entitlementURL() == "" || entitlementAccountKey() == "" {
		return nil // no portal linkage configured — standalone harness
	}
	entitlementCacheMu.Lock()
	defer entitlementCacheMu.Unlock()
	if entitlementCachedVal != nil && time.Since(entitlementCachedAt) < entitlementCacheTTL {
		return entitlementCachedVal
	}
	fetched, err := resolveEntitlementClient().Fetch(ctx)
	if err != nil {
		return entitlementCachedVal // stale is better than nothing; nil if none yet
	}
	entitlementCachedVal = &fetched
	entitlementCachedAt = time.Now()
	return entitlementCachedVal
}

func resetEntitlementCacheForTests() {
	entitlementCacheMu.Lock()
	defer entitlementCacheMu.Unlock()
	entitlementCachedVal = nil
	entitlementCachedAt = time.Time{}
}

// TierMonthly reports whether the verified entitlement is the Monthly tier
// (the provider-locked tier).
func (e portalEntitlement) TierMonthly() bool { return e.Tier == "monthly" }

// UncappedTurns reports whether the entitlement carries no monthly turn cap.
func (e portalEntitlement) UncappedTurns() bool { return e.TurnCapMonthly == nil }

// signEntitlementToken builds a token in the portal's exact wire format
// (used by office evals to prove the harness verifies real portal tokens).
func signEntitlementToken(payload map[string]any, key string) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(payloadB64))
	return fmt.Sprintf("%s.%s", payloadB64, base64.RawURLEncoding.EncodeToString(mac.Sum(nil))), nil
}

// ── Pre-turn gate (Phase A2.3) ───────────────────────────────────────────────

// turnGateBlockedByPortal reports whether the portal's authoritative meter
// report says the account's monthly turn budget is exhausted. Fail-open: no
// report yet (fresh boot, metering outage) allows the turn — the portal
// still meters whatever runs; the next report re-arms the gate.
func turnGateBlockedByPortal() bool {
	b := lastPortalTurnBudget()
	return b != nil && b.Capped
}

// turnCapNoticeCooldown bounds how often the upgrade prompt is posted into
// the office feed so a capped account doesn't spam the channel.
const turnCapNoticeCooldown = 10 * time.Minute

var (
	turnCapNoticeMu   sync.Mutex
	turnCapNoticeLast time.Time
)

func postTurnCapGateNotice(b *Broker, slug string) {
	turnCapNoticeMu.Lock()
	if time.Since(turnCapNoticeLast) < turnCapNoticeCooldown {
		turnCapNoticeMu.Unlock()
		return
	}
	turnCapNoticeLast = time.Now()
	turnCapNoticeMu.Unlock()
	if b == nil {
		return
	}
	_, _, _ = b.PostAutomationMessage(
		"hive", "", "Turn cap reached",
		fmt.Sprintf("Monthly free-turn cap reached for %s — new turns pause until next month. Upgrade any time from the Hive Customer Portal → Pricing (crypto via SUI or card via Stripe).", slug),
		"", "", "", nil, "",
	)
}

func resetTurnCapNoticeForTests() {
	turnCapNoticeMu.Lock()
	defer turnCapNoticeMu.Unlock()
	turnCapNoticeLast = time.Time{}
}
