package team

// turn_events.go — the observable turn-event bus (the AG-UI seam): every
// Turn Engine v2 state change is emitted as a typed event that observers
// (the SSE route, future wire protocols) can stream. AG-UI vocabulary:
// RUN_STARTED on open, CUSTOM per state change, RUN_FINISHED on settle,
// RUN_ERROR on fail.
//
// Contract: emissions are fire-and-forget — a slow or broken observer must
// never stall the turn it observes. Observers are copied under the broker
// lock and invoked outside it.

import (
	"context"
	"time"
)

// TurnEvent is one observable turn lifecycle event.
type TurnEvent struct {
	TurnID  string    `json:"turn_id"`
	Bot     string    `json:"agent"`
	TaskID  string    `json:"task_id,omitempty"`
	Channel string    `json:"channel,omitempty"`
	Type    string    `json:"type"` // RUN_STARTED | CUSTOM | RUN_FINISHED | RUN_ERROR
	State   TurnState `json:"state"`
	Detail  string    `json:"detail,omitempty"`
	At      string    `json:"at"`
}

// TurnObserver receives turn events. Implementations must be safe for
// concurrent use and must never block the caller.
type TurnObserver interface {
	OnTurnEvent(TurnEvent)
}

// AddTurnObserver registers an observer; returns a remove func.
func (b *Broker) AddTurnObserver(o TurnObserver) func() {
	if b == nil {
		return func() {}
	}
	b.mu.Lock()
	b.turnObservers = append(b.turnObservers, o)
	b.mu.Unlock()
	return func() { b.RemoveTurnObserver(o) }
}

// RemoveTurnObserver unregisters an observer (idempotent).
func (b *Broker) RemoveTurnObserver(o TurnObserver) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := range b.turnObservers {
		if b.turnObservers[i] == o {
			b.turnObservers = append(b.turnObservers[:i], b.turnObservers[i+1:]...)
			return
		}
	}
}

// emitTurnEvent fans an event out to the registered observers outside the
// broker lock. Nil-safe; observer panics are recovered — the observed turn
// must never break because a viewer did.
func (b *Broker) emitTurnEvent(e TurnEvent) {
	if b == nil {
		return
	}
	b.mu.Lock()
	observers := make([]TurnObserver, len(b.turnObservers))
	copy(observers, b.turnObservers)
	b.mu.Unlock()
	for _, o := range observers {
		emitTurnEventTo(o, e)
	}
}

func emitTurnEventTo(o TurnObserver, e TurnEvent) {
	defer func() { _ = recover() }()
	o.OnTurnEvent(e)
}

// turnEventFor builds the AG-UI event for a state change.
func turnEventFor(rec TurnRecord, state TurnState, detail string) TurnEvent {
	eventType := "CUSTOM"
	switch state {
	case TurnSettled:
		eventType = "RUN_FINISHED"
	case TurnFailed:
		eventType = "RUN_ERROR"
	}
	return TurnEvent{
		TurnID:  rec.ID,
		Bot:     rec.Bot,
		TaskID:  rec.TaskID,
		Channel: rec.Channel,
		Type:    eventType,
		State:   state,
		Detail:  detail,
		At:      time.Now().UTC().Format(time.RFC3339),
	}
}

// contextWithTurnEventTimeout is a helper for SSE writers that poll.
var turnEventStreamTick = 15 * time.Second

func turnEventKeepAlive(ctx context.Context) <-chan time.Time {
	ch := make(chan time.Time, 1)
	go func() {
		t := time.NewTicker(turnEventStreamTick)
		defer t.Stop()
		select {
		case t1 := <-t.C:
			select {
			case ch <- t1:
			case <-ctx.Done():
			}
		case <-ctx.Done():
		}
	}()
	return ch
}
