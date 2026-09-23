package gridframe

import (
	"testing"
	"time"
)

func TestDeclareRejectsUnknownSeverity(t *testing.T) {
	s := NewStore()
	if _, err := s.Declare(Exception{Date: "2026-09-23", Severity: "SEV9"}, time.Time{}); err == nil {
		t.Fatal("SEV9: want ErrBadSeverity")
	}
}

func TestDeclareCreatesOpenRow(t *testing.T) {
	s := NewStore()
	row, err := s.Declare(Exception{Date: "2026-09-23", Severity: Sev2,
		Dept: "PE", DeclaredBy: "ENG-04", Description: "release gate red"}, time.Time{})
	if err != nil {
		t.Fatalf("Declare: %v", err)
	}
	sch, _ := Lookup(TExc)
	if got := row.Get(sch, "exc_id"); got != "EXC-20260923-01" {
		t.Errorf("exc_id = %q, want EXC-20260923-01", got)
	}
	if got := row.Get(sch, "status"); got != "open" {
		t.Errorf("status = %q, want open", got)
	}
	// Second declaration same date: NN increments, IDs never reused.
	row2, _ := s.Declare(Exception{Date: "2026-09-23", Severity: Sev3,
		Dept: "OF", DeclaredBy: "OPS-02"}, time.Time{})
	if got := row2.Get(sch, "exc_id"); got != "EXC-20260923-02" {
		t.Errorf("second exc_id = %q, want EXC-20260923-02", got)
	}
}

// §4/§6: SEV1 acts under L3 first; the log lands within the 15-min window.
func TestSEV1PostLogWindow(t *testing.T) {
	e := Exception{Date: "2026-09-23", Severity: Sev1, Dept: "PE",
		DeclaredBy: "ENG-04", Description: "prod DB failover"}
	acted := time.Date(2026, 9, 23, 9, 0, 0, 0, time.UTC)
	if SEV1PostLogLate(e, acted, acted.Add(14*time.Minute)) {
		t.Error("post-log at 14 min flagged late, want on time (≤15)")
	}
	if !SEV1PostLogLate(e, acted, acted.Add(16*time.Minute)) {
		t.Error("post-log at 16 min not flagged late, want late (>15)")
	}
	// SEV2 is logged before the act: never judged by the SEV1 window.
	if SEV1PostLogLate(Exception{Severity: Sev2}, acted, acted.Add(time.Hour)) {
		t.Error("SEV2 judged by SEV1 window, want false")
	}
}

func TestResolve(t *testing.T) {
	s := NewStore()
	row, _ := s.Declare(Exception{Date: "2026-09-23", Severity: Sev2,
		Dept: "PE", DeclaredBy: "ENG-04"}, time.Time{})
	id := row[0]
	if err := s.Resolve(id, "failover completed, root cause patched"); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	sch, _ := Lookup(TExc)
	td, _ := s.Table(TExc)
	if got := td.Rows[0].Get(sch, "status"); got != "resolved" {
		t.Errorf("status = %q, want resolved", got)
	}
	if got := td.Rows[0].Get(sch, "resolution"); got != "failover completed, root cause patched" {
		t.Errorf("resolution = %q, want recorded text", got)
	}
	if err := s.Resolve(id, "again"); err != ErrNotPending {
		t.Errorf("re-resolve err = %v, want ErrNotPending", err)
	}
	if err := s.Resolve("EXC-19990101-99", ""); err != ErrExcNotFound {
		t.Errorf("unknown id err = %v, want ErrExcNotFound", err)
	}
}

// §6: every SEV1/SEV2 gets a postmortem ≤ 48h; SEV3 and rows carrying a
// postmortem_ref are not flagged.
func TestPostmortemDue(t *testing.T) {
	s := NewStore()
	s.Declare(Exception{Date: "2026-09-20", Severity: Sev1,
		Dept: "PE", DeclaredBy: "ENG-04"}, time.Time{}) // overdue, no ref
	s.Declare(Exception{Date: "2026-09-22", Severity: Sev2,
		Dept: "OF", DeclaredBy: "OPS-01"}, time.Time{}) // within 48h
	s.Declare(Exception{Date: "2026-09-18", Severity: Sev3,
		Dept: "PA", DeclaredBy: "PPL-01"}, time.Time{}) // SEV3: never due
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	due := s.PostmortemDue(now)
	if len(due) != 1 {
		t.Fatalf("postmortem-due rows = %d, want 1 (only the >48h SEV1)", len(due))
	}
	sch, _ := Lookup(TExc)
	if got := due[0].Get(sch, "exc_id"); got != "EXC-20260920-01" {
		t.Errorf("due exc_id = %q, want EXC-20260920-01", got)
	}
}
