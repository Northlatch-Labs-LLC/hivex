package gridframe

import (
	"testing"
	"time"
)

// Acceptance #4: a simulated T2 agent action is BLOCKED pre-execution;
// after human approval it executes and logs.
func TestGateBlocksT2PreExecution(t *testing.T) {
	g := NewGate(NewStore())
	a := Action{Description: "buy CI add-on", Dept: "PE", RaisedBy: "ENG-01",
		OneTimeUSD: 480, External: true}
	d := g.Evaluate(a, "2026-09-23")
	if d.Decision != Block {
		t.Fatalf("decision = %s, want Block (T2 must block pre-execution)", d.Decision)
	}
	if d.ItemID == "" {
		t.Fatal("blocked decision carries no approval-queue item")
	}
	sch, _ := Lookup(TApprov)
	td, _ := g.s.Table(TApprov)
	if len(td.Rows) != 1 || td.Rows[0].Get(sch, "status") != "pending" {
		t.Fatal("queue row must exist and be pending before execution")
	}
	if err := g.Approve(d.ItemID, ApprovalEvent{Signers: []string{"HOB-00"}, Human: true}, "2026-09-24"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if td.Rows[0].Get(sch, "status") != "approved" {
		t.Error("row status after approval = pending, want approved")
	}
	if err := g.Approve(d.ItemID, ApprovalEvent{Signers: []string{"HOB-00"}, Human: true}, "2026-09-24"); err != ErrNotPending {
		t.Errorf("re-approve err = %v, want ErrNotPending", err)
	}
}

// T1 proceeds (dept lead) with its queue row raised; T0 log-only, no row.
func TestGateT1T0(t *testing.T) {
	g := NewGate(NewStore())
	d := g.Evaluate(Action{Dept: "PE", RaisedBy: "ENG-01", RecurringUSD: 50,
		External: true}, "2026-09-23")
	if d.Decision != Proceed {
		t.Fatalf("T1 decision = %s, want Proceed", d.Decision)
	}
	td, _ := g.s.Table(TApprov)
	if len(td.Rows) != 1 {
		t.Fatalf("T1 rows = %d, want 1 (T1 is execute + log)", len(td.Rows))
	}
	d = g.Evaluate(Action{Description: "note"}, "2026-09-23")
	if d.Decision != Proceed {
		t.Fatalf("T0 decision = %s, want Proceed", d.Decision)
	}
	td, _ = g.s.Table(TApprov)
	if len(td.Rows) != 1 {
		t.Errorf("T0 must not raise a row; rows = %d, want 1", len(td.Rows))
	}
}

// T3 requires DUAL confirmation: HOB-00 + RSK-01 (§4).
func TestGateT3DualConfirmation(t *testing.T) {
	g := NewGate(NewStore())
	d := g.Evaluate(Action{Dept: "RG", RaisedBy: "REV-00",
		OneTimeUSD: 6000, External: true}, "2026-09-23")
	if d.Decision != Block {
		t.Fatalf("decision = %s, want Block (T3)", d.Decision)
	}
	if err := g.Approve(d.ItemID, ApprovalEvent{Signers: []string{"HOB-00"}, Human: true}, "2026-09-23"); err != ErrWrongSigners {
		t.Errorf("T3 with only HOB-00 err = %v, want ErrWrongSigners", err)
	}
	if err := g.Approve(d.ItemID, ApprovalEvent{Signers: []string{"HOB-00", "RSK-01"}, Human: true}, "2026-09-23"); err != nil {
		t.Errorf("T3 dual confirmation: %v", err)
	}
}

// Acceptance #4: a human-reserve action (publish price list) is REJECTED
// even with approval — it needs an authenticated human action event.
func TestGateHumanReserveRejected(t *testing.T) {
	g := NewGate(NewStore())
	price := Action{Dept: "RG", RaisedBy: "REV-02", Reserve: ReservePriceList}
	d := g.Evaluate(price, "2026-09-23")
	if d.Decision != Reject {
		t.Fatalf("reserve decision = %s, want Reject (even with T3 approval)", d.Decision)
	}
	td, _ := g.s.Table(TApprov)
	if len(td.Rows) != 0 {
		t.Errorf("rejected reserve action raised %d queue rows, want 0", len(td.Rows))
	}
	// Even granting a T3 approval elsewhere does not open reserve:
	// only the authenticated human action event does.
	if err := g.HumanExecute(price, "2026-09-23"); err != nil {
		t.Errorf("HumanExecute on reserve: %v", err)
	}
	// Every §4 reserve category is engine-enforced.
	for _, c := range []string{ReserveEntityPayments, ReserveBank,
		ReserveTaxFiling, ReserveContractSign, ReservePriceList,
		ReserveRoleElim, ReservePublicStmt} {
		if !IsReserve(c) {
			t.Errorf("IsReserve(%q) = false, want true", c)
		}
	}
	if IsReserve("snack-order") {
		t.Error("IsReserve(snack-order) = true, want false")
	}
}

// §4: undecided T2 items older than 48h auto-escalate to the next MBR
// agenda with a cost-of-delay estimate.
func TestGateEscalation48h(t *testing.T) {
	g := NewGate(NewStore())
	old := Action{Dept: "OF", RaisedBy: "OPS-01", OneTimeUSD: 400, External: true}
	fresh := Action{Dept: "PE", RaisedBy: "ENG-02", OneTimeUSD: 300, External: true}
	if d := g.Evaluate(old, "2026-09-20"); d.Decision != Block {
		t.Fatal("old T2 not blocked")
	}
	if d := g.Evaluate(fresh, "2026-09-23"); d.Decision != Block {
		t.Fatal("fresh T2 not blocked")
	}
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	due := g.EscalationDue(now)
	if len(due) != 1 {
		t.Fatalf("escalation-due rows = %d, want 1 (only the >48h item)", len(due))
	}
	sch, _ := Lookup(TApprov)
	if due[0].Get(sch, "item_id") != "APP-20260920-01" {
		t.Errorf("escalated item = %q, want APP-20260920-01", due[0].Get(sch, "item_id"))
	}
	// Cost of delay rides the agenda for that item.
	if c := CostOfDelay(old); c != 40 { // 10% of $400
		t.Errorf("CostOfDelay($400 one-time) = %v, want 40", c)
	}
	if c := CostOfDelay(Action{}); c != 25 {
		t.Errorf("CostOfDelay floor = %v, want 25", c)
	}
	// Approving the old item clears it from escalation.
	if err := g.Approve("APP-20260920-01", ApprovalEvent{Signers: []string{"HOB-00"}, Human: true}, "2026-09-23"); err != nil {
		t.Fatalf("Approve old item: %v", err)
	}
	if n := len(g.EscalationDue(now)); n != 0 {
		t.Errorf("after approval, escalation-due = %d, want 0", n)
	}
}
