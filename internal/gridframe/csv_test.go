package gridframe

import (
	"os"
	"path/filepath"
	"testing"
)

// §10 acceptance test 2: export of each of the 8 tables is byte-compatible
// with the shipped seed CSVs (exact §5.1 headers, quoting, and terminators).
func TestRoundTripSeedsByteCompatible(t *testing.T) {
	for _, name := range TableNames() {
		raw, err := os.ReadFile(filepath.Join(seedsDir, name+".csv"))
		if err != nil {
			t.Fatalf("read seed %s: %v", name, err)
		}
		td, err := ParseTable(name, raw)
		if err != nil {
			t.Errorf("%s: ParseTable: %v", name, err)
			continue
		}
		if got := td.Export(); string(got) != string(raw) {
			t.Errorf("%s: round-trip not byte-compatible\n got %q\nwant %q", name, got, raw)
		}
	}
}

// Import through Store, then export: store rows re-render byte-identically.
func TestStoreImportExportRoundTrip(t *testing.T) {
	for _, name := range TableNames() {
		raw, err := os.ReadFile(filepath.Join(seedsDir, name+".csv"))
		if err != nil {
			t.Fatalf("read seed %s: %v", name, err)
		}
		s := NewStore()
		if err := s.ImportTable(name, raw); err != nil {
			t.Errorf("%s: ImportTable: %v", name, err)
			continue
		}
		td, _ := s.Table(name)
		if got := td.Export(); string(got) != string(raw) {
			t.Errorf("%s: store round-trip differs\n got %q\nwant %q", name, got, raw)
		}
	}
}

// Import must reject a header that is not the byte-exact §5.1 schema.
func TestImportHeaderMismatchRejected(t *testing.T) {
	bad := "txn_id,date,customer,segment,plan,mrr_usd,amount_usd,status,invoice_ref\n" +
		"TXN-20260901-00,2026-09-01,Acme,enterprise,pro,0.00,329.00,paid,INV-001,REV-00\n"
	s := NewStore()
	if err := s.ImportTable(TRevenue, []byte(bad)); err == nil {
		t.Fatal("short header accepted, want ErrHeaderMismatch/ErrBadRow")
	}
}

// Import must reject a data row with the wrong number of values.
func TestImportBadRowWidthRejected(t *testing.T) {
	sch, _ := Lookup(TCost)
	short := make(Row, len(sch.Headers)-1)
	bad := joinRec(sch.Headers) + "\n" + joinRec(short) + "\n"
	s := NewStore()
	if err := s.ImportTable(TCost, []byte(bad)); err == nil {
		t.Fatal("short row accepted, want ErrBadRow")
	}
}
