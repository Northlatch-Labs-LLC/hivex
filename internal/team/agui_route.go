package team

// agui_route.go — GET /agui/turns: the AG-UI wire seam over the Turn Engine
// v2 event bus (turn_events.go). Streams every turn's lifecycle as SSE:
//   event: RUN_STARTED  | CUSTOM (state changes) | RUN_FINISHED | RUN_ERROR
//   data: <TurnEvent JSON>
// Broker-token gated (host trust — same boundary as /policy/*). Disconnects
// unregister the observer; a stalled client blocks only its own goroutine.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// aguiEventBufferSize bounds per-client queueing: a slow client drops
// oldest events instead of ballooning memory (events are also in the turn
// journal, so nothing is lost — only the live stream lags).
const aguiEventBufferSize = 64

// channelTurnObserver adapts a buffered channel to TurnObserver.
type channelTurnObserver struct {
	ch chan TurnEvent
}

func (o *channelTurnObserver) OnTurnEvent(e TurnEvent) {
	select {
	case o.ch <- e:
	default: // buffer full: drop the oldest, deliver the newest
		select {
		case <-o.ch:
		default:
		}
		select {
		case o.ch <- e:
		default:
		}
	}
}

// handleAguiTurns streams the AG-UI turn event feed as SSE.
func (b *Broker) handleAguiTurns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	obs := &channelTurnObserver{ch: make(chan TurnEvent, aguiEventBufferSize)}
	remove := b.AddTurnObserver(obs)
	defer remove()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	_, _ = fmt.Fprint(w, ": agui turn event stream open\n\n")
	flusher.Flush()

	keepAlive := time.NewTicker(turnEventStreamTick)
	defer keepAlive.Stop()
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-obs.ch:
			raw, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, raw)
			flusher.Flush()
		case <-keepAlive.C:
			_, _ = fmt.Fprint(w, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}
