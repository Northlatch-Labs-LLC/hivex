package team

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Northlatch-Labs-LLC/hivex/internal/provider"
)

// countingTurnMeter is a TurnMeter that counts calls; safe for concurrent
// use because the settle path may be driven from worker goroutines.
type countingTurnMeter struct {
	mu    sync.Mutex
	calls int
	fails int
	byBot map[string]int
}

func (m *countingTurnMeter) MeterTurn(bot, taskID string, failed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if failed {
		m.fails++
	}
	if m.byBot == nil {
		m.byBot = map[string]int{}
	}
	m.byBot[bot]++
}

// TestTurnEngineLifecycleRecordsTheTrail pins the happy path: begin opens in
// intake, every transition lands in the audit trail with its detail, and the
// record is visible through TurnRecords.
func TestTurnEngineLifecycleRecordsTheTrail(t *testing.T) {
	b := newTestBroker(t)
	id := b.TurnBegin("researcher", "task-9", "general")
	b.TurnTransition(id, TurnPolicyGate, "gate armed")
	b.TurnTransition(id, TurnContextAssembly, "packet assembled")
	b.TurnTransition(id, TurnDispatch, "claude runner")
	b.TurnTransition(id, TurnExecuting, "runner in flight")
	b.TurnTransition(id, TurnVerifying, "runner returned")
	b.TurnSettle(id, "settled")

	records := b.TurnRecords()
	if len(records) != 1 {
		t.Fatalf("expected one record, got %d", len(records))
	}
	rec := records[0]
	if rec.Bot != "researcher" || rec.TaskID != "task-9" || rec.Channel != "general" {
		t.Fatalf("record header mismatch: %+v", rec)
	}
	if rec.State != TurnSettled {
		t.Fatalf("state = %q, want %q", rec.State, TurnSettled)
	}
	// intake + five transitions + settle = seven trail entries.
	if len(rec.Transitions) != 7 {
		t.Fatalf("trail length = %d, want 7: %+v", len(rec.Transitions), rec.Transitions)
	}
	if !strings.Contains(rec.Transitions[len(rec.Transitions)-1].Detail, "settled") {
		t.Fatalf("last transition detail = %q", rec.Transitions[len(rec.Transitions)-1].Detail)
	}
}

// TestTurnEngineTerminalIsImmutable pins that history never reopens: a
// transition after settle is dropped, and the meter is not re-fired.
func TestTurnEngineTerminalIsImmutable(t *testing.T) {
	b := newTestBroker(t)
	m := &countingTurnMeter{}
	b.SetTurnMeter(m)
	id := b.TurnBegin("researcher", "", "")
	b.TurnFail(id, "runner error")
	if m.calls != 1 {
		t.Fatalf("meter fired %d times on close, want 1", m.calls)
	}
	b.TurnSettle(id, "late attempt to settle a failed turn")
	b.TurnTransition(id, TurnDispatch, "late attempt")
	if m.calls != 1 {
		t.Fatalf("meter re-fired on a terminal record: %d", m.calls)
	}
	rec := b.TurnRecords()[0]
	if rec.State != TurnFailed {
		t.Fatalf("state drifted to %q after late transitions", rec.State)
	}
	if len(rec.Transitions) != 2 {
		t.Fatalf("late transitions leaked into the trail: %+v", rec.Transitions)
	}
}

// TestTurnEngineMetersExactlyOncePerTurn pins the settle-phase contract the
// tier enforcement (P6) hangs off: one turn lifecycle, one meter event.
func TestTurnEngineMetersExactlyOncePerTurn(t *testing.T) {
	b := newTestBroker(t)
	m := &countingTurnMeter{}
	b.SetTurnMeter(m)

	id := b.TurnBegin("researcher", "task-1", "general")
	b.TurnSettle(id, "settled")

	failed := b.TurnBegin("researcher", "task-2", "general")
	b.TurnFail(failed, "timeout")

	if m.calls != 2 {
		t.Fatalf("meter fired %d times, want 2 (one per turn)", m.calls)
	}
	if m.fails != 1 {
		t.Fatalf("failed turns metered: %d, want 1", m.fails)
	}
	if m.byBot["researcher"] != 2 {
		t.Fatalf("per-bot meter count = %d, want 2", m.byBot["researcher"])
	}
}

