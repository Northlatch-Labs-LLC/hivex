package gridframe

import (
	"testing"
)

func TestForcedResponseBands(t *testing.T) {
	cases := []struct {
		aei    float64
		band   string
		freeze bool
		hrs    int
	}{
		{0.5, "survival", true, 48},
		{1.0, "warning", false, 0},
		{1.99, "warning", false, 0},
		{2.36, "healthy", false, 0},
		{4.0, "healthy", false, 0},
		{4.01, "investigate", false, 0},
	}
	for _, c := range cases {
		r := ForcedResponse(c.aei)
		if r.Band != c.band || r.FreezeT1Spend != c.freeze || r.DeadlineHours != c.hrs {
			t.Errorf("aei %.2f: %+v", c.aei, r)
		}
	}
	sur := ForcedResponse(0.8)
	if sur.Actions[0] != "freeze-t1-discretionary-spend" {
		t.Errorf("survival actions %v", sur.Actions)
	}
	inv := ForcedResponse(5.0)
	if inv.Owners[0] != "PPL-00" || inv.Owners[1] != "REV-00" {
		t.Errorf("investigate owners %v", inv.Owners)
	}
}

func TestSpendFreezeActiveFromLedger(t *testing.T) {
	s := seededStore(t) // latest aei row: 2026-09, 2.36 healthy
	fz, err := SpendFreezeActive(s)
	if err != nil {
		t.Fatal(err)
	}
	if fz {
		t.Error("seed latest band is healthy; freeze must be off")
	}
	low := AeiResult{Month: "2026-10", AEI: 0.70, Status: "survival"}
	if err := AppendAeiRow(s, low); err != nil {
		t.Fatal(err)
	}
	if fz, _ = SpendFreezeActive(s); !fz {
		t.Error("after AEI<1.0 row: spend-freeze action must fire")
	}
}
func TestScorecardGradesAndT3(t *testing.T) {
	cases := []struct {
		in   ScorecardInput
		want string
		t3   bool
	}{
		{ScorecardInput{"2026-Q4", "EXE-00", 95, 4.5, 2.5, 4.5, 0}, "S", false},
		{ScorecardInput{"2026-Q4", "OPS-01", 80, 4.0, 1.5, 4.0, 0}, "A", false},
		{ScorecardInput{"2026-Q4", "ENG-05", 65, 3.0, 1.0, 3.0, 0}, "B", false},
		{ScorecardInput{"2026-Q4", "REV-04", 55, 3.0, 1.0, 3.0, 0}, "C", false},
		{ScorecardInput{"2026-Q4", "OPS-03", 10, 1.0, 0.1, 1.0, 3}, "D", true},
	}
	for _, c := range cases {
		sc := Score(c.in)
		if sc.Grade != c.want || sc.RequiresT3 != c.t3 {
			t.Errorf("%s: grade %s composite %.2f t3=%v, want %s t3=%v",
				c.in.AgentID, sc.Grade, sc.Composite, sc.RequiresT3, c.want, c.t3)
		}
	}
	d := Score(cases[4].in)
	if d.Action != "retire/merge" || !d.RequiresT3 {
		t.Errorf("D mandate %+v", d)
	}
}

func TestAppendScorecardRow(t *testing.T) {
	s := seededStore(t)
	in := ScorecardInput{"2026-Q4", "ENG-05", 65, 3.0, 1.0, 3.0, 0}
	sc := Score(in)
	if err := AppendScorecardRow(s, in, sc); err != nil {
		t.Fatal(err)
	}
	td, _ := s.Table(TScore)
	last := td.Rows[len(td.Rows)-1]
	if last[0] != "2026-Q4" || last[1] != "ENG-05" || last[7] != "0.60" || last[8] != "coach 30-day re-check" {
		t.Errorf("appended row %v", last)
	}
}
