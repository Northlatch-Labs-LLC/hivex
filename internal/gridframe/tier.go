package gridframe

// Spec §4 approval tiers: Classify maps action facts onto the tier table;
// Raise creates the approval-queue row BEFORE execution (§4 pre-logging).

import (
	"strconv"
	"strings"
)

// Tier constants per §4.
const (
	Tier0 = "T0"
	Tier1 = "T1"
	Tier2 = "T2"
	Tier3 = "T3"
)

// Action is the fact set the §4 tier table keys on.
type Action struct {
	Description  string
	Dept         string // GOV|PE|RG|OF|RL|PA
	RaisedBy     string // agent ID
	OneTimeUSD   float64
	RecurringUSD float64 // new recurring commitment per month
	ContractTCV  float64
	DiscountPct  float64
	Legal        bool
	Financial    bool
	Bank         bool
	Tax          bool
	Irreversible bool
	External     bool   // any external effect
	PublicDraft  bool   // non-contractual public draft
	Reserve      string // human-reserve category (§4); "" if none — see gate.go
}

// Classify returns the §4 tier. Fail-closed: anything the table does not
// place in T0–T2 is T3 ("above T2").
func Classify(a Action) string {
	if a.Legal || a.Financial || a.Bank || a.Tax || a.Irreversible {
		return Tier3
	}
	if !a.External && a.OneTimeUSD == 0 && a.RecurringUSD == 0 && a.ContractTCV == 0 && !a.PublicDraft {
		return Tier0
	}
	if a.OneTimeUSD > 500 || a.RecurringUSD > 250 || a.ContractTCV >= 5000 || a.DiscountPct > 10 {
		return Tier3
	}
	if (a.RecurringUSD > 0 && a.RecurringUSD <= 100) || a.PublicDraft {
		return Tier1
	}
	if a.OneTimeUSD > 0 || a.RecurringUSD > 0 || a.ContractTCV > 0 || a.DiscountPct > 0 {
		return Tier2
	}
	return Tier3 // external effect the table does not name → above T2
}

// deptLeads maps §2 departments to their lead (the T1 approver).
var deptLeads = map[string]string{
	"GOV": "EXE-00", "PE": "ENG-00", "RG": "REV-00",
	"OF": "OPS-00", "RL": "RSK-00", "PA": "PPL-00",
}

// LeadOf returns the lead agent ID for a department ("" if unknown).
func LeadOf(dept string) string { return deptLeads[dept] }

// ApproverFor returns the §4 approver for a tier.
func ApproverFor(tier, dept string) string {
	switch tier {
	case Tier0:
		return "self"
	case Tier1:
		return LeadOf(dept)
	case Tier2:
		return "HOB-00"
	case Tier3:
		return "HOB-00+RSK-01"
	}
	return ""
}

// ErrNoQueueRow reports that T0 is log-only — §4 creates queue rows for T1+ only.
var ErrNoQueueRow = errNew("gridframe: tier T0 is log-only; no approval-queue row")

// Raise creates the approval-queue row for a T1+ action BEFORE execution
// (§4 pre-logging). The caller executes only after Raise succeeds.
// date is ISO YYYY-MM-DD; ID is APP-YYYYMMDD-NN per §5.1.
func Raise(s *Store, a Action, tier, date string) (Row, error) {
	if tier == Tier0 {
		return nil, ErrNoQueueRow
	}
	if tier != Tier1 && tier != Tier2 && tier != Tier3 {
		return nil, errWrap(ErrBadRow, "unknown tier %q", tier)
	}
	sch, _ := Lookup(TApprov)
	row := make(Row, len(sch.Headers))
	row = row.With(sch, "item_id", s.nextID(TApprov, "APP", date)).
		With(sch, "raised_date", date).
		With(sch, "dept", a.Dept).
		With(sch, "raised_by", a.RaisedBy).
		With(sch, "description", a.Description).
		With(sch, "tier", tier).
		With(sch, "status", "pending").
		With(sch, "approver", ApproverFor(tier, a.Dept))
	if err := s.Append(TApprov, row); err != nil {
		return nil, err
	}
	return row, nil
}

// nextID returns the next <PREFIX>-YYYYMMDD-NN ID for a date (§5.1):
// highest existing NN for that date plus one, so IDs are never reused.
// table is the ledger to scan; prefix is its §5.1 ID prefix.
func (s *Store) nextID(table string, prefix, date string) string {
	p := prefix + "-" + strings.ReplaceAll(date, "-", "") + "-"
	max := 0
	sch, ok := Lookup(table)
	if !ok {
		return p + pad2(1)
	}
	td, _ := s.Table(table)
	if td != nil {
		for _, r := range td.Rows {
			id := r.Get(sch, sch.IDCol)
			if len(id) > len(p) && id[:len(p)] == p {
				if n, err := strconv.Atoi(id[len(p):]); err == nil && n > max {
					max = n
				}
			}
		}
	}
	return p + pad2(max+1)
}

func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}
