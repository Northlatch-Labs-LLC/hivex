// Package gridframe implements the Gridframe ledger/records/state core (spec §5.1, §7, §8).
package gridframe

import "strings"

// Table names and their EXACT §5.1 CSV headers (normative schema).
const (
	TRevenue = "revenue-ledger"
	TCost    = "cost-ledger"
	TAEI     = "aei-monthly"
	TApprov  = "approval-queue"
	TExc     = "exception-log"
	TScore   = "agent-scorecards"
	TSprint  = "sprint-log"
	TCompl   = "compliance-calendar"
)

// Table holds the normative schema for one ledger table.
type Table struct {
	Name    string   // table name == CSV file name (no extension)
	Headers []string // EXACT order from §5.1
	IDCol   string   // column holding the row ID ("" if none)
	Prefix  string   // ID prefix from §5.1 ("" if none)
	DateCol string   // column holding the row date ("" if month-keyed)
}

// tables is the registry of all 8 ledgers, §5.1.
var tables = []Table{
	{TRevenue, split("txn_id,date,customer,segment,plan,mrr_usd,amount_usd,status,invoice_ref,owner"), "txn_id", "TXN", "date"},
	{TCost, split("txn_id,date,block,category,description,amount_usd,vendor,approval_ref"), "txn_id", "CST", "date"},
	{TAEI, split("month,revenue,pipeline,value_v,cost_d,cost_l,cost_a,cost_c,aei,status,action_ref"), "", "", "month"},
	{TApprov, split("item_id,raised_date,dept,raised_by,description,tier,status,approver,decision_date"), "item_id", "APP", "raised_date"},
	{TExc, split("exc_id,date,severity,dept,declared_by,description,status,resolution,postmortem_ref"), "exc_id", "EXC", "date"},
	// SCR: §5.1 lists prefixes TXN/CST/APP/EXC/SCR/SPR/CMP; scorecards is the
	// remaining ledger, but §5.1 gives it no ID column (keyed cycle+agent_id).
	{TScore, split("cycle,agent_id,kpi_pct,quality,cost_eff,collab,incidents,composite,action"), "", "SCR", "cycle"},
	{TSprint, split("sprint_id,week,goal,items_committed,items_done,carryover,cost_usd,value_note"), "sprint_id", "SPR", "week"},
	{TCompl, split("item_id,due_date,obligation,authority,owner,status,recurrence,evidence_ref"), "item_id", "CMP", "due_date"},
}

func split(h string) []string { return strings.Split(h, ",") }

// Lookup returns the schema for a table name.
func Lookup(name string) (Table, bool) {
	for _, t := range tables {
		if t.Name == name {
			return t, true
		}
	}
	return Table{}, false
}

// Tables returns all 8 table schemas.
func Tables() []Table { return tables }

// TableNames returns the 8 table names in §5.1 order.
func TableNames() []string {
	names := make([]string, len(tables))
	for i, t := range tables {
		names[i] = t.Name
	}
	return names
}
