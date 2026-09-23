package gridframe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Normative §5.1 header lines, copied verbatim from the spec
// (/Users/admin/Desktop/11-hivex-build-instruction.md L129-136).
var specHeaders = map[string]string{
	TRevenue: "txn_id,date,customer,segment,plan,mrr_usd,amount_usd,status,invoice_ref,owner",
	TCost:    "txn_id,date,block,category,description,amount_usd,vendor,approval_ref",
	TAEI:     "month,revenue,pipeline,value_v,cost_d,cost_l,cost_a,cost_c,aei,status,action_ref",
	TApprov:  "item_id,raised_date,dept,raised_by,description,tier,status,approver,decision_date",
	TExc:     "exc_id,date,severity,dept,declared_by,description,status,resolution,postmortem_ref",
	TScore:   "cycle,agent_id,kpi_pct,quality,cost_eff,collab,incidents,composite,action",
	TSprint:  "sprint_id,week,goal,items_committed,items_done,carryover,cost_usd,value_note",
	TCompl:   "item_id,due_date,obligation,authority,owner,status,recurrence,evidence_ref",
}

const seedsDir = "../../hivex-home/.hivex/GRIDFRAME/reference/ledgers"

func TestEightTables(t *testing.T) {
	if got := len(Tables()); got != 8 {
		t.Fatalf("Tables() = %d entries, want 8", got)
	}
	for _, name := range TableNames() {
		if _, ok := Lookup(name); !ok {
			t.Errorf("Lookup(%q) failed", name)
		}
		if _, ok := specHeaders[name]; !ok {
			t.Errorf("table %q not in spec §5.1 list", name)
		}
	}
}

// specPrefixes: §5.1 ID scheme, prefixes per table (aei-monthly none).
var specPrefixes = map[string]string{
	TRevenue: "TXN", TCost: "CST", TAEI: "", TApprov: "APP",
	TExc: "EXC", TScore: "SCR", TSprint: "SPR", TCompl: "CMP",
}

func TestHeadersByteExact(t *testing.T) {
	for _, tb := range Tables() {
		want := specHeaders[tb.Name]
		if got := strings.Join(tb.Headers, ","); got != want {
			t.Errorf("%s header:\n got %q\nwant %q", tb.Name, got, want)
		}
		if tb.IDCol != "" && !strings.Contains(want, tb.IDCol+",") && !strings.HasSuffix(want, tb.IDCol) {
			t.Errorf("%s IDCol %q not in header", tb.Name, tb.IDCol)
		}
		if tb.Prefix != specPrefixes[tb.Name] {
			t.Errorf("%s prefix = %q, want %q", tb.Name, tb.Prefix, specPrefixes[tb.Name])
		}
	}
}

// Headers must equal the first line of each shipped seed CSV (§5.2).
func TestHeadersMatchSeeds(t *testing.T) {
	for _, tb := range Tables() {
		b, err := os.ReadFile(filepath.Join(seedsDir, tb.Name+".csv"))
		if err != nil {
			t.Fatalf("read seed %s: %v", tb.Name, err)
		}
		seedHeader := strings.SplitN(strings.TrimSuffix(string(b), "\n"), "\n", 2)[0]
		seedHeader = strings.TrimSuffix(seedHeader, "\r")
		if got := strings.Join(tb.Headers, ","); got != seedHeader {
			t.Errorf("%s header vs seed:\n got %q\nseed %q", tb.Name, got, seedHeader)
		}
	}
}
