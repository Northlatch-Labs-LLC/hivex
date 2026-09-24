package gridframe

// G4 Principal surface: HTTP handlers the office UI talks to (spec §9 P4
// surfaces). Digest fetch, approval decide, register add-row (append-only),
// CSV export (byte parity with csv.go), compliance timeline. Wiring into the
// broker mux happens in a later slice.

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

// AuthMiddleware wraps each handler, mirroring onboarding.RegisterRoutes
// (internal/onboarding/handlers.go:108). May be nil (tests).
type AuthMiddleware func(http.HandlerFunc) http.HandlerFunc

// ErrEngineOnly reports that aei-monthly appends are engine-only (G3 §5.3 constraint).
var ErrEngineOnly = errNew("gridframe: aei-monthly rows are appended by the AEI engine only")

// RegisterRoutes wires the G4 surface onto mux (Go 1.22 method patterns).
func RegisterRoutes(mux *http.ServeMux, s *Store, g *Gate, auth AuthMiddleware) {
	wrap := func(h http.HandlerFunc) http.HandlerFunc {
		if auth != nil {
			return auth(h)
		}
		return h
	}
	mux.HandleFunc("GET /gridframe/digest", wrap(makeHandleDigest(s, g)))
	mux.HandleFunc("POST /gridframe/approvals/decide", wrap(makeHandleDecide(s, g)))
	mux.HandleFunc("POST /gridframe/register/{table}/rows", wrap(makeHandleAddRow(s)))
	mux.HandleFunc("GET /gridframe/register/{table}", wrap(makeHandleListRows(s)))
	mux.HandleFunc("GET /gridframe/register/{table}/export", wrap(makeHandleExport(s)))
	mux.HandleFunc("GET /gridframe/compliance", wrap(makeHandleCompliance(s)))
}

// todayPT returns the spec's wall-clock date (America/Los_Angeles, §6).
func todayPT() string { return time.Now().In(CadenceLoc()).Format("2006-01-02") }

func rowMap(t Table, r Row) map[string]string {
	m := map[string]string{}
	for i, h := range t.Headers {
		if i < len(r) {
			m[h] = r[i]
		}
	}
	return m
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, ErrUnknownTable), errors.Is(err, ErrExcNotFound), errors.Is(err, ErrUnknownOriginal):
		status = http.StatusNotFound
	case errors.Is(err, ErrLockedMonth), errors.Is(err, ErrNotPending), errors.Is(err, ErrEditExisting), errors.Is(err, ErrIDReuse):
		status = http.StatusConflict
	case errors.Is(err, ErrWrongSigners), errors.Is(err, ErrNeedsHuman), errors.Is(err, ErrEngineOnly):
		status = http.StatusForbidden
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// ── Digest (spec §9.1: decisions as actionable items) ─────────────────

func makeHandleDigest(s *Store, g *Gate) http.HandlerFunc {
	digestJob, _ := FindJob("gridframe_principal_digest")
	return func(w http.ResponseWriter, r *http.Request) {
		date := r.URL.Query().Get("date")
		if date == "" {
			date = todayPT()
		}
		asch, _ := Lookup(TApprov)
		atd, _ := s.Table(TApprov)
		esc := map[string]bool{}
		for _, row := range g.EscalationDue(time.Now()) {
			esc[row.Get(asch, "item_id")] = true
		}
		decisions := []map[string]any{}
		for _, row := range atd.Rows {
			tier := row.Get(asch, "tier")
			if row.Get(asch, "status") != "pending" || (tier != Tier2 && tier != Tier3) {
				continue
			}
			item := map[string]any{"row": rowMap(asch, row), "escalated": esc[row.Get(asch, "item_id")]}
			a, ok := g.raised[row.Get(asch, "item_id")]
			if !ok {
				a = Action{} // $25 floor (gate.go CostOfDelay)
			}
			item["cost_of_delay_usd"] = CostOfDelay(a)
			decisions = append(decisions, item)
		}
		esch, _ := Lookup(TExc)
		etd, _ := s.Table(TExc)
		exceptions := []map[string]string{}
		for _, row := range etd.Rows {
			if row.Get(esch, "status") == "open" {
				exceptions = append(exceptions, rowMap(esch, row))
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"date": date, "artifact": RenderArtifact(digestJob.Artifact, time.Now()),
			"decisions": decisions, "open_exceptions": exceptions,
		})
	}
}

// ── Approval decide (spec §9.1: approve / reject / defer) ──────────────

type decideRequest struct {
	ItemID   string   `json:"item_id"`
	Decision string   `json:"decision"` // approve | reject | defer
	Signers  []string `json:"signers"`
	Human    bool     `json:"human"`
	Reason   string   `json:"reason"`
}

