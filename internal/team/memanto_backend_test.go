package team

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Northlatch-Labs-LLC/hivex/internal/config"
)

// fakeMemanto is the injected client for backend tests: scriptable answers,
// captured writes, no network.
type fakeMemanto struct {
	ready     bool
	memories  []memantoMemory
	answerErr error
	writeErr  error
	answers   []string
	remembers []string
}

func (f *fakeMemanto) Ready(context.Context) bool { return f.ready }

func (f *fakeMemanto) Answer(_ context.Context, agentID, question string, topK int) ([]memantoMemory, error) {
	f.answers = append(f.answers, agentID+"|"+question)
	if f.answerErr != nil {
		return nil, f.answerErr
	}
	if topK > 0 && len(f.memories) > topK {
		return f.memories[:topK], nil
	}
	return f.memories, nil
}

func (f *fakeMemanto) BatchRemember(_ context.Context, agentID, actor, content string) (string, error) {
	f.remembers = append(f.remembers, agentID+"|"+actor+"|"+content)
	if f.writeErr != nil {
		return "", f.writeErr
	}
	return "mem-123", nil
}

// withMemanto installs a fake client for the test's duration and restores the
// production resolver after.
func withMemanto(t *testing.T, f *fakeMemanto) {
	t.Helper()
	setMemantoClient(f)
	t.Cleanup(func() { setMemantoClient(nil) })
}

// TestMemantoReadyProbesTheInstance pins the readiness contract: Ready is
// exactly the /ready probe of the configured instance.
func TestMemantoReadyProbesTheInstance(t *testing.T) {
	b := memantoMemoryBackend{}
	withMemanto(t, &fakeMemanto{ready: true})
	if !b.Ready() {
		t.Fatal("ready instance must report Ready")
	}
	withMemanto(t, &fakeMemanto{ready: false})
	if b.Ready() {
		t.Fatal("unreachable instance must not report Ready")
	}
}

// TestMemantoFetchBriefRendersTheContextBlock pins the read path's shape:
// memories render into the == MEMANTO CONTEXT == block (mirroring the gbrain
// block's shape), and an unreachable instance renders nothing — fail-open,
// never a turn-blocking error.
func TestMemantoFetchBriefRendersTheContextBlock(t *testing.T) {
	b := memantoMemoryBackend{}

	withMemanto(t, &fakeMemanto{memories: []memantoMemory{
		{ID: "m1", Content: "Verified: broker state writes must be atomic (tmp+rename)."},
		{ID: "m2", Content: "The team prefers structured intake interviews."},
	}})
	brief := b.FetchBrief(context.Background(), "how should broker state be written?")
	if !strings.Contains(brief, "== MEMANTO CONTEXT ==") || !strings.Contains(brief, "atomic") {
		t.Fatalf("brief must render the block with the memory content, got %q", brief)
	}
	if strings.Count(brief, "Verified:") != 1 {
		t.Fatalf("brief must not duplicate memories, got %q", brief)
	}

	withMemanto(t, &fakeMemanto{answerErr: errors.New("instance down")})
	if brief := b.FetchBrief(context.Background(), "anything"); brief != "" {
		t.Fatalf("unreachable instance must render an empty brief, got %q", brief)
	}
}

// TestMemantoQuerySharedMapsHitsAndFailsLoud pins the retrieval contract:
// memories map to scoped hits with the memanto backend marker, and errors
// PROPAGATE so the caller's hybrid fallback (wiki BM25) takes over — the
// fail-open happens at the caller, where the fallback lives.
func TestMemantoQuerySharedMapsHitsAndFailsLoud(t *testing.T) {
	b := memantoMemoryBackend{}

	withMemanto(t, &fakeMemanto{memories: []memantoMemory{
		{ID: "m1", Content: "atomic broker writes"},
		{ID: "m2", Content: "structured intake"},
	}})
	hits, err := b.QueryShared(context.Background(), "broker writes", 1)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("limit must cap hits, got %d", len(hits))
	}
	if hits[0].Backend != config.MemoryBackendMemanto || hits[0].Scope != "shared" || hits[0].Source != "memanto" {
		t.Fatalf("hit provenance mismatch: %+v", hits[0])
	}
	if !strings.Contains(hits[0].Snippet, "atomic") {
		t.Fatalf("hit snippet = %q", hits[0].Snippet)
	}

	withMemanto(t, &fakeMemanto{answerErr: errors.New("instance down")})
	if _, err := b.QueryShared(context.Background(), "broker writes", 5); err == nil {
		t.Fatal("query errors must propagate for the caller's fallback")
	}
}

// TestMemantoWriteSharedPersistsLoudly pins the write path: the distill
// payload lands in the team estate with actor attribution, and a failing
// instance is a loud error — never a silent drop.
func TestMemantoWriteSharedPersistsLoudly(t *testing.T) {
	b := memantoMemoryBackend{}

	f := &fakeMemanto{ready: true}
	withMemanto(t, f)
	id, err := b.WriteShared(context.Background(), SharedMemoryWrite{
		Actor: "researcher", Content: "Verified: migrations must be additive.",
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if id != "mem-123" {
		t.Fatalf("write must return the server id, got %q", id)
	}
	if len(f.remembers) != 1 || !strings.Contains(f.remembers[0], "librarian|researcher|") {
		t.Fatalf("write must target the team estate with actor attribution, got %v", f.remembers)
	}

	withMemanto(t, &fakeMemanto{writeErr: errors.New("instance down")})
	if _, err := b.WriteShared(context.Background(), SharedMemoryWrite{Content: "x"}); err == nil {
		t.Fatal("write failures must be loud")
	}
}

// TestMemantoResolverStatus pins the resolver's contract end to end: a
// selected+ready memanto backend is ACTIVE, and an unreachable instance
// degrades to none with a NextStep that names the env contract.
func TestMemantoResolverStatus(t *testing.T) {
	t.Setenv("HIVEX_MEMORY_BACKEND", config.MemoryBackendMemanto)

	withMemanto(t, &fakeMemanto{ready: true})
	status := ResolveMemoryBackendStatus()
	if status.SelectedKind != config.MemoryBackendMemanto || status.ActiveKind != config.MemoryBackendMemanto {
		t.Fatalf("ready memanto must be selected+active, got %+v", status)
	}
	if !strings.Contains(status.Detail, "Memanto") {
		t.Fatalf("detail must name the backend, got %q", status.Detail)
	}

	withMemanto(t, &fakeMemanto{ready: false})
	status = ResolveMemoryBackendStatus()
	if status.ActiveKind != config.MemoryBackendNone {
		t.Fatalf("unreachable memanto must degrade to none, got %+v", status)
	}
	if !strings.Contains(status.NextStep, "HIVEX_MEMANTO_URL") {
		t.Fatalf("next step must name the env contract, got %q", status.NextStep)
	}

	// Selection: the active backend for a ready memanto is the memanto
	// implementation, not the gbrain or none fallback.
	withMemanto(t, &fakeMemanto{ready: true})
	if kind := activeMemoryBackendKind(); kind != config.MemoryBackendMemanto {
		t.Fatalf("active kind = %q, want memanto", kind)
	}
}
