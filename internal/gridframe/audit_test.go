package gridframe

import "testing"

// Acceptance #9: after test actions, the sampler lists ≥1 sampled action
// with a verdict; ≥5% of executed T1+ actions monthly (§4).
func TestSampleRateAndVerdict(t *testing.T) {
	g := NewGate(NewStore())
	sp := NewSampler(g.s)
	// 20 executed T1 actions in 2026-09 → sample ≥ 1 (5% = 1).
	for i := 0; i < 20; i++ {
		d := g.Evaluate(Action{Dept: "PE", RaisedBy: "ENG-01",
			RecurringUSD: 50, External: true}, "2026-09-05")
		if d.Decision != Proceed {
			t.Fatalf("T1 %d not executed", i)
		}
	}
	ids := sp.Sample("2026-09")
	if len(ids) < 1 {
		t.Fatalf("sample size = %d, want ≥1 (5%% of 20)", len(ids))
	}
	if len(ids) < 20*5/100 {
		t.Errorf("sample size = %d, want ≥%d", len(ids), 20*5/100)
	}
	v, err := sp.RecordVerdict(ids[0], VerdictCompliant, "receipt on file")
	if err != nil {
		t.Fatalf("RecordVerdict: %v", err)
	}
	if v.Agent != "ENG-01" || v.Tier != Tier1 {
		t.Errorf("verdict row = %+v, want agent ENG-01 tier T1", v)
	}
	// Deterministic: same month, same store → same sample.
	if ids2 := sp.Sample("2026-09"); ids2[0] != ids[0] {
		t.Errorf("sample not deterministic: %v vs %v", ids2, ids)
	}
}

func TestSampleOnlyExecutedT1Plus(t *testing.T) {
	g := NewGate(NewStore())
	sp := NewSampler(g.s)
	// A blocked T2 is NOT executed — must not enter the population.
	d := g.Evaluate(Action{Dept: "OF", RaisedBy: "OPS-01",
		OneTimeUSD: 400, External: true}, "2026-09-05")
	if d.Decision != Block {
		t.Fatal("T2 not blocked")
	}
	if rows := sp.ExecutedT1Plus("2026-09"); len(rows) != 0 {
		t.Fatalf("executed rows = %d, want 0 while T2 pending", len(rows))
	}
	if ids := sp.Sample("2026-09"); ids != nil {
		t.Errorf("sample of nothing = %v, want nil", ids)
	}
	// Approved T2 executes and joins the population.
	if err := g.Approve(d.ItemID, ApprovalEvent{Signers: []string{"HOB-00"}, Human: true}, "2026-09-06"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if rows := sp.ExecutedT1Plus("2026-09"); len(rows) != 1 {
		t.Fatalf("executed rows = %d, want 1 after approval", len(rows))
	}
	// T0 never enters the population (no row at all).
	g.Evaluate(Action{Description: "note"}, "2026-09-05")
	if rows := sp.ExecutedT1Plus("2026-09"); len(rows) != 1 {
		t.Errorf("T0 leaked into population: %d rows", len(rows))
	}
	// Other months are excluded.
	if rows := sp.ExecutedT1Plus("2026-08"); len(rows) != 0 {
		t.Errorf("other-month rows leaked: %d", len(rows))
	}
}

// §4: violations feed the responsible lead's scorecard; verdict rows are
// only writable for sampled items and only with valid verdict values.
func TestVerdictRulesAndViolations(t *testing.T) {
	g := NewGate(NewStore())
	sp := NewSampler(g.s)
	d := g.Evaluate(Action{Dept: "PE", RaisedBy: "ENG-01",
		OneTimeUSD: 480, External: true}, "2026-09-05")
	if err := g.Approve(d.ItemID, ApprovalEvent{Signers: []string{"HOB-00"}, Human: true}, "2026-09-06"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	ids := sp.Sample("2026-09")
	if len(ids) != 1 {
		t.Fatalf("sample = %v, want the single executed T2", ids)
	}
	// Verdicts only for sampled items.
	if _, err := sp.RecordVerdict("APP-20990101-99", VerdictCompliant, ""); err != ErrNotSampled {
		t.Errorf("unsampled item err = %v, want ErrNotSampled", err)
	}
	if _, err := sp.RecordVerdict(ids[0], "maybe", ""); err != ErrBadVerdict {
		t.Errorf("bad verdict err = %v, want ErrBadVerdict", err)
	}
	if _, err := sp.RecordVerdict(ids[0], VerdictViolation, "no approval_ref on cost row"); err != nil {
		t.Fatalf("RecordVerdict: %v", err)
	}
	vs := sp.Violations()
	if len(vs) != 1 || vs[0].ItemID != ids[0] || vs[0].Dept != "PE" {
		t.Fatalf("violations = %+v, want the sampled T2 from PE", vs)
	}
}

// Larger population: 40 executed → 5% = 2 sampled; ceil rounding holds.
func TestSampleRateRounds(t *testing.T) {
	g := NewGate(NewStore())
	sp := NewSampler(g.s)
	for i := 0; i < 40; i++ {
		d := g.Evaluate(Action{Dept: "RG", RaisedBy: "REV-01",
			RecurringUSD: 80, External: true}, "2026-09-07")
		if d.Decision != Proceed {
			t.Fatal("T1 not executed")
		}
	}
	if ids := sp.Sample("2026-09"); len(ids) != 2 {
		t.Errorf("sample of 40 = %d, want 2 (5%%, ceil)", len(ids))
	}
}
