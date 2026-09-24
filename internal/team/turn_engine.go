package team

// turn_engine.go — the explicit Turn Engine v2 state machine.
//
// Before this file a turn's lifecycle was implicit: scattered across the
// launcher dispatch, the ledger write, the recovery guards, and the distill
// queue — each piece correct, but nothing recorded the turn ITSELF as a
// first-class, inspectable object. The engine makes every turn exactly that:
// a persisted record with a typed state, an audited transition trail, and a
// crash story — a broker restarted mid-turn finds the record in a non-terminal
// state and closes it honestly as interrupted, instead of leaving a ghost.
//
// Canonical flow (each state is recorded as it is ENTERED, with the detail
// line saying why):
//
//   intake -> policy_gate -> context_assembly -> dispatch -> executing
//          -> verifying -> distilling -> settled | failed
//
// The launcher wrapper (defaultHeadlessCodexRunTurn) drives the states it can
// observe directly; the deeper seams (ledger write, distill queue) hook in
// the later states via TurnDistillQueued/TurnSettleLedger so the record tells
// the whole story without the runners knowing the engine exists. Fresh
// sessions, verification gates, and the ledger stay EXACTLY as they were —
// the engine adds the observability and the resume story on top, per the
// "EXTEND-don't-duplicate" house rule.

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Northlatch-Labs-LLC/hivex/internal/provider"
)

// TurnState is one stage of the canonical turn lifecycle.
type TurnState string

const (
	TurnIntake          TurnState = "intake"
	TurnPolicyGate      TurnState = "policy_gate"
	TurnContextAssembly TurnState = "context_assembly"
	TurnDispatch        TurnState = "dispatch"
	TurnExecuting       TurnState = "executing"
	TurnVerifying       TurnState = "verifying"
	TurnDistilling      TurnState = "distilling"
	TurnSettled         TurnState = "settled"
	TurnFailed          TurnState = "failed"
)

// turnTerminal reports whether a state ends the turn. Terminal records are
// immutable — a late transition is dropped and logged rather than reopening
// history.
func turnTerminal(s TurnState) bool {
	return s == TurnSettled || s == TurnFailed
}

// maxTurnTransitions bounds one turn's audit trail. The states are few; a
// trail that exceeds this cap means a loop, and the record should say so
// instead of growing unbounded.
const maxTurnTransitions = 64

// maxTurnRecords bounds the persisted turn journal. The ledger on each task
// carries the long-lived working memory; this journal is the recent-turn
// observability surface, so a ring of the latest N turns is the contract.
const maxTurnRecords = 200

// TurnTransition is one state change: when, to where, and why.
type TurnTransition struct {
	At     string    `json:"at"`
	State  TurnState `json:"state"`
	Detail string    `json:"detail,omitempty"`
}

// TurnRecord is one bot turn as a first-class object.
type TurnRecord struct {
	ID          string           `json:"id"`
	Bot         string           `json:"agent"`
	TaskID      string           `json:"task_id,omitempty"`
	Channel     string           `json:"channel,omitempty"`
	State       TurnState        `json:"state"`
	StartedAt   string           `json:"started_at"`
	UpdatedAt   string           `json:"updated_at"`
	Transitions []TurnTransition `json:"transitions,omitempty"`
	// Usage is the turn's token truth, stamped by the runner as the stream
	// closes. Per-turn cost attribution and the live-work surface read it.
	Usage *provider.ClaudeUsage `json:"usage,omitempty"`
}

// TurnMeter is the settle-phase usage meter hook (tier enforcement, P6).
// The broker installs one implementation; nil means metering disabled — the
// Free tier's cap enforcement and the Stripe meter events both hang off it.
type TurnMeter interface {
	// MeterTurn is called once per turn as it settles, with the bot and the
	// turn's terminal state. Implementations must be safe for concurrent use
	// and must never block the settle path (fire-and-forget internally).
	MeterTurn(bot, taskID string, failed bool)
}

// StampTurnUsage records the turn's token usage from the runner's stream
// close. Nil-safe and idempotent (last write wins while the turn is open).
func (b *Broker) StampTurnUsage(id string, usage provider.ClaudeUsage) {
	if usage.InputTokens == 0 && usage.OutputTokens == 0 && usage.CacheReadTokens == 0 && usage.CacheCreationTokens == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := range b.turnRecords {
		if b.turnRecords[i].ID == id {
			u := usage
			b.turnRecords[i].Usage = &u
			return
		}
	}
}

