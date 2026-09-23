package gridframe

import (
	"os"
	"path/filepath"
	"testing"
)

const seedRevenueCSV = "txn_id,date,customer,segment,plan,mrr_usd,amount_usd,status,invoice_ref,owner\n" +
	"TXN-20260901-00,2026-09-01,Acme,self-serve,monthly,49.00,49.00,paid,,REV-00\n"

func revRow(id string) Row {
	return Row{"TXN-20260923-0" + id, "2026-09-23", "Probe", "self-serve", "monthly", "49.00", "49.00", "paid", "", "REV-00"}
}

func TestLedgerPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	seed := t.TempDir()
	if err := os.WriteFile(filepath.Join(seed, "revenue-ledger.csv"), []byte(seedRevenueCSV), 0o644); err != nil {
		t.Fatal(err)
	}

	s1 := NewStore()
	if err := s1.PersistTo(dir); err != nil {
		t.Fatal(err)
	}
	if err := s1.SeedFrom(seed); err != nil {
		t.Fatal(err)
	}
	if err := s1.Append("revenue-ledger", revRow("1")); err != nil {
		t.Fatal(err)
	}
	s1.LockMonth("2026-08")

	// Restart equivalent: a fresh store armed on the same directory must
	// see the seed + the appended row and the lock.
	s2 := NewStore()
	if err := s2.PersistTo(dir); err != nil {
		t.Fatal(err)
	}
	td, err := s2.Table("revenue-ledger")
	if err != nil {
		t.Fatal(err)
	}
	if len(td.Rows) != 2 {
		t.Fatalf("want 2 rows after reload, got %d", len(td.Rows))
	}
	if !s2.IsLockedMonth("2026-08") {
		t.Fatal("month lock did not survive reload")
	}

	// Append-only still enforced across restarts.
	if err := s2.Append("revenue-ledger", revRow("1")); err == nil {
		t.Fatal("re-append of existing ID must fail after reload")
	}

	// The persisted file is byte-exact with Export.
	raw, err := os.ReadFile(filepath.Join(dir, "revenue-ledger.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(td.Export()) {
		t.Fatalf("persisted CSV diverges from Export:\n%q\n%q", raw, td.Export())
	}
}
