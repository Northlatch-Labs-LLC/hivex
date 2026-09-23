package gridframe

import "testing"

// §4 tier table cases, incl. boundaries.
func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		a    Action
		want string
	}{
		{"internal note: T0", Action{Description: "draft internal memo"}, Tier0},
		{"$100/mo new recurring: T1", Action{RecurringUSD: 100, External: true}, Tier1},
		{"public draft non-contractual: T1", Action{PublicDraft: true, External: true}, Tier1},
		{"$500 one-time: T2", Action{OneTimeUSD: 500, External: true}, Tier2},
		{"$250/mo: T2", Action{RecurringUSD: 250, External: true}, Tier2},
		{"contract $4999 TCV: T2", Action{ContractTCV: 4999, External: true}, Tier2},
		{"discount 10%: T2", Action{DiscountPct: 10, External: true}, Tier2},
		{"$501 one-time: T3", Action{OneTimeUSD: 501, External: true}, Tier3},
		{"$251/mo: T3", Action{RecurringUSD: 251, External: true}, Tier3},
		{"contract $5000 TCV: T3", Action{ContractTCV: 5000, External: true}, Tier3},
		{"discount 11%: T3", Action{DiscountPct: 11, External: true}, Tier3},
		{"legal: T3 even at $0", Action{Legal: true}, Tier3},
		{"bank movement: T3", Action{Bank: true, OneTimeUSD: 10}, Tier3},
		{"irreversible: T3", Action{Irreversible: true}, Tier3},
		{"fail-closed unlisted external: T3", Action{External: true}, Tier3},
	}
	for _, c := range cases {
		if got := Classify(c.a); got != c.want {
			t.Errorf("%s: Classify = %s, want %s", c.name, got, c.want)
		}
	}
}

func TestApproverFor(t *testing.T) {
	if got := ApproverFor(Tier1, "PE"); got != "ENG-00" {
		t.Errorf("T1 PE approver = %q, want ENG-00", got)
	}
	if got := ApproverFor(Tier2, "RG"); got != "HOB-00" {
		t.Errorf("T2 approver = %q, want HOB-00", got)
	}
	if got := ApproverFor(Tier3, "OF"); got != "HOB-00+RSK-01" {
		t.Errorf("T3 approver = %q, want HOB-00+RSK-01", got)
	}
	if got := ApproverFor(Tier1, "XX"); got != "" {
		t.Errorf("unknown dept approver = %q, want empty", got)
	}
}

// §4: every T1+ action creates its approval-queue row BEFORE execution;
// T0 is log-only and gets no row.
func TestRaisePreExecutionRow(t *testing.T) {
	s := NewStore()
	a := Action{Description: "buy CI add-on", Dept: "PE",
		RaisedBy: "ENG-01", OneTimeUSD: 480, External: true}
	tier := Classify(a)
	if tier != Tier2 {
		t.Fatalf("tier = %s, want T2", tier)
	}
	row, err := Raise(s, a, tier, "2026-09-23")
	if err != nil {
		t.Fatalf("Raise: %v", err)
	}
	sch, _ := Lookup(TApprov)
	if got := row.Get(sch, "item_id"); got != "APP-20260923-01" {
		t.Errorf("item_id = %q, want APP-20260923-01", got)
	}
	if got := row.Get(sch, "status"); got != "pending" {
		t.Errorf("status = %q, want pending", got)
	}
	if got := row.Get(sch, "approver"); got != "HOB-00" {
		t.Errorf("approver = %q, want HOB-00", got)
	}
	td, _ := s.Table(TApprov)
	if len(td.Rows) != 1 {
		t.Fatalf("approval-queue rows = %d, want 1 (row exists pre-execution)", len(td.Rows))
	}
	// Second raise same date: NN increments, IDs never reused.
	if _, err := Raise(s, a, tier, "2026-09-23"); err != nil {
		t.Fatalf("Raise 2: %v", err)
	}
	td, _ = s.Table(TApprov)
	if got := td.Rows[1].Get(sch, "item_id"); got != "APP-20260923-02" {
		t.Errorf("second item_id = %q, want APP-20260923-02", got)
	}
	// T0 gets no queue row.
	if _, err := Raise(s, Action{Description: "note"}, Tier0, "2026-09-23"); err != ErrNoQueueRow {
		t.Errorf("Raise T0 err = %v, want ErrNoQueueRow", err)
	}
	// Unknown tier is rejected fail-closed.
	if _, err := Raise(s, a, "T9", "2026-09-23"); err == nil {
		t.Error("Raise T9: want error")
	}
}
