package team

// memanto_backend.go — HIVEX_MEMORY_BACKEND=memanto: the self-hosted Memanto
// memory agent as the shared organizational memory backend.
//
// Doctrine (docs/specs/core-loop.md + third_party/memanto/INTEGRATION.md):
// deterministic hooks decide WHAT is remembered and WHEN. This backend is
// pure transport — the broker's distill/verification seams remain the only
// write path, and context assembly remains the read path. Memanto adds its
// own consolidation, conflict reconciliation, and forgetting policies on top
// of what the harness stores.
//
// Two-tier mapping: the team estate lives in Memanto's agent id `librarian`
// (every shared write and every shared query targets it), so the
// private/team boundary stays an access decision in the harness. Private
// per-bot memory keeps flowing through the harness's notebook system —
// Memanto's per-agent scoping is available for later phases without a wire
// change.
//
// Failure posture: reads fail OPEN with an empty result (a memory outage
// must never brick a turn — the caller's hybrid retrieval falls back to
// the wiki BM25), writes fail LOUD (a distill that cannot persist returns
// an error, never a silent drop).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Northlatch-Labs-LLC/hivex/internal/config"
)

// memantoAgentID is the Memanto agent that holds the team's shared estate.
const memantoAgentID = "librarian"

// memantoReadTimeout keeps the read path fail-open fast: context assembly
// runs on the turn's critical path, so an unreachable instance must not
// stall it.
const memantoReadTimeout = 2 * time.Second

// memantoWriteTimeout gives the write path room to persist: writes run
// post-mutation (distill queue), off the hot path, and must succeed loudly
// rather than timing out quietly.
const memantoWriteTimeout = 10 * time.Second

// memantoClient is the narrow HTTP surface the backend depends on. Tests
// inject a fake; production talks to HIVEX_MEMANTO_URL.
type memantoClient interface {
	Ready(ctx context.Context) bool
	Answer(ctx context.Context, agentID, question string, topK int) ([]memantoMemory, error)
	BatchRemember(ctx context.Context, agentID, actor, content string) (string, error)
}

// memantoMemory is one memory item as Memanto's answer endpoint returns it.
// Field names are the integration contract (third_party/memanto openapi
// governs); the harness tolerates absent fields.
type memantoMemory struct {
	ID      string   `json:"id,omitempty"`
	Content string   `json:"content,omitempty"`
	Score   *float64 `json:"score,omitempty"`
}

// memantoHTTPClient is the production client: bearer auth against the
// configured instance. A missing URL makes every call fail fast (Ready
// false, reads empty, writes error) — the resolver steers the user to the
// env contract.
type memantoHTTPClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func memantoConfiguredURL() string {
	return strings.TrimRight(strings.TrimSpace(config.Getenv("HIVEX_MEMANTO_URL")), "/")
}

func memantoConfiguredKey() string {
	return strings.TrimSpace(config.Getenv("HIVEX_MEMANTO_API_KEY"))
}

// newMemantoHTTPClient builds the production client from the env contract.
func newMemantoHTTPClient() *memantoHTTPClient {
	return &memantoHTTPClient{
		baseURL: memantoConfiguredURL(),
		apiKey:  memantoConfiguredKey(),
		http:    &http.Client{},
	}
}

func (c *memantoHTTPClient) post(ctx context.Context, path string, body any, timeout time.Duration) ([]byte, int, error) {
	if c.baseURL == "" {
		return nil, 0, fmt.Errorf("HIVEX_MEMANTO_URL is not set")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	out, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, res.StatusCode, err
	}
	return out, res.StatusCode, nil
}