// requireSigners enforces the §4 signer rules on every decision type.
func requireSigners(tier string, signers []string) error {
	has := func(id string) bool {
		for _, s := range signers {
			if s == id {
				return true
			}
		}
		return false
	}
	switch tier {
	case Tier2:
		if !has("HOB-00") {
			return ErrWrongSigners
		}
	case Tier3:
		if !has("HOB-00") || !has("RSK-01") {
			return ErrWrongSigners
		}
	}
	return nil
}

func makeHandleDecide(s *Store, g *Gate) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req decideRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, errBad("bad JSON body: %v", err))
			return
		}
		if req.ItemID == "" || (req.Decision != "approve" && req.Decision != "reject" && req.Decision != "defer") {
			writeErr(w, errBad("item_id and decision (approve|reject|defer) are required"))
			return
		}
		if req.Decision == "approve" {
			if err := g.Approve(req.ItemID, ApprovalEvent{Signers: req.Signers, Human: req.Human}, todayPT()); err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"item_id": req.ItemID, "status": "approved"})
			return
		}
		sch, _ := Lookup(TApprov)
		td, err := s.Table(TApprov)
		if err != nil {
			writeErr(w, err)
			return
		}
		for i, row := range td.Rows {
			if row.Get(sch, "item_id") != req.ItemID {
				continue
			}
			if row.Get(sch, "status") != "pending" {
				writeErr(w, ErrNotPending)
				return
			}
			if err := requireSigners(row.Get(sch, "tier"), req.Signers); err != nil {
				writeErr(w, err)
				return
			}
			if a, ok := g.raised[req.ItemID]; ok && a.Reserve != "" && !req.Human {
				writeErr(w, ErrNeedsHuman)
				return
			}
			status := map[string]string{"reject": "rejected", "defer": "deferred"}[req.Decision]
			td.Rows[i] = row.With(sch, "status", status).
				With(sch, "approver", strings.Join(req.Signers, "+")).
				With(sch, "decision_date", todayPT())
			writeJSON(w, http.StatusOK, map[string]string{"item_id": req.ItemID, "status": status})
			return
		}
		writeErr(w, ErrUnknownOriginal)
	}
}

// ── Register add-row (spec §9.3 Board add-row forms) ──────────────────

// makeHandleListRows returns a register table's §5.1 headers + rows as
// JSON for the Board tabs (append-only view; mutations go through add-row).
func makeHandleListRows(s *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		table := r.PathValue("table")
		sch, ok := Lookup(table)
		if !ok {
			writeErr(w, errWrap(ErrUnknownTable, "%s", table))
			return
		}
		td, err := s.Table(table)
		if err != nil {
			writeErr(w, err)
			return
		}
		rows := make([]map[string]string, 0, len(td.Rows))
		for _, row := range td.Rows {
			rows = append(rows, rowMap(sch, row))
		}
		writeJSON(w, http.StatusOK, map[string]any{"table": table, "headers": sch.Headers, "rows": rows})
	}
}

type addRowRequest struct {
	Values    map[string]string `json:"values"`
	ReverseOf string            `json:"reverse_of"` // optional: correcting row ID
}

func makeHandleAddRow(s *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		table := r.PathValue("table")
		sch, ok := Lookup(table)
		if !ok {
			writeErr(w, errWrap(ErrUnknownTable, "%s", table))
			return
		}
		if table == TAEI {
			writeErr(w, ErrEngineOnly)
			return
		}
		var req addRowRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, errBad("bad JSON body: %v", err))
			return
		}
		row := make(Row, len(sch.Headers))
		for i, h := range sch.Headers {
			row[i] = req.Values[h]
		}
		if req.ReverseOf != "" {
			if err := s.AppendReversal(table, req.ReverseOf, row); err != nil {
				writeErr(w, err)
				return
			}
		} else if err := s.Append(table, row); err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"table": table, "row": rowMap(sch, row)})
	}
}

// ── CSV export (spec §9.3 CSV export parity) ──────────────────────────

func makeHandleExport(s *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		table := r.PathValue("table")
		td, err := s.Table(table)
		if err != nil {
			writeErr(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="`+table+`.csv"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(td.Export())
	}
}

// ── Compliance timeline (spec §9.4: overdue=red + exception link) ─────

func makeHandleCompliance(s *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		csch, _ := Lookup(TCompl)
		ctd, err := s.Table(TCompl)
		if err != nil {
			writeErr(w, err)
			return
		}
		today := todayPT()
		items := []map[string]any{}
		for _, row := range ctd.Rows {
			m := map[string]any{"row": rowMap(csch, row)}
			m["overdue"] = row.Get(csch, "due_date") != "" &&
				row.Get(csch, "due_date") < today &&
				row.Get(csch, "status") != "done"
			items = append(items, m)
		}
		esch, _ := Lookup(TExc)
		etd, _ := s.Table(TExc)
		open := []map[string]string{}
		for _, row := range etd.Rows {
			if row.Get(esch, "status") == "open" {
				open = append(open, rowMap(esch, row))
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "open_exceptions": open})
	}
}
