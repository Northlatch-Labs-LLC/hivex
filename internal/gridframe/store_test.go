package gridframe

import (
	"errors"
	"testing"
)

// costRow builds a §5.1 cost-ledger row with the given id/date/amount.
func costRow(id, date, amount string) Row {
	sch, _ := Lookup(TCost)
	r := make(Row, len(sch.Headers))
	for i := range r {
		r[i] = ""
	}
	return r.With(sch, "txn_id", id).With(sch, "date", date).
		With(sch, "block", "D").With(sch, "category", "infra").
		With(sch, "description", "OVH node").With(sch, "amount_usd", amount).
		With(sch, "vendor", "OVH")
}

func TestAppendAddsRow(t *testing.T) {
	s := NewStore()
	if err := s.Append(TCost, costRow("CST-20260901-00", "2026-09-01", "85.05")); err != nil {
		t.Fatalf("Append: %v", err)
	}
	td, _ := s.Table(TCost)
	if len(td.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(td.Rows))
	}
}

func TestEditExistingRejected(t *testing.T) {
	s := NewStore()
	if err := s.Append(TCost, costRow("CST-20260901-00", "2026-09-01", "85.05")); err != nil {
		t.Fatalf("seed Append: %v", err)
	}
	// Same ID, changed amount: an edit attempt -> rejected.
	err := s.Append(TCost, costRow("CST-20260901-00", "2026-09-01", "99.99"))
	if !errors.Is(err, ErrEditExisting) {
		t.Fatalf("re-append same ID: err = %v, want ErrEditExisting", err)
	}
	td, _ := s.Table(TCost)
	if len(td.Rows) != 1 {
		t.Fatalf("rejected row must not land: rows = %d, want 1", len(td.Rows))
	}
}

func TestReversingRowAccepted(t *testing.T) {
	s := NewStore()
	orig := costRow("CST-20260901-00", "2026-09-01", "85.05")
	if err := s.Append(TCost, orig); err != nil {
		t.Fatalf("seed Append: %v", err)
	}
	// §5.1: correction = NEW row referencing the original ID.
	rev := costRow("CST-20260902-01", "2026-09-02", "-85.05").
		With(mustLookup(TCost), "description", "REVERSE CST-20260901-00")
	if err := s.AppendReversal(TCost, "CST-20260901-00", rev); err != nil {
		t.Fatalf("AppendReversal: %v", err)
	}
	td, _ := s.Table(TCost)
	if len(td.Rows) != 2 || len(td.Reversals) != 1 {
		t.Fatalf("rows=%d reversals=%d, want 2/1", len(td.Rows), len(td.Reversals))
	}
	if td.Reversals[0] != (Reversal{"CST-20260901-00", "CST-20260902-01"}) {
		t.Fatalf("reversal link = %+v", td.Reversals[0])
	}
}

func mustLookup(name string) Table {
	t, ok := Lookup(name)
	if !ok {
		panic("unknown table " + name)
	}
	return t
}

func TestReversalValidation(t *testing.T) {
	s := NewStore()
	orig := costRow("CST-20260901-00", "2026-09-01", "85.05")
	_ = s.Append(TCost, orig)
	rev := costRow("CST-20260902-01", "2026-09-02", "-85.05")

	// Reversing an unknown original.
	if err := s.AppendReversal(TCost, "CST-19990101-99", rev); !errors.Is(err, ErrUnknownOriginal) {
		t.Errorf("unknown original: err = %v, want ErrUnknownOriginal", err)
	}
	// Reversing row reusing an existing ID.
	bad := rev.With(mustLookup(TCost), "txn_id", "CST-20260901-00")
	if err := s.AppendReversal(TCost, "CST-20260901-00", bad); !errors.Is(err, ErrIDReuse) {
		t.Errorf("id reuse: err = %v, want ErrIDReuse", err)
	}
	// Reversal of a reversal.
	if err := s.AppendReversal(TCost, "CST-20260901-00", rev); err != nil {
		t.Fatalf("first reversal: %v", err)
	}
	rev2 := costRow("CST-20260903-02", "2026-09-03", "85.05")
	if err := s.AppendReversal(TCost, "CST-20260902-01", rev2); !errors.Is(err, ErrReversalOfReversal) {
		t.Errorf("reversal-of-reversal: err = %v, want ErrReversalOfReversal", err)
	}
}

func TestNoIDTableEditRejected(t *testing.T) {
	// aei-monthly has no ID col; same month key re-append = edit, rejected.
	sch, _ := Lookup(TAEI)
	row := make(Row, len(sch.Headers))
	row = row.With(sch, "month", "2026-09").With(sch, "revenue", "329.00").With(sch, "aei", "2.36")
	s := NewStore()
	if err := s.Append(TAEI, row); err != nil {
		t.Fatalf("Append aei: %v", err)
	}
	if err := s.Append(TAEI, row); !errors.Is(err, ErrEditExisting) {
		t.Fatalf("re-append same month: err = %v, want ErrEditExisting", err)
	}
}
