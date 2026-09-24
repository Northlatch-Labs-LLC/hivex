package gridframe

// Exception engine (§6): always-on, not scheduled. Any agent may declare
// SEV1 (act under L3 now; notify Principal ≤30 min; post-log ≤15 min),
// SEV2 (4h target), SEV3 (next business day). Declaring is never penalized;
// hiding is. SEV1 is the only tier whose log may land AFTER the act —
// within the 15-minute post-log window.

import "time"

// Severities per §6.
const (
	Sev1 = "SEV1"
	Sev2 = "SEV2"
	Sev3 = "SEV3"
)

// PostLogWindow bounds when a SEV1 emergency must be logged — action
// happens under L3 first; the log follows (§4 "log within 15 min after", §6).
const PostLogWindow = 15 * time.Minute

// ErrBadSeverity reports an unknown severity; the gate rejects it fail-closed.
var ErrBadSeverity = errNew("gridframe: severity must be SEV1, SEV2 or SEV3")

// Exception is the fact set for a declaration.
type Exception struct {
	Date        string // ISO date of declaration
	Severity    string
	Dept        string
	DeclaredBy  string // agent ID (declaring is never penalized)
	Description string
}

// Declare appends an OPEN exception-log row (EXC-YYYYMMDD-NN per §5.1) and
// returns it. For SEV1, actedAt is when the emergency action occurred (the
// row may follow the act); for SEV2/SEV3 pass time.Time{} — logged first,
// act follows. The row records status "open".
func (s *Store) Declare(e Exception, actedAt time.Time) (Row, error) {
	if e.Severity != Sev1 && e.Severity != Sev2 && e.Severity != Sev3 {
		return nil, errWrap(ErrBadSeverity, "%q", e.Severity)
	}
	sch, _ := Lookup(TExc)
	row := make(Row, len(sch.Headers))
	row = row.With(sch, "exc_id", s.nextID(TExc, "EXC", e.Date)).
		With(sch, "date", e.Date).
		With(sch, "severity", e.Severity).
		With(sch, "dept", e.Dept).
		With(sch, "declared_by", e.DeclaredBy).
		With(sch, "description", e.Description).
		With(sch, "status", "open")
	if err := s.Append(TExc, row); err != nil {
		return nil, err
	}
	return row, nil
}

// SEV1PostLogLate reports whether a SEV1 declaration missed the 15-minute
// post-log window (§4/§6): actedAt → loggedAt gap > PostLogWindow. SEV2/SEV3
// are logged before the act and never late by this rule. A late post-log is
// a hide-leaning violation: it feeds the scorecard, not a rejection — the
// emergency already happened, and declaring late beats not declaring.
func SEV1PostLogLate(e Exception, actedAt, loggedAt time.Time) bool {
	if e.Severity != Sev1 || actedAt.IsZero() || loggedAt.IsZero() {
		return false
	}
	return loggedAt.Sub(actedAt) > PostLogWindow
}

var ErrExcNotFound = errNew("gridframe: exception row not found")

// Resolve closes an open exception: sets status "resolved" and records the
// resolution text. Returns ErrExcNotFound for unknown IDs and
// ErrNotPending (gate.go) for already-resolved rows.
func (s *Store) Resolve(excID, resolution string) error {
	sch, _ := Lookup(TExc)
	td, err := s.Table(TExc)
	if err != nil {
		return err
	}
	for i, r := range td.Rows {
		if r.Get(sch, "exc_id") != excID {
			continue
		}
		if r.Get(sch, "status") != "open" {
			return ErrNotPending
		}
		td.Rows[i] = r.With(sch, "status", "resolved").With(sch, "resolution", resolution)
		return nil
	}
	return ErrExcNotFound
}

// PostmortemDueWindow is the deadline for a SEV1/SEV2 postmortem: ≤ 48h (§6).
const PostmortemDueWindow = 48 * time.Hour

// PostmortemDue lists open SEV1/SEV2 rows with no postmortem_ref whose
// declared date is older than the 48h window — these feed the next MBR.
func (s *Store) PostmortemDue(now time.Time) []Row {
	sch, _ := Lookup(TExc)
	td, _ := s.Table(TExc)
	var out []Row
	for _, r := range td.Rows {
		if r.Get(sch, "postmortem_ref") != "" {
			continue
		}
		if r.Get(sch, "severity") != Sev1 && r.Get(sch, "severity") != Sev2 {
			continue
		}
		declared, err := time.Parse("2006-01-02", r.Get(sch, "date"))
		if err != nil {
			continue
		}
		if now.Sub(declared) > PostmortemDueWindow {
			out = append(out, r)
		}
	}
	return out
}
