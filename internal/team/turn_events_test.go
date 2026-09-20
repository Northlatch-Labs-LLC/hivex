package team

// turn_events_test.go — pins the AG-UI seam: every turn lifecycle maps to
// the canonical event order (RUN_STARTED → CUSTOM… → RUN_FINISHED/RUN_ERROR),
// observers never block the engine, and the SSE route streams real frames.

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// collectorObserver records every event it receives.
type collectorObserver struct {
	events chan TurnEvent
}

func newCollector() *collectorObserver {
	return &collectorObserver{events: make(chan TurnEvent, 32)}
}

func (c *collectorObserver) OnTurnEvent(e TurnEvent) {
	select {
	case c.events <- e:
	default:
	}
}

func (c *collectorObserver) waitFor(t *testing.T, want string) TurnEvent {
	t.Helper()
	select {
	case e := <-c.events:
		if e.Type != want {
			t.Fatalf("event type = %q, want %q", e.Type, want)
		}
		return e
	case <-time.After(3 * time.Second):
		t.Fatalf("event %q never arrived", want)
		return TurnEvent{}
	}
}

// TestTurnEventBusEmitsCanonicalOrder pins the AG-UI mapping: RUN_STARTED on
// open, CUSTOM per state change, RUN_FINISHED on settle / RUN_ERROR on fail.
func TestTurnEventBusEmitsCanonicalOrder(t *testing.T) {
	b := newTestBroker(t)
	c := newCollector()
	remove := b.AddTurnObserver(c)
	defer remove()

	id := b.TurnBegin("eng", "task-1", "general")
	e := c.waitFor(t, "RUN_STARTED")
	if e.TurnID != id || e.Bot != "eng" || e.State != TurnIntake {
		t.Fatalf("RUN_STARTED payload mismatch: %+v", e)
	}
	b.TurnTransition(id, TurnPolicyGate, "gate armed")
	c.waitFor(t, "CUSTOM")
	b.TurnTransition(id, TurnDispatch, "runner")
	c.waitFor(t, "CUSTOM")
	b.TurnSettle(id, "done")
	e = c.waitFor(t, "RUN_FINISHED")
	if e.State != TurnSettled {
		t.Fatalf("RUN_FINISHED must carry the settled state, got %+v", e)
	}

	// A failed turn maps to RUN_ERROR.
	id2 := b.TurnBegin("eng", "task-2", "general")
	c.waitFor(t, "RUN_STARTED")
	b.TurnFail(id2, "runner errored")
	c.waitFor(t, "RUN_ERROR")

	// Removal stops delivery.
	remove()
	id3 := b.TurnBegin("eng", "task-3", "general")
	select {
	case e := <-c.events:
		t.Fatalf("removed observer must not receive events, got %+v (turn %s)", e, id3)
	case <-time.After(200 * time.Millisecond):
	}
}

// TestTurnEventBusPanickingObserverIsContained pins the isolation
// invariant: a broken viewer never breaks the observed turn.
func TestTurnEventBusPanickingObserverIsContained(t *testing.T) {
	b := newTestBroker(t)
	b.AddTurnObserver(panicObserver{})
	c := newCollector()
	b.AddTurnObserver(c)

	id := b.TurnBegin("eng", "task-x", "general")
	c.waitFor(t, "RUN_STARTED")
	if !b.TurnTransition(id, TurnPolicyGate, "still works") {
		t.Fatal("engine must keep transitioning despite a panicking observer")
	}
	c.waitFor(t, "CUSTOM")
}

type panicObserver struct{}

func (panicObserver) OnTurnEvent(TurnEvent) { panic("observer exploded") }

// TestAguiTurnsRouteStreamsFrames pins the wire: the SSE route streams real
// `event:`/`data:` frames for a live turn, in canonical order.
func TestAguiTurnsRouteStreamsFrames(t *testing.T) {
	b := newTestBroker(t)
	srv := httptest.NewServer(http.HandlerFunc(b.handleAguiTurns))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content-type = %q, want text/event-stream", ct)
	}

	// Drive a turn while the stream is open; read frames as they arrive.
	id := b.TurnBegin("eng", "task-sse", "general")
	b.TurnTransition(id, TurnDispatch, "runner")
	b.TurnSettle(id, "done")

	want := []string{"RUN_STARTED", "CUSTOM", "RUN_FINISHED"}
	scanner := bufio.NewScanner(res.Body)
	deadline := time.Now().Add(3 * time.Second)
	for _, wantType := range want {
		var gotType, gotData string
		for time.Now().Before(deadline) {
			if !scanner.Scan() {
				t.Fatal("stream closed before all expected events arrived")
			}
			line := scanner.Text()
			if strings.HasPrefix(line, "event: ") {
				gotType = strings.TrimPrefix(line, "event: ")
			}
			if strings.HasPrefix(line, "data: ") {
				gotData = strings.TrimPrefix(line, "data: ")
			}
			if gotType != "" && gotData != "" {
				break
			}
		}
		if gotType != wantType {
			t.Fatalf("frame type = %q, want %q", gotType, wantType)
		}
		if !strings.Contains(gotData, `"turn_id":"`+id+`"`) {
			t.Fatalf("frame data must carry the turn id, got %s", gotData)
		}
		gotType, gotData = "", ""
	}
}