// TestTurnEngineNilMeterIsSafe pins that metering is optional: a broker with
// no meter settles turns without error.
func TestTurnEngineNilMeterIsSafe(t *testing.T) {
	b := newTestBroker(t)
	id := b.TurnBegin("researcher", "", "")
	b.TurnSettle(id, "settled")
	if rec := b.TurnRecords()[0]; rec.State != TurnSettled {
		t.Fatalf("state = %q, want settled", rec.State)
	}
}

// TestTurnEngineResumeClosesInterruptedTurns pins the crash story: any record
// a restart finds mid-flight is closed as failed with a reason naming the
// state it was found in. Terminal records are left untouched.
func TestTurnEngineResumeClosesInterruptedTurns(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	records := []TurnRecord{
		{ID: "t-1", Bot: "researcher", State: TurnDispatch, StartedAt: now, UpdatedAt: now},
		{ID: "t-2", Bot: "writer", State: TurnVerifying, StartedAt: now, UpdatedAt: now},
		{ID: "t-3", Bot: "researcher", State: TurnSettled, StartedAt: now, UpdatedAt: now},
		{ID: "t-4", Bot: "researcher", State: TurnFailed, StartedAt: now, UpdatedAt: now},
	}
	out := turnResumeInterruptedLocked(records)
	byID := map[string]TurnRecord{}
	for _, r := range out {
		byID[r.ID] = r
	}
	for _, id := range []string{"t-1", "t-2"} {
		rec := byID[id]
		if rec.State != TurnFailed {
			t.Fatalf("%s state = %q, want failed (interrupted)", id, rec.State)
		}
		last := rec.Transitions[len(rec.Transitions)-1]
		if !strings.Contains(last.Detail, "interrupted by broker restart") {
			t.Fatalf("%s close detail = %q", id, last.Detail)
		}
	}
	if byID["t-3"].State != TurnSettled || byID["t-4"].State != TurnFailed {
		t.Fatal("resume must not touch already-terminal records")
	}
}

// TestTurnEngineJournalIsBounded pins the journal ring: only the latest
// maxTurnRecords turns are kept, so a long-lived office cannot grow the
// persisted state without limit.
func TestTurnEngineJournalIsBounded(t *testing.T) {
	b := newTestBroker(t)
	for i := 0; i < maxTurnRecords+10; i++ {
		id := b.TurnBegin("researcher", "", "")
		b.TurnSettle(id, "settled")
	}
	records := b.TurnRecords()
	if len(records) > maxTurnRecords {
		t.Fatalf("journal holds %d records, cap is %d", len(records), maxTurnRecords)
	}
	if len(records) < maxTurnRecords {
		t.Fatalf("journal lost records early: %d", len(records))
	}
}

// A runner's stream usage lands on the turn record — per-turn cost
// attribution reads the record, not a nearest-message heuristic.
func TestStampTurnUsage(t *testing.T) {
	b := newTestBroker(t)
	id := b.TurnBegin("cos", "", "cos__human")
	b.StampTurnUsage(id, provider.ClaudeUsage{InputTokens: 11, OutputTokens: 7})
	b.TurnSettle(id, "done")
	for _, rec := range b.TurnRecords() {
		if rec.ID == id {
			if rec.Usage == nil || rec.Usage.InputTokens != 11 || rec.Usage.OutputTokens != 7 {
				t.Fatalf("usage not stamped: %+v", rec.Usage)
			}
			return
		}
	}
	t.Fatal("turn record not found")
}
