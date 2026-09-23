package gridframe

import (
	"strings"
	"testing"
)

const seedDir = "../../hivex-home/.hivex/GRIDFRAME/reference/ledgers"

func seededStore(t *testing.T) *Store {
	t.Helper()
	s := NewStore()
	if err := s.SeedStore(seedDir); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDeriveAEISeed2026September(t *testing.T) {
	s := seededStore(t)
	res, err := DeriveAEI(s, "2026-09", 12000, "MBR-2026-09")
	if err != nil {
		t.Fatal(err)
	}
	if res.Revenue != 329.00 || res.CostD != 170.10 || res.CostL != 340.00 || res.CostA != 137.60 {
		t.Errorf("inputs: rev %.2f D %.2f L %.2f A %.2f", res.Revenue, res.CostD, res.CostL, res.CostA)
	}
	if res.CostC != 647.70 || res.ValueV != 1529.00 {
		t.Errorf("C %.2f V %.2f, want 647.70 / 1529.00", res.CostC, res.ValueV)
	}
	if res.AEI != 2.36 || res.Status != "healthy" {
		t.Errorf("AEI %.2f status %q, want 2.36 healthy", res.AEI, res.Status)
	}
	// 2026-08: no revenue, C=170 -> AEI 0, band survival per seed row.
	r8, err := DeriveAEI(s, "2026-08", 0, "MBR-2026-08")
	if err != nil {
		t.Fatal(err)
	}
	if r8.AEI != 0 || r8.Status != "survival" {
		t.Errorf("2026-08 AEI %.2f status %q, want 0 survival", r8.AEI, r8.Status)
	}
}

func TestAppendAeiRowOnceThenRejected(t *testing.T) {
	s := seededStore(t)
	res, _ := DeriveAEI(s, "2026-10", 5000, "MBR-2026-10")
	if err := AppendAeiRow(s, res); err != nil {
		t.Fatal(err)
	}
	if err := AppendAeiRow(s, res); err == nil {
		t.Fatal("re-append same month: want edit rejection")
	}
}

func TestReverifyAgreesAndDisagrees(t *testing.T) {
	s := seededStore(t)
	sch, _ := Lookup(TAEI)
	td, _ := s.Table(TAEI)
	var pub Row
	for _, r := range td.Rows {
		if r.Get(sch, "month") == "2026-09" {
			pub = r
		}
	}
	if pub == nil {
		t.Fatal("seed 2026-09 aei row missing")
	}
	ck, err := ReverifyAEI(s, "2026-09", 12000, pub)
	if err != nil {
		t.Fatal(err)
	}
	if !ck.Agrees || len(ck.Diffs) != 0 {
		t.Errorf("agreement: %+v diffs %v", ck.Rebuilt, ck.Diffs)
	}
	if !strings.Contains(strings.Join(pub, ","), "2.36,healthy") {
		t.Errorf("published row %v", pub)
	}
	bad := append(Row{}, pub...)
	bad[schIdx(sch, "aei")] = "9.99"
	ck2, err := ReverifyAEI(s, "2026-09", 12000, bad)
	if err != nil {
		t.Fatal(err)
	}
	if ck2.Agrees || len(ck2.Diffs) == 0 {
		t.Errorf("tampered row must disagree: %+v", ck2)
	}
}

func schIdx(sch Table, col string) int {
	for i, h := range sch.Headers {
		if h == col {
			return i
		}
	}
	return -1
}
