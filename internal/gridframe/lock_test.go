package gridframe

import (
	"errors"
	"testing"
)

// Spec §5.1: monthly close locks the month; writes to locked months rejected.
func TestLockedMonthWriteRejected(t *testing.T) {
	s := NewStore()
	s.LockMonth("2026-08")
	err := s.Append(TCost, costRow("CST-20260815-00", "2026-08-15", "85.05"))
	if !errors.Is(err, ErrLockedMonth) {
		t.Fatalf("locked-month append: err = %v, want ErrLockedMonth", err)
	}
	td, _ := s.Table(TCost)
	if len(td.Rows) != 0 {
		t.Fatalf("rejected row landed: rows = %d", len(td.Rows))
	}
}

func TestUnlockedMonthWriteAccepted(t *testing.T) {
	s := NewStore()
	s.LockMonth("2026-08")
	if err := s.Append(TCost, costRow("CST-20260901-00", "2026-09-01", "85.05")); err != nil {
		t.Fatalf("open-month append: %v", err)
	}
	if !s.IsLockedMonth("2026-08") || s.IsLockedMonth("2026-09") {
		t.Fatal("lock state wrong after append")
	}
}

// Reversal whose reversing row is dated in a locked month is rejected.
func TestLockedMonthReversalRejected(t *testing.T) {
	s := NewStore()
	if err := s.Append(TCost, costRow("CST-20260815-00", "2026-08-15", "85.05")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	s.LockMonth("2026-08")
	rev := costRow("CST-20260901-01", "2026-08-20", "-85.05")
	if err := s.AppendReversal(TCost, "CST-20260815-00", rev); !errors.Is(err, ErrLockedMonth) {
		t.Fatalf("locked reversal: err = %v, want ErrLockedMonth", err)
	}
}

// aei-monthly is month-keyed (DateCol "month"); locked month rejected.
func TestLockedAEIMonthRejected(t *testing.T) {
	s := NewStore()
	s.LockMonth("2026-08")
	sch, _ := Lookup(TAEI)
	row := make(Row, len(sch.Headers)).With(sch, "month", "2026-08").With(sch, "aei", "2.01")
	if err := s.Append(TAEI, row); !errors.Is(err, ErrLockedMonth) {
		t.Fatalf("locked aei month: err = %v, want ErrLockedMonth", err)
	}
}

// Week/quarter-keyed tables have no month: locks don't apply.
func TestLockIgnoresWeekQuarterTables(t *testing.T) {
	s := NewStore()
	s.LockMonth("2026-09")
	sch, _ := Lookup(TSprint)
	row := make(Row, len(sch.Headers)).With(sch, "sprint_id", "SPR-20260921-00").With(sch, "week", "S-2026-W39")
	if err := s.Append(TSprint, row); err != nil {
		t.Fatalf("sprint append under lock: %v", err)
	}
	sch2, _ := Lookup(TScore)
	row2 := make(Row, len(sch2.Headers)).With(sch2, "cycle", "2026-Q3").With(sch2, "agent_id", "ENG-00")
	if err := s.Append(TScore, row2); err != nil {
		t.Fatalf("scorecard append under lock: %v", err)
	}
}
