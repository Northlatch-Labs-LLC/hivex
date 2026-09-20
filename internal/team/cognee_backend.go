package team

// cognee_backend.go — HIVEX_MEMORY_BACKEND=cognee: the self-hosted Cognee
// memory engine (Apache-2.0, topoteretes/cognee) as the shared organizational
// memory backend. Fully local: keyless GLiNER extraction + fastembed
// embeddings, no cloud dependency (see third_party/cognee/Dockerfile).
//
// Doctrine (docs/specs/core-loop.md): same as the memanto backend — the
// broker's distill/verification seams remain the only write path; context
// assembly remains the read path. Cognee adds its own graph build,
// hybrid retrieval, and session distillation on top.
//
// Failure posture: reads fail OPEN with an empty result (a memory outage
// must never brick a turn), writes fail LOUD (a distill that cannot persist
// returns an error, never a silent drop).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Northlatch-Labs-LLC/hivex/internal/config"
)

// cogneeDataset is the Cognee dataset holding the team's shared estate.
const cogneeDataset = "hivex-team-estate"

// cogneeReadTimeout bounds the read path; local retrieval is slower than a
// hosted vector store on first hit (model load), so this is roomier than
// memanto's 2s while still fail-open fast.
const cogneeReadTimeout = 5 * time.Second

// cogneeWriteTimeout bounds the write submit. /api/v1/add runs the ingest
// pipeline synchronously; steady state is seconds, but the FIRST run
// downloads the local extraction/embedding models — up to a couple of
// minutes, once. The distill path is off the turn's critical path.
const cogneeWriteTimeout = 120 * time.Second

// cogneeClient is the narrow HTTP surface the backend depends on. Tests
// inject a fake; production talks to HIVEX_COGNEE_URL.
type cogneeClient interface {
	Ready(ctx context.Context) bool
	Search(ctx context.Context, query string, topK int) ([]cogneeHit, error)
	Remember(ctx context.Context, content, actor string) (string, error)
}

// cogneeHit is one retrieved chunk. Field names tolerate the server's
// chunk schema (text|content) and optional score; absent fields degrade.
type cogneeHit struct {
	Text  string   `json:"text"`
	Title string   `json:"title"`
	Score *float64 `json:"score"`
}

