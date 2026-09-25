package team

import (
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func (b *Broker) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Exempt liveness and version checks from all rate limiting.
		if isLivenessPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		authenticated := b.requestHasBrokerAuth(r)

		// Authenticated callers bypass the IP-scoped bucket (web UI and trusted
		// tools must not share a bucket with anonymous callers), but authenticated
		// *bot* traffic is still subject to a separate per-bot bucket below.
		if !authenticated {
			retryAfter, limited := b.consumeRateLimit(clientIPFromRequest(r))
			if limited {
				writeRateLimitedResponse(w, retryAfter)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		// Authenticated — check the per-bot bucket so a prompt-injected bot
		// cannot loop forever on team_broadcast / team_action_execute. Operator
		// traffic (web UI) does not set X-hivebot-Bot and is exempt.
		botSlug := strings.TrimSpace(r.Header.Get(botRateLimitHeader))
		if botSlug == "" || isBotBucketExemptPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		retryAfter, limited := b.consumeBotRateLimit(botSlug)
		if limited {
			log.Printf("broker: bot %q tripped per-bot rate limit (%d req / %s) on %s — possible runaway loop", botSlug, b.botRateLimitRequests, b.botRateLimitWindow, r.URL.Path)
			writeRateLimitedResponse(w, retryAfter)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isLivenessPath reports whether the request path is a pure liveness or
// version probe that must never be rate-limited (operators need these even
// when the broker is saturated). /web-token is NOT on this list — it
// dispenses the broker bearer and an unthrottled enumeration path would be
// surprising, even though the handler itself is loopback+Host gated.
func isLivenessPath(path string) bool {
	return path == "/health" || path == "/version"
}

// isBotBucketExemptPath reports whether the path is an open SSE stream or
// otherwise doesn't represent a tool-call-shaped loopable request. These
// connections stay open for a long time rather than spinning on request
// count, so counting them against the bot bucket would be incorrect.
func isBotBucketExemptPath(path string) bool {
	if path == "/events" {
		return true
	}
	if strings.HasPrefix(path, "/agent-stream/") {
		return true
	}
	return false
}

func writeRateLimitedResponse(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := int((retryAfter + time.Second - 1) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = io.WriteString(w, `{"error":"rate_limited"}`)
}

// rateLimitNow returns the current time for rate-limit calculations.
// Tests may override b.nowFn to advance a synthetic clock without sleeping.
func (b *Broker) rateLimitNow() time.Time {
	if b.nowFn != nil {
		return b.nowFn()
	}
	return time.Now()
}

func (b *Broker) consumeRateLimit(clientIP string) (time.Duration, bool) {
	limit := b.rateLimitRequests
	if limit <= 0 {
		limit = defaultRateLimitRequestsPerWindow
	}
	window := b.rateLimitWindow
	if window <= 0 {
		window = defaultRateLimitWindow
	}

	now := b.rateLimitNow()
	key := rateLimitKey(clientIP)
	cutoff := now.Add(-window)

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.rateLimitBuckets == nil {
		b.rateLimitBuckets = make(map[string]ipRateLimitBucket)
	}
	if b.lastRateLimitPrune.IsZero() || now.Sub(b.lastRateLimitPrune) >= window {
		for ip, bucket := range b.rateLimitBuckets {
			bucket.timestamps = pruneRateLimitEntries(bucket.timestamps, cutoff)
			if len(bucket.timestamps) == 0 {
				delete(b.rateLimitBuckets, ip)
				continue
			}
			b.rateLimitBuckets[ip] = bucket
		}
		b.lastRateLimitPrune = now
	}

	bucket := b.rateLimitBuckets[key]
	bucket.timestamps = pruneRateLimitEntries(bucket.timestamps, cutoff)
	if len(bucket.timestamps) >= limit {
		retryAfter := bucket.timestamps[0].Add(window).Sub(now)
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		b.rateLimitBuckets[key] = bucket
		return retryAfter, true
	}

	bucket.timestamps = append(bucket.timestamps, now)
	b.rateLimitBuckets[key] = bucket
	return 0, false
}

// consumeBotRateLimit counts an authenticated request against the per-bot
// bucket keyed by the X-HIVEX-Bot header. It mirrors consumeRateLimit but
// lives in its own bucket so bot traffic cannot starve operator traffic and
// vice versa.
func (b *Broker) consumeBotRateLimit(botSlug string) (time.Duration, bool) {
	botSlug = strings.TrimSpace(botSlug)
	if botSlug == "" {
		return 0, false
	}

	limit := b.botRateLimitRequests
	if limit <= 0 {
		limit = defaultBotRateLimitRequestsPerWindow
	}
	window := b.botRateLimitWindow
	if window <= 0 {
		window = defaultBotRateLimitWindow
	}

	now := b.rateLimitNow()
	cutoff := now.Add(-window)

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.botRateLimitBuckets == nil {
		b.botRateLimitBuckets = make(map[string]ipRateLimitBucket)
	}
	if b.lastBotRateLimitPrune.IsZero() || now.Sub(b.lastBotRateLimitPrune) >= window {
		for slug, bucket := range b.botRateLimitBuckets {
			bucket.timestamps = pruneRateLimitEntries(bucket.timestamps, cutoff)
			if len(bucket.timestamps) == 0 {
				delete(b.botRateLimitBuckets, slug)
				continue
			}
			b.botRateLimitBuckets[slug] = bucket
		}
		b.lastBotRateLimitPrune = now
	}

	bucket := b.botRateLimitBuckets[botSlug]
	bucket.timestamps = pruneRateLimitEntries(bucket.timestamps, cutoff)
	if len(bucket.timestamps) >= limit {
		retryAfter := bucket.timestamps[0].Add(window).Sub(now)
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		b.botRateLimitBuckets[botSlug] = bucket
		return retryAfter, true
	}

	bucket.timestamps = append(bucket.timestamps, now)
	b.botRateLimitBuckets[botSlug] = bucket
	return 0, false
}

func pruneRateLimitEntries(entries []time.Time, cutoff time.Time) []time.Time {
	keepIdx := 0
	for keepIdx < len(entries) && !entries[keepIdx].After(cutoff) {
		keepIdx++
	}
	if keepIdx == 0 {
		return entries
	}
	if keepIdx >= len(entries) {
		return nil
	}
	return entries[keepIdx:]
}

func rateLimitKey(remoteAddr string) string {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if remoteAddr == "" {
		return "unknown"
	}
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil && strings.TrimSpace(host) != "" {
		return host
	}
	return remoteAddr
}

func clientIPFromRequest(r *http.Request) string {
	if r == nil {
		return "unknown"
	}
	if trustForwardedClientIP(r.RemoteAddr) {
		if forwarded := firstForwardedIP(r.Header.Get("X-Forwarded-For")); forwarded != "" {
			return forwarded
		}
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
			return rateLimitKey(realIP)
		}
	}
	return rateLimitKey(r.RemoteAddr)
}

func firstForwardedIP(value string) string {
	for _, part := range strings.Split(value, ",") {
		candidate := rateLimitKey(part)
		if candidate == "" || candidate == "unknown" {
			continue
		}
		if ip := net.ParseIP(candidate); ip != nil {
			return ip.String()
		}
	}
	return ""
}

func trustForwardedClientIP(remoteAddr string) bool {
	host := rateLimitKey(remoteAddr)
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func setProxyClientIPHeaders(header http.Header, remoteAddr string) {
	if header == nil {
		return
	}
	if clientIP := rateLimitKey(remoteAddr); clientIP != "unknown" {
		header.Set("X-Forwarded-For", clientIP)
		header.Set("X-Real-IP", clientIP)
	}
}

func (b *Broker) requestAuthToken(r *http.Request) string {
	if r == nil {
		return ""
	}
	if token := strings.TrimSpace(r.URL.Query().Get("token")); token != "" {
		return token
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	return ""
}

func (b *Broker) requestHasBrokerAuth(r *http.Request) bool {
	return b.requestAuthToken(r) == b.token
}

// corsMiddleware adds CORS headers only for the web UI origin.
// If no web UI origins are configured, no CORS headers are set.
//
// Requests with empty or "null" Origin are same-origin or non-browser callers
// (curl, Go tests, CLI tools). They do not need a CORS header to succeed. We
// intentionally do NOT set Access-Control-Allow-Origin: * for them — that
// would let a file:// page or sandboxed iframe make authenticated cross-origin
// reads once it has the Bearer token.
func (b *Broker) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && origin != "null" && len(b.webUIOrigins) > 0 {
			for _, allowed := range b.webUIOrigins {
				if origin == allowed {
					w.Header().Set("Access-Control-Allow-Origin", allowed)
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
					break
				}
			}
		}
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isLoopbackRemote reports whether r.RemoteAddr is loopback (127.0.0.0/8, ::1).
// Returns false if RemoteAddr is empty or unparseable — fail closed.
func isLoopbackRemote(r *http.Request) bool {
	if r == nil {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if host == "" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return host == "localhost"
}

// hostHeaderIsLoopback reports whether the HTTP Host header is a loopback
// hostname (localhost, 127.0.0.1, ::1). DNS-rebinding attacks rely on r.Host
// being an attacker-controlled name like rebind.example.com that only
// resolves to 127.0.0.1 at request time; Go's default mux routes by path and
// ignores Host, so routes that sit on 127.0.0.1 will happily serve responses
// to the attacker's origin. Validating Host on sensitive handlers closes this.
//
// The port component is intentionally not validated — the broker and web UI
// run on different ports and dev setups may proxy through 80/443. The
// loopback hostname is the security boundary.
func hostHeaderIsLoopback(r *http.Request) bool {
	if r == nil {
		return false
	}
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// publicWebUIHostsEnv names the exact-host allowlist environment variable.
// Comma-separated hostnames (e.g. "gridframes.app,office.example.org").
const publicWebUIHostsEnv = "HIVEX_WEB_PUBLIC_HOSTS"

// hostAllowedByPublicHostsEnv reports whether host — an already
// port-stripped, case-folded Host-header hostname — exactly matches an entry
// of HIVEX_WEB_PUBLIC_HOSTS. Entries are compared whole after trimming and
// lowercasing: "gridframes.app" matches Host "gridframes.app" and
// "gridframes.app:443" but NOT "evil.gridframes.app" or
// "gridframes.app.evil.io" (no suffix, prefix, or wildcard matching — an
// allowlist that pattern-matches is an allowlist that leaks). An unset or
// empty env means no public host is allowlisted.
func hostAllowedByPublicHostsEnv(host string) bool {
	raw := strings.TrimSpace(os.Getenv(publicWebUIHostsEnv))
	if raw == "" {
		return false
	}
	host = strings.ToLower(strings.TrimSpace(host))
	for _, entry := range strings.Split(raw, ",") {
		entryHost := strings.ToLower(strings.TrimSpace(entry))
		// Tolerate an operator who writes the port into the entry; the Host
		// comparison is on the hostname only (see webUIHostAllowed).
		if h, _, err := net.SplitHostPort(entryHost); err == nil {
			entryHost = h
		}
		if entryHost != "" && entryHost == host {
			return true
		}
	}
	return false
}

// webUIHostAllowed reports whether the Host header may pass the rebinding
// gate: a recognized localhost form, or a hostname the operator explicitly
// listed in HIVEX_WEB_PUBLIC_HOSTS for serving the web UI behind a reverse
// proxy on a public domain. The env path exists ONLY for that deployment
// shape: the proxy is expected to run on the same host (RemoteAddr stays
// loopback, enforced separately by webUIRebindGuard), terminate TLS, and
// forward with the original Host preserved. Everything else still 403s.
func webUIHostAllowed(r *http.Request) bool {
	if hostHeaderIsLoopback(r) {
		return true
	}
	if r == nil {
		return false
	}
	host := strings.ToLower(r.Host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return hostAllowedByPublicHostsEnv(host)
}

// webUIRebindGuard wraps a handler with a DNS-rebinding / cross-origin gate.
// It rejects any request whose RemoteAddr is not loopback or whose Host header
// is neither a recognized localhost form nor an operator-allowlisted public
// host (HIVEX_WEB_PUBLIC_HOSTS, exact match). Applied on the web UI mux
// because that mux auto-attaches the broker's Bearer token on forwarded
// requests; without this gate, a malicious website can use DNS rebinding to
// ride the token. The RemoteAddr loopback requirement is unchanged by the
// public-host allowlist: a proxied deployment must keep the proxy on the same
// host, and the Bearer-token auth on forwarded requests is unaffected.
func webUIRebindGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackRemote(r) || !webUIHostAllowed(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