func (c *memantoHTTPClient) get(ctx context.Context, path string, timeout time.Duration) (int, error) {
	if c.baseURL == "" {
		return 0, fmt.Errorf("HIVEX_MEMANTO_URL is not set")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return 0, err
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	return res.StatusCode, nil
}

// Ready probes GET /ready — the same endpoint the deployment healthcheck
// uses, so "ready" means the same thing in both places.
func (c *memantoHTTPClient) Ready(ctx context.Context) bool {
	code, err := c.get(ctx, "/ready", memantoReadTimeout)
	return err == nil && code >= 200 && code < 300
}

// Answer asks one agent's estate a question and returns the top memories.
// The response shape tolerates both a bare array and an {"answer": [...]}
// envelope so a Memanto schema evolution degrades instead of breaking.
func (c *memantoHTTPClient) Answer(ctx context.Context, agentID, question string, topK int) ([]memantoMemory, error) {
	body := map[string]any{"question": question}
	if topK > 0 {
		body["top_k"] = topK
	}
	raw, code, err := c.post(ctx, "/"+agentID+"/answer", body, memantoReadTimeout)
	if err != nil {
		return nil, err
	}
	if code < 200 || code >= 300 {
		return nil, fmt.Errorf("memanto answer: status %d", code)
	}
	var envelope struct {
		Answer []memantoMemory `json:"answer"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && len(envelope.Answer) > 0 {
		return envelope.Answer, nil
	}
	var direct []memantoMemory
	if err := json.Unmarshal(raw, &direct); err != nil {
		return nil, fmt.Errorf("memanto answer: unparseable response: %w", err)
	}
	return direct, nil
}

// BatchRemember writes one memory into an agent's estate, returning the
// server-assigned memory id.
func (c *memantoHTTPClient) BatchRemember(ctx context.Context, agentID, actor, content string) (string, error) {
	body := map[string]any{
		"memories": []map[string]any{{
			"content": content,
			"metadata": map[string]any{
				"actor": actor, "source": "hivex", "verified": true,
			},
		}},
	}
	raw, code, err := c.post(ctx, "/"+agentID+"/batch-remember", body, memantoWriteTimeout)
	if err != nil {
		return "", err
	}
	if code < 200 || code >= 300 {
		return "", fmt.Errorf("memanto batch-remember: status %d: %s", code, truncate(string(raw), 200))
	}
	var out struct {
		IDs []string `json:"ids"`
		ID  string   `json:"id"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		// The write persisted server-side even when the echo is not
		// parseable; synthesize an identifier from the content hash.
		// Returning an error here would make the distill retry a write that
		// already happened — a double-write, not a retry.
		//nolint:nilerr // see comment: unparseable echo ≠ failed write
		return fmt.Sprintf("memanto-%s", slugify(truncate(content, 48))), nil
	}
	if len(out.IDs) > 0 {
		return out.IDs[0], nil
	}
	if out.ID != "" {
		return out.ID, nil
	}
	return fmt.Sprintf("memanto-%s", slugify(truncate(content, 48))), nil
}

// memantoClient injection (same pattern as the gbrain shared client):
// tests register a fake; production resolves lazily.
var (
	memantoClientMu sync.RWMutex
	memantoClientV  memantoClient
)

func setMemantoClient(c memantoClient) {
	memantoClientMu.Lock()
	defer memantoClientMu.Unlock()
	memantoClientV = c
}

func resolveMemantoClient() memantoClient {
	memantoClientMu.RLock()
	defer memantoClientMu.RUnlock()
	if memantoClientV != nil {
		return memantoClientV
	}
	return newMemantoHTTPClient()
}

// memantoMemoryBackend implements the memoryBackend interface.
type memantoMemoryBackend struct{}

func (memantoMemoryBackend) Kind() string { return config.MemoryBackendMemanto }
func (memantoMemoryBackend) Label() string {
	return config.MemoryBackendLabel(config.MemoryBackendMemanto)
}

// Ready reports whether the configured instance answers /ready. An unset
// URL is not ready — the resolver's NextStep steers to the env contract.
func (b memantoMemoryBackend) Ready() bool {
	return resolveMemantoClient().Ready(context.Background())
}

// MCPServer is nil: Memanto is a network service, not a subprocess — the
// bots reach it through this backend, never directly.
func (memantoMemoryBackend) MCPServer() (*memoryMCPServer, error) {
	return nil, nil
}

// FetchBrief renders the pre-task context block from the team estate,
// mirroring the gbrain block's shape so downstream prompt text stays
// uniform across backends. Fail-open: an unreachable instance renders "".
func (b memantoMemoryBackend) FetchBrief(ctx context.Context, notification string) string {
	query := strings.TrimSpace(notification)
	if query == "" {
		return ""
	}
	if len(query) > 400 {
		query = query[:400]
	}
	mems, err := resolveMemantoClient().Answer(ctx, memantoAgentID, query, 5)
	if err != nil || len(mems) == 0 {
		return ""
	}
	var lines []string
	lines = append(lines, "== MEMANTO CONTEXT ==")
	for _, m := range mems {
		snippet := strings.TrimSpace(strings.ReplaceAll(m.Content, "\n", " "))
		if snippet == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s", truncate(snippet, 220)))
		if len(lines) >= 4 {
			break
		}
	}
	if len(lines) == 1 {
		return ""
	}
	lines = append(lines, "== END MEMANTO CONTEXT ==")
	return strings.Join(lines, "\n")
}

// QueryShared returns the team-estate hits for hybrid retrieval. Errors
// propagate so the caller's fallback (wiki BM25) can take over — fail-open
// happens at the caller, where the fallback lives.
func (b memantoMemoryBackend) QueryShared(ctx context.Context, query string, limit int) ([]ScopedMemoryHit, error) {
	if limit <= 0 {
		limit = 5
	}
	mems, err := resolveMemantoClient().Answer(ctx, memantoAgentID, query, limit)
	if err != nil {
		return nil, err
	}
	hits := make([]ScopedMemoryHit, 0, len(mems))
	for _, m := range mems {
		snippet := strings.TrimSpace(strings.ReplaceAll(m.Content, "\n", " "))
		if snippet == "" {
			continue
		}
		hits = append(hits, ScopedMemoryHit{
			Scope:      "shared",
			Backend:    config.MemoryBackendMemanto,
			Identifier: strings.TrimSpace(m.ID),
			Title:      truncate(snippet, 80),
			Snippet:    truncate(snippet, 220),
			Source:     "memanto",
			Score:      m.Score,
		})
		if len(hits) >= limit {
			break
		}
	}
	return hits, nil
}

// WriteShared persists one verified memory into the team estate. Fail-loud:
// the distill path retries on error, never drops silently.
func (b memantoMemoryBackend) WriteShared(ctx context.Context, note SharedMemoryWrite) (string, error) {
	actor := slugify(firstNonEmpty(note.Actor, "hivex"))
	id, err := resolveMemantoClient().BatchRemember(ctx, memantoAgentID, actor, note.Content)
	if err != nil {
		return "", fmt.Errorf("write shared memanto memory: %w", err)
	}
	return id, nil
}