func cogneeTextOf(raw map[string]any) string {
	for _, key := range []string{"text", "content", "chunk_text", "chunk"} {
		if s, ok := raw[key].(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	// v1.6 SearchResult nests the payload under "search_result".
	if nested, ok := raw["search_result"].(map[string]any); ok {
		if s := cogneeTextOf(nested); s != "" {
			return s
		}
	}
	if s, ok := raw["search_result"].(string); ok && strings.TrimSpace(s) != "" {
		return s
	}
	return ""
}

// cogneeHTTPClient is the production client against the configured instance.
// A missing URL fails every call fast (Ready false, reads empty, writes
// error) — the resolver's NextStep steers to the env contract.
type cogneeHTTPClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func cogneeConfiguredURL() string {
	return strings.TrimRight(strings.TrimSpace(config.Getenv("HIVEX_COGNEE_URL")), "/")
}

func cogneeConfiguredKey() string {
	return strings.TrimSpace(config.Getenv("HIVEX_COGNEE_API_KEY"))
}

func newCogneeHTTPClient() *cogneeHTTPClient {
	return &cogneeHTTPClient{baseURL: cogneeConfiguredURL(), apiKey: cogneeConfiguredKey(), http: &http.Client{}}
}

func (c *cogneeHTTPClient) do(ctx context.Context, method, path string, contentType string, body io.Reader, timeout time.Duration) ([]byte, int, error) {
	if c.baseURL == "" {
		return nil, 0, fmt.Errorf("HIVEX_COGNEE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, 0, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	out, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return out, res.StatusCode, err
	}
	return out, res.StatusCode, nil
}

// Ready probes GET /health — the same endpoint the compose healthcheck uses.
func (c *cogneeHTTPClient) Ready(ctx context.Context) bool {
	_, code, err := c.do(ctx, http.MethodGet, "/health", "", nil, cogneeReadTimeout)
	return err == nil && code >= 200 && code < 300
}

// Search runs pure retrieval over the team dataset — no server-side LLM
// (onlyContext: true): the harness's own model reasons over the context.
// The response envelope tolerates a bare array, {"items": …}, and
// {"search_results": …} so a schema evolution degrades instead of breaking.
func (c *cogneeHTTPClient) Search(ctx context.Context, query string, topK int) ([]cogneeHit, error) {
	if topK <= 0 {
		topK = 5
	}
	body, err := json.Marshal(map[string]any{
		"query":       query,
		"searchType":  "CHUNKS",
		"onlyContext": true,
		"topK":        topK,
		"datasets":    []string{cogneeDataset},
	})
	if err != nil {
		return nil, err
	}
	raw, code, err := c.do(ctx, http.MethodPost, "/api/v1/search", "application/json", bytes.NewReader(body), cogneeReadTimeout)
	if err != nil {
		return nil, err
	}
	if code < 200 || code >= 300 {
		return nil, fmt.Errorf("cognee search: status %d: %s", code, truncate(string(raw), 200))
	}
	var hits []cogneeHit
	// v1.6 reality: a bare JSON array of chunk strings.
	var chunks []string
	if err := json.Unmarshal(raw, &chunks); err == nil {
		for _, text := range chunks {
			if strings.TrimSpace(text) != "" {
				hits = append(hits, cogneeHit{Text: text})
				if len(hits) >= topK {
					break
				}
			}
		}
		return hits, nil
	}
	for _, envelope := range []string{`{"items":`, `{"search_results":`, `{"chunks":`} {
		if strings.HasPrefix(strings.TrimSpace(string(raw)), envelope[:len(envelope)-1]) {
			var wrapped struct {
				Items         []map[string]any `json:"items"`
				SearchResults []map[string]any `json:"search_results"`
				Chunks        []map[string]any `json:"chunks"`
			}
			if err := json.Unmarshal(raw, &wrapped); err != nil {
				continue
			}
			rows := wrapped.Items
			if len(rows) == 0 {
				rows = wrapped.SearchResults
			}
			if len(rows) == 0 {
				rows = wrapped.Chunks
			}
			for _, row := range rows {
				if text := cogneeTextOf(row); text != "" {
					hits = append(hits, cogneeHit{Text: text})
					if len(hits) >= topK {
						break
					}
				}
			}
			return hits, nil
		}
	}
	// Bare array of row objects.
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err == nil {
		for _, row := range rows {
			if text := cogneeTextOf(row); text != "" {
				hits = append(hits, cogneeHit{Text: text})
				if len(hits) >= topK {
					break
				}
			}
		}
		return hits, nil
	}
	// Single row object.
	var row map[string]any
	if err := json.Unmarshal(raw, &row); err == nil {
		if text := cogneeTextOf(row); text != "" {
			hits = append(hits, cogneeHit{Text: text})
		}
	}
	return hits, nil
}

// Remember submits one memory into the team dataset via /api/v1/add —
// empirically the ingest endpoint that runs the pipeline (v1 /remember only
// stores session records). Returns the pipeline run reference.
func (c *cogneeHTTPClient) Remember(ctx context.Context, content, actor string) (string, error) {
	var buf bytes.Buffer
	form := multipart.NewWriter(&buf)
	if err := form.WriteField("raw_data", content); err != nil {
		return "", err
	}
	if err := form.WriteField("datasetName", cogneeDataset); err != nil {
		return "", err
	}
	if err := form.Close(); err != nil {
		return "", err
	}
	raw, code, err := c.do(ctx, http.MethodPost, "/api/v1/add", form.FormDataContentType(), &buf, cogneeWriteTimeout)
	if err != nil {
		return "", err
	}
	if code < 200 || code >= 300 {
		return "", fmt.Errorf("cognee add: status %d: %s", code, truncate(string(raw), 200))
	}

	// Empirically required: add stores the data, but only cognify builds
	// the index that makes it retrievable (proven against the live server).
	cognifyBody, err := json.Marshal(map[string]any{"datasets": []string{cogneeDataset}})
	if err != nil {
		return "", err
	}
	cogRaw, cogCode, err := c.do(ctx, http.MethodPost, "/api/v1/cognify", "application/json", bytes.NewReader(cognifyBody), cogneeWriteTimeout)
	if err != nil {
		// The data landed; the index build is what failed — loud, but with
		// the ingestion id so the caller can see how far it got.
		return "", fmt.Errorf("cognee cognify after add: %w", err)
	}
	if cogCode < 200 || cogCode >= 300 {
		return "", fmt.Errorf("cognee cognify: status %d: %s", cogCode, truncate(string(cogRaw), 200))
	}

	var out struct {
		PipelineRunID string `json:"pipeline_run_id"`
		DatasetID     string `json:"dataset_id"`
		Status        string `json:"status"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		// Unparseable echo: the submit was accepted (2xx); synthesize a
		// stable id instead of double-writing on retry.
		//nolint:nilerr // see comment: unparseable echo ≠ failed write
		return "cognee-" + slugify(truncate(content, 48)), nil
	}
	for _, id := range []string{out.PipelineRunID, out.DatasetID} {
		if strings.TrimSpace(id) != "" {
			return id, nil
		}
	}
	return "cognee-" + slugify(truncate(content, 48)), nil
}

// cogneeClient injection (same pattern as the memanto client): tests
// register a fake; production resolves lazily.
var (
	cogneeClientMu sync.RWMutex
	cogneeClientV  cogneeClient
)

func setCogneeClient(c cogneeClient) {
	cogneeClientMu.Lock()
	defer cogneeClientMu.Unlock()
	cogneeClientV = c
}

func resolveCogneeClient() cogneeClient {
	cogneeClientMu.RLock()
	defer cogneeClientMu.RUnlock()
	if cogneeClientV != nil {
		return cogneeClientV
	}
	return newCogneeHTTPClient()
}

// cogneeMemoryBackend implements the memoryBackend interface.
type cogneeMemoryBackend struct{}

func (cogneeMemoryBackend) Kind() string { return config.MemoryBackendCognee }
func (cogneeMemoryBackend) Label() string {
	return config.MemoryBackendLabel(config.MemoryBackendCognee)
}

// Ready reports whether the configured instance answers /health. An unset
// URL is not ready — the resolver's NextStep steers to the env contract.
func (b cogneeMemoryBackend) Ready() bool {
	return resolveCogneeClient().Ready(context.Background())
}

// MCPServer is nil: Cognee is a network service, not a subprocess — the
// bots reach it through this backend, never directly.
func (cogneeMemoryBackend) MCPServer() (*memoryMCPServer, error) {
	return nil, nil
}

// FetchBrief renders the pre-task context block from the team estate,
// mirroring the other shared backends' shape. Fail-open: "" when the
// instance is unreachable or returns nothing.
func (b cogneeMemoryBackend) FetchBrief(ctx context.Context, notification string) string {
	query := strings.TrimSpace(notification)
	if query == "" {
		return ""
	}
	if len(query) > 400 {
		query = query[:400]
	}
	hits, err := resolveCogneeClient().Search(ctx, query, 5)
	if err != nil || len(hits) == 0 {
		return ""
	}
	var lines []string
	lines = append(lines, "== COGNEE CONTEXT ==")
	for _, h := range hits {
		snippet := strings.TrimSpace(strings.ReplaceAll(h.Text, "\n", " "))
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
	lines = append(lines, "== END COGNEE CONTEXT ==")
	return strings.Join(lines, "\n")
}

// QueryShared returns the team-estate hits for hybrid retrieval. Errors
// propagate so the caller's fallback (wiki BM25) can take over.
func (b cogneeMemoryBackend) QueryShared(ctx context.Context, query string, limit int) ([]ScopedMemoryHit, error) {
	if limit <= 0 {
		limit = 5
	}
	hits, err := resolveCogneeClient().Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]ScopedMemoryHit, 0, len(hits))
	for _, h := range hits {
		snippet := strings.TrimSpace(strings.ReplaceAll(h.Text, "\n", " "))
		if snippet == "" {
			continue
		}
		out = append(out, ScopedMemoryHit{
			Scope:   "shared",
			Backend: config.MemoryBackendCognee,
			Title:   truncate(snippet, 80),
			Snippet: truncate(snippet, 220),
			Source:  "cognee",
			Score:   h.Score,
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// WriteShared persists one verified memory into the team estate. Fail-loud:
// the distill path retries on error, never drops silently.
func (b cogneeMemoryBackend) WriteShared(ctx context.Context, note SharedMemoryWrite) (string, error) {
	actor := slugify(firstNonEmpty(note.Actor, "hivex"))
	id, err := resolveCogneeClient().Remember(ctx, note.Content, actor)
	if err != nil {
		return "", fmt.Errorf("write shared cognee memory: %w", err)
	}
	return id, nil
}
