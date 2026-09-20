package team

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Northlatch-Labs-LLC/hivex/internal/config"
)

// fakeCognee is the injected client for backend tests: scriptable answers,
// captured writes, no network.
type fakeCognee struct {
	ready       bool
	hits        []cogneeHit
	searchErr   error
	rememberErr error
	searches    []string
	remembers   []string
}

func (f *fakeCognee) Ready(context.Context) bool { return f.ready }

func (f *fakeCognee) Search(_ context.Context, query string, topK int) ([]cogneeHit, error) {
	f.searches = append(f.searches, query)
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	if topK > 0 && len(f.hits) > topK {
		return f.hits[:topK], nil
	}
	return f.hits, nil
}

func (f *fakeCognee) Remember(_ context.Context, content, actor string) (string, error) {
	f.remembers = append(f.remembers, actor+"|"+content)
	if f.rememberErr != nil {
		return "", f.rememberErr
	}
	return "cogrun-123", nil
}

// withCognee installs a fake client for the test's duration and restores
// the production resolver after.
func withCognee(t *testing.T, f *fakeCognee) {
	t.Helper()
	setCogneeClient(f)
	t.Cleanup(func() { setCogneeClient(nil) })
}

// TestCogneeReadyProbesTheInstance pins the readiness contract.
func TestCogneeReadyProbesTheInstance(t *testing.T) {
	b := cogneeMemoryBackend{}
	withCognee(t, &fakeCognee{ready: true})
	if !b.Ready() {
		t.Fatal("ready instance must report Ready")
	}
	withCognee(t, &fakeCognee{ready: false})
	if b.Ready() {
		t.Fatal("unreachable instance must not report Ready")
	}
}

// TestCogneeFetchBriefRendersTheContextBlock pins the read path's shape and
// the fail-open posture.
func TestCogneeFetchBriefRendersTheContextBlock(t *testing.T) {
	b := cogneeMemoryBackend{}

	withCognee(t, &fakeCognee{hits: []cogneeHit{
		{Text: "Verified: broker state writes must be atomic (tmp+rename)."},
		{Text: "The team prefers structured intake interviews."},
	}})
	brief := b.FetchBrief(context.Background(), "how should broker state be written?")
	if !strings.Contains(brief, "== COGNEE CONTEXT ==") || !strings.Contains(brief, "atomic") {
		t.Fatalf("brief must render the block with the memory content, got %q", brief)
	}
	if strings.Count(brief, "atomic") != 1 {
		t.Fatalf("brief must not duplicate memories, got %q", brief)
	}

	withCognee(t, &fakeCognee{searchErr: errors.New("instance down")})
	if brief := b.FetchBrief(context.Background(), "anything"); brief != "" {
		t.Fatalf("unreachable instance must render an empty brief, got %q", brief)
	}
	if brief := b.FetchBrief(context.Background(), "  "); brief != "" {
		t.Fatalf("empty query must render an empty brief, got %q", brief)
	}
}

// TestCogneeQuerySharedMapsHitsAndFailsLoud pins the retrieval contract.
func TestCogneeQuerySharedMapsHitsAndFailsLoud(t *testing.T) {
	b := cogneeMemoryBackend{}

	withCognee(t, &fakeCognee{hits: []cogneeHit{
		{Text: "atomic broker writes"},
		{Text: "structured intake"},
	}})
	hits, err := b.QueryShared(context.Background(), "broker writes", 1)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("limit must cap hits, got %d", len(hits))
	}
	if hits[0].Backend != config.MemoryBackendCognee || hits[0].Scope != "shared" || hits[0].Source != "cognee" {
		t.Fatalf("hit provenance mismatch: %+v", hits[0])
	}

	withCognee(t, &fakeCognee{searchErr: errors.New("instance down")})
	if _, err := b.QueryShared(context.Background(), "broker writes", 5); err == nil {
		t.Fatal("query errors must propagate for the caller's fallback")
	}
}

// TestCogneeWriteSharedPersistsLoudly pins the write path: the distill
// payload lands in the team dataset with actor attribution, and a failing
// instance is a loud error.
func TestCogneeWriteSharedPersistsLoudly(t *testing.T) {
	b := cogneeMemoryBackend{}

	f := &fakeCognee{ready: true}
	withCognee(t, f)
	id, err := b.WriteShared(context.Background(), SharedMemoryWrite{
		Actor: "researcher", Content: "Verified: migrations must be additive.",
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if id != "cogrun-123" {
		t.Fatalf("write must return the run reference, got %q", id)
	}
	if len(f.remembers) != 1 || !strings.Contains(f.remembers[0], "researcher|") {
		t.Fatalf("write must carry actor attribution, got %v", f.remembers)
	}

	withCognee(t, &fakeCognee{rememberErr: errors.New("instance down")})
	if _, err := b.WriteShared(context.Background(), SharedMemoryWrite{Content: "x"}); err == nil {
		t.Fatal("write failures must be loud")
	}
}

// TestCogneeResolverStatus pins the resolver contract end to end.
func TestCogneeResolverStatus(t *testing.T) {
	t.Setenv("HIVEX_MEMORY_BACKEND", config.MemoryBackendCognee)

	withCognee(t, &fakeCognee{ready: true})
	status := ResolveMemoryBackendStatus()
	if status.SelectedKind != config.MemoryBackendCognee || status.ActiveKind != config.MemoryBackendCognee {
		t.Fatalf("ready cognee must be selected+active, got %+v", status)
	}
	if !strings.Contains(status.Detail, "Cognee") {
		t.Fatalf("detail must name the backend, got %q", status.Detail)
	}

	withCognee(t, &fakeCognee{ready: false})
	status = ResolveMemoryBackendStatus()
	if status.ActiveKind != config.MemoryBackendNone {
		t.Fatalf("unreachable cognee must degrade to none, got %+v", status)
	}
	if !strings.Contains(status.NextStep, "HIVEX_COGNEE_URL") {
		t.Fatalf("next step must name the env contract, got %q", status.NextStep)
	}

	withCognee(t, &fakeCognee{ready: true})
	if kind := activeMemoryBackendKind(); kind != config.MemoryBackendCognee {
		t.Fatalf("active kind = %q, want cognee", kind)
	}
}
