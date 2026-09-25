package team

// turn_live_http.go — GET /turns/live: the office's live-work surface.
//
// The turn engine already journals every turn as a first-class record
// (turn_engine.go); this handler is only the read side for the web office.
// The sidebar's working dot and the agent subspace's "Latest turn" line
// both poll it. No second system: it reads the same bounded journal the
// broker persists, collapses it to the most recent turn per agent, and
// enriches each with the member binding's model so a human sees WHO is
// working, in WHICH state, since WHEN, and at what token cost — without
// opening a stream or a new route tree.

import (
	"net/http"

	"github.com/Northlatch-Labs-LLC/hivex/internal/provider"
)

// liveTurnView is one agent's latest turn as the office renders it. Model
// comes from the member binding ("" when the agent runs the global runtime
// — omitted on the wire); Usage is present once the runner has stamped the
// stream close, so a mid-turn poll simply has no tokens yet.
type liveTurnView struct {
	Agent     string                `json:"agent"`
	State     string                `json:"state"`
	TaskID    string                `json:"task_id,omitempty"`
	StartedAt string                `json:"started_at"`
	UpdatedAt string                `json:"updated_at"`
	Model     string                `json:"model,omitempty"`
	Usage     *provider.ClaudeUsage `json:"usage,omitempty"`
}

// handleTurnsLive answers with the most recent turn per agent, newest
// first. Read-only over the journal: TurnRecords copies under the lock and
// MemberProviderBinding locks per lookup, so no broker mutex is ever held
// across the response write.
func (b *Broker) handleTurnsLive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	records := b.TurnRecords() // newest last
	turns := make([]liveTurnView, 0, len(records))
	seen := make(map[string]bool, len(records))
	for i := len(records) - 1; i >= 0; i-- {
		rec := records[i]
		if seen[rec.Bot] {
			continue
		}
		seen[rec.Bot] = true
		view := liveTurnView{
			Agent:     rec.Bot,
			State:     string(rec.State),
			TaskID:    rec.TaskID,
			StartedAt: rec.StartedAt,
			UpdatedAt: rec.UpdatedAt,
			Usage:     rec.Usage,
		}
		if binding := b.MemberProviderBinding(rec.Bot); binding.Model != "" {
			view.Model = binding.Model
		}
		turns = append(turns, view)
	}
	writeJSON(w, http.StatusOK, map[string]any{"turns": turns})
}
