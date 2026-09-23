package gridframe

import (
	"strconv"
	"testing"
)

// §5.2: seed import succeeds; counts and the 2026-09 sums match §10 test 3.
func TestSeedImportCounts(t *testing.T) {
	s := NewStore()
	if err := s.SeedStore(seedsDir); err != nil {
		t.Fatalf("SeedStore: %v", err)
	}
	wantRows := map[string]int{
		TRevenue: 2, TCost: 6, TAEI: 2, TApprov: 5,
		TExc: 2, TScore: 3, TSprint: 2, TCompl: 7,
	}
	for name, want := range wantRows {
		td, err := s.Table(name)
		if err != nil {
			t.Fatalf("table %s: %v", name, err)
		}
		if len(td.Rows) != want {
			t.Errorf("%s: rows = %d, want %d", name, len(td.Rows), want)
		}
	}
}

// cents parses a USD 2-decimal string into integer cents.
func cents(v string) int {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}
	return int(f*100 + 0.5)
}

// §10 test 3: 2026-09 revenue 329.00; cost D 170.10, L 340.00, A 137.60.
func TestSeedSums202609(t *testing.T) {
	s := NewStore()
	if err := s.SeedStore(seedsDir); err != nil {
		t.Fatalf("SeedStore: %v", err)
	}
	rev, _ := s.Table(TRevenue)
	sum := 0
	for _, r := range rev.Rows {
		if r[1][:7] == "2026-09" {
			sum += cents(r[6])
		}
	}
	if sum != 32900 {
		t.Errorf("revenue 2026-09 = %d cents, want 32900", sum)
	}
	cost, _ := s.Table(TCost)
	for _, tc := range []struct {
		block string
		want  int
	}{
		{"D", 17010}, {"L", 34000}, {"A", 13760},
	} {
		sum := 0
		for _, r := range cost.Rows {
			if r[1][:7] == "2026-09" && r[2] == tc.block {
				sum += cents(r[5])
			}
		}
		if sum != tc.want {
			t.Errorf("cost %s 2026-09 = %d cents, want %d", tc.block, sum, tc.want)
		}
	}
}
