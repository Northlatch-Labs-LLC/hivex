package gridframe

// Blocking gate (§4): T2/T3 actions BLOCK pre-execution; human-reserve
// actions are rejected even with T3 approval — they need an authenticated
// HUMAN action event, never an agent action. Undecided T2 items older than
// 48h are flagged for MBR escalation with a cost-of-delay estimate.

import "time"

// Decision values for GateDecision.
const (
	Proceed = "proceed"
	Block   = "block"
)

// Gate wraps the ledger store: T2/T3 rows it raises are BLOCKING — nothing
// executes until Approve flips the row. raised keeps the action facts so a
// reserve action is re-checked at approval time too.
type Gate struct {
	s      *Store
	raised map[string]Action
}

func NewGate(s *Store) *Gate {
	return &Gate{s: s, raised: map[string]Action{}}
}

// Human-reserve list (§4, unconditional; NOT delegable via config). These
// categories are engine constants — a config value cannot remove one.
const (
	ReserveEntityPayments = "entity-registration-payments"
	ReserveBank           = "bank-account-opening-movement"
	ReserveTaxFiling      = "tax-filing"
	ReserveContractSign   = "contract-signature"
	ReservePriceList      = "publish-price-list"
	ReserveRoleElim       = "role-elimination-of-record"
	ReservePublicStmt     = "public-statement"
)

// IsReserve reports whether a category is on the human-reserve list.
func IsReserve(category string) bool {
	switch category {
	case ReserveEntityPayments, ReserveBank, ReserveTaxFiling,
		ReserveContractSign, ReservePriceList, ReserveRoleElim, ReservePublicStmt:
		return true
	}
	return false
}

// ErrHumanReserve: reserve actions require an authenticated human action
// event; no agent action and no approval tier can authorize them.
var ErrHumanReserve = errNew("gridframe: human-reserve action rejected; requires authenticated human action event")

// GateDecision is the gate's answer: proceed, or block on a raised
// approval-queue row (T2/T3).
type GateDecision struct {
	Decision string // Proceed | Block | Reject
	Reason   string
	ItemID   string // set when blocked: the pending queue row
}

// Evaluate classifies an agent action and enforces §4 pre-execution:
// T0/T1 proceed (T1 row raised first, dept-lead approver, execute+log);
// T2/T3 raise the queue row and BLOCK. A reserve-category action is
// REJECTED outright — even with a T3 approval already granted.
func (g *Gate) Evaluate(a Action, date string) GateDecision {
	if a.Reserve != "" {
		if !IsReserve(a.Reserve) {
			return GateDecision{Decision: Reject, Reason: "unknown reserve category " + a.Reserve}
		}
		return GateDecision{Decision: Reject, Reason: ErrHumanReserve.Error() + " (" + a.Reserve + ")"}
	}
	tier := Classify(a)
	if tier == Tier0 {
		return GateDecision{Decision: Proceed, Reason: "T0 log-only"}
	}
	row, err := Raise(g.s, a, tier, date)
	if err != nil {
		return GateDecision{Decision: Reject, Reason: "raise approval row: " + err.Error()}
	}
	g.raised[row[0]] = a
	if tier == Tier1 {
		// §4: T1 = execute + log. Execution is synchronous, so the dept
		// lead's decision lands on the row immediately (approved, dated).
		_ = g.Approve(row[0], ApprovalEvent{Signers: []string{LeadOf(a.Dept)}}, date)
		return GateDecision{Decision: Proceed, Reason: "T1 dept-lead approve + log", ItemID: row[0]}
	}
	return GateDecision{Decision: Block, Reason: tier + " blocks until human approval", ItemID: row[0]}
}

// refuses those outright. Human records the action as executed-by-human.
func (g *Gate) HumanExecute(a Action, date string) error {
	if a.Reserve == "" {
		return errNew("gridframe: HumanExecute is for human-reserve actions only")
	}
	if !IsReserve(a.Reserve) {
		return errNew("gridframe: unknown reserve category " + a.Reserve)
	}
	_ = date // decision records land with the Principal surface (G4)
	return nil
}

// EscalationDue: undecided T2 items older than 48h auto-escalate to the
// next MBR agenda with a "cost of delay" estimate (§4). Returns the pending
// T2 rows whose raised_date is more than 48h before now.
func (g *Gate) EscalationDue(now time.Time) []Row {
	sch, _ := Lookup(TApprov)
	td, _ := g.s.Table(TApprov)
	var out []Row
	for _, r := range td.Rows {
		if r.Get(sch, "tier") != Tier2 || r.Get(sch, "status") != "pending" {
			continue
		}
		raised, err := time.Parse("2006-01-02", r.Get(sch, "raised_date"))
		if err != nil {
			continue
		}
		if now.Sub(raised) > 48*time.Hour {
			out = append(out, r)
		}
	}
	return out
}

// CostOfDelay estimates the weekly cost of an undecided item: for spend
// rows, 10% of the one-time amount per week of delay (minimum $25); rows
// with no amount carry the $25 floor. The estimate rides the MBR agenda.
func CostOfDelay(a Action) float64 {
	c := 0.10*a.OneTimeUSD + 0.10*a.RecurringUSD*12
	if c < 25 {
		return 25
	}
	return c
}

// Reject: the action is refused outright — not blocked-for-approval.
const Reject = "reject"

// ApprovalEvent is the decision record for a pending queue row. Human=true
// marks an AUTHENTICATED HUMAN action event; an agent action never carries it.
type ApprovalEvent struct {
	Signers []string // approver IDs
	Human   bool
}

var (
	ErrNotPending   = errNew("gridframe: approval item is not pending")
	ErrWrongSigners = errNew("gridframe: required approver(s) missing")
	ErrNeedsHuman   = errNew("gridframe: approval event is not an authenticated human action")
)

// Approve flips a pending T2/T3 queue row to approved — the ONLY unblock
// path. T2 needs signer HOB-00; T3 needs HOB-00 AND RSK-01 (§4 dual
// confirmation). A reserve-category item additionally requires
// ApprovalEvent.Human; agent signers alone are refused even at T3.
func (g *Gate) Approve(itemID string, ev ApprovalEvent, date string) error {
	sch, _ := Lookup(TApprov)
	td, err := g.s.Table(TApprov)
	if err != nil {
		return err
	}
	for i, r := range td.Rows {
		if r.Get(sch, "item_id") != itemID {
			continue
		}
		if r.Get(sch, "status") != "pending" {
			return ErrNotPending
		}
		has := func(id string) bool {
			for _, s := range ev.Signers {
				if s == id {
					return true
				}
			}
			return false
		}
		switch r.Get(sch, "tier") {
		case Tier2:
			if !has("HOB-00") {
				return ErrWrongSigners
			}
		case Tier3:
			if !has("HOB-00") || !has("RSK-01") {
				return ErrWrongSigners
			}
		}
		if a, ok := g.raised[itemID]; ok && a.Reserve != "" && !ev.Human {
			return ErrNeedsHuman
		}
		td.Rows[i] = r.With(sch, "status", "approved").With(sch, "decision_date", date)
		return nil
	}
	return ErrUnknownOriginal
}