// TurnBegin opens a turn in intake and persists the record. Nil-safe: a
// brokerless launcher (tests, degraded modes) gets a record ID that no-ops.
func (b *Broker) TurnBegin(bot, taskID, channel string) string {
	id := fmt.Sprintf("turn-%s-%d", strings.ToLower(strings.TrimSpace(bot)), time.Now().UnixNano())
	if b == nil {
		return id
	}
	now := time.Now().UTC().Format(time.RFC3339)
	rec := TurnRecord{
		ID: id, Bot: bot, TaskID: taskID, Channel: channel,
		State: TurnIntake, StartedAt: now, UpdatedAt: now,
		Transitions: []TurnTransition{{At: now, State: TurnIntake, Detail: "turn opened"}},
	}
	b.mu.Lock()
	b.turnRecords = append(b.turnRecords, rec)
	if excess := len(b.turnRecords) - maxTurnRecords; excess > 0 {
		b.turnRecords = b.turnRecords[excess:]
	}
	b.mu.Unlock()
	ev := turnEventFor(rec, TurnIntake, "turn opened")
	ev.Type = "RUN_STARTED"
	b.emitTurnEvent(ev)
	return id
}

// TurnTransition moves a turn to the next state, appending the audit trail.
// Reports whether the transition applied: transitions on terminal records
// are dropped (logged) — history stays immutable — and unknown IDs no-op; the
// journal is an observability surface and a lost record must never break the
// turn it observes.
func (b *Broker) TurnTransition(id string, state TurnState, detail string) bool {
	if b == nil || strings.TrimSpace(id) == "" {
		return false
	}
	now := time.Now().UTC().Format(time.RFC3339)
	b.mu.Lock()
	applied := false
	var appliedRec TurnRecord
	for i := range b.turnRecords {
		if b.turnRecords[i].ID != id {
			continue
		}
		rec := &b.turnRecords[i]
		if turnTerminal(rec.State) {
			log.Printf("turn engine: dropping transition %q on terminal turn %s (%s)", state, id, rec.State)
			break
		}
		if len(rec.Transitions) >= maxTurnTransitions {
			log.Printf("turn engine: transition cap hit for turn %s; recording %s without trail", id, state)
			rec.State = state
			rec.UpdatedAt = now
			applied = true
			appliedRec = *rec
			break
		}
		rec.State = state
		rec.UpdatedAt = now
		rec.Transitions = append(rec.Transitions, TurnTransition{At: now, State: state, Detail: detail})
		applied = true
		appliedRec = *rec
		break
	}
	b.mu.Unlock()
	if applied {
		b.emitTurnEvent(turnEventFor(appliedRec, state, detail))
	}
	return applied
}

// TurnFail closes a turn as failed — the runner errored, the turn timed out,
// or the broker restart found it mid-flight. Terminal. The meter fires only
// when the transition applied, so a duplicate close never double-meters.
func (b *Broker) TurnFail(id, detail string) {
	if b.TurnTransition(id, TurnFailed, detail) {
		b.meterTurn(id)
	}
}

// TurnSettle closes a turn as settled — the ledger recorded the outcome and
// the scheduler is free. Terminal. Fires the meter hook exactly once per
// turn lifecycle.
func (b *Broker) TurnSettle(id, detail string) {
	if b.TurnTransition(id, TurnSettled, detail) {
		b.meterTurn(id)
	}
}

// meterTurn emits the settle-phase meter event for a terminal turn.
// The meter is nil-safe; a nil meter means metering is disabled.
func (b *Broker) meterTurn(id string) {
	if b == nil || b.turnMeter == nil {
		return
	}
	b.mu.Lock()
	bot, taskID, failed := "", "", false
	for i := range b.turnRecords {
		if b.turnRecords[i].ID == id {
			bot, taskID, failed = b.turnRecords[i].Bot, b.turnRecords[i].TaskID, b.turnRecords[i].State == TurnFailed
			break
		}
	}
	m := b.turnMeter
	b.mu.Unlock()
	if m != nil && bot != "" {
		m.MeterTurn(bot, taskID, failed)
	}
}

// SetTurnMeter installs the settle-phase meter (tier enforcement). Nil-safe.
func (b *Broker) SetTurnMeter(m TurnMeter) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.turnMeter = m
	b.mu.Unlock()
}

// TurnRecords returns a copy of the recent-turn journal (newest last).
// Locks b.mu.
func (b *Broker) TurnRecords() []TurnRecord {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]TurnRecord, len(b.turnRecords))
	copy(out, b.turnRecords)
	return out
}

// turnResumeInterrupted closes any record left in a non-terminal state by a
// previous process — the crash story. Called from the broker load path with
// b.mu held; every interrupted turn is closed as failed with a reason that
// names the state it was found in, so the journal tells the truth about what
// a restart cut short.
func turnResumeInterruptedLocked(records []TurnRecord) []TurnRecord {
	now := time.Now().UTC().Format(time.RFC3339)
	for i := range records {
		rec := &records[i]
		if turnTerminal(rec.State) {
			continue
		}
		found := rec.State
		rec.State = TurnFailed
		rec.UpdatedAt = now
		rec.Transitions = append(rec.Transitions, TurnTransition{
			At: now, State: TurnFailed,
			Detail: fmt.Sprintf("interrupted by broker restart while %s", found),
		})
	}
	return records
}
