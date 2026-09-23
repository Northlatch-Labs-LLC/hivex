package gridframe

// G5 acceptance tests 1-5 (spec §10), run against the real roster + seeds.

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestDispatcher(t *testing.T) *Dispatcher {
	t.Helper()
	d, err := NewDispatcher(testRoster, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func mustDispatch(t *testing.T, d *Dispatcher, role string, write []string) Dispatch {
	t.Helper()
	dp, err := d.Dispatch(role, "cadence job", write)
	if err != nil {
		t.Fatal(err)
	}
	return dp
}

// §10.1 Roster integrity: 25 agents + HOB-00; dept/lead/tier/autonomy resolve;
// RL lead reports to the Principal.
func TestAcc1RosterIntegrity(t *testing.T) {
	m, err := LoadRoster(testRoster)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 26 {
		t.Fatalf("roster = %d entries, want 26 (25 + HOB-00)", len(m))
	}
	if _, ok := m["HOB-00"]; !ok {
		t.Fatal("HOB-00 missing")
	}
	leads := map[string]string{"GOV": "EXE-00", "PE": "ENG-00", "RG": "REV-00",
		"OF": "OPS-00", "RL": "RSK-00", "PA": "PPL-00"}
	n := 0
	for id, a := range m {
		if id == "HOB-00" {
			continue
		}
		n++
		lead, ok := leads[a.Department]
		if !ok {
			t.Errorf("%s: dept %q not one of 6", id, a.Department)
			continue
		}
		if a.ModelTier != "A" && a.ModelTier != "B" && a.ModelTier != "C" {
			t.Errorf("%s: tier %q", id, a.ModelTier)
		}
		if AutonomyCeiling(a.Autonomy) < 0 {
			t.Errorf("%s: autonomy %q", id, a.Autonomy)
		}
		if a.ReportsTo == nil {
			t.Errorf("%s: no reports_to", id)
			continue
		}
		want := lead
		if id == lead { // dept leads report up: EXE-00 & RSK-00 to HOB-00, others to EXE-00
			want = "EXE-00"
			if id == "EXE-00" || id == "RSK-00" {
				want = "HOB-00" // coordinator + independent RL line
			}
		}
		if *a.ReportsTo != want {
			t.Errorf("%s: reports_to %s, want %s", id, *a.ReportsTo, want)
		}
	}
	if n != 25 {
		t.Fatalf("agents = %d, want 25", n)
	}
	if r := m["RSK-00"]; r.ReportsTo == nil || *r.ReportsTo != "HOB-00" {
		t.Error("RL lead must report to Principal")
	}
}

// §10.2 Ledger round-trip: seeded export byte-equals each seed CSV; re-import
// round-trips; append-only (edit rejected, reversal accepted).
func TestAcc2LedgerRoundTrip(t *testing.T) {
	s := seededStore(t)
	for _, tb := range Tables() {
		raw, err := os.ReadFile(filepath.Join(seedDir, tb.Name+".csv"))
		if err != nil {
			t.Fatal(err)
		}
		td, _ := s.Table(tb.Name)
		if got := td.Export(); !bytes.Equal(got, raw) {
			t.Errorf("%s: export != seed bytes (%d vs %d)", tb.Name, len(got), len(raw))
		}
		s2 := NewStore()
		if err := s2.ImportTable(tb.Name, td.Export()); err != nil {
			t.Fatalf("%s re-import: %v", tb.Name, err)
		}
		td2, _ := s2.Table(tb.Name)
		if !bytes.Equal(td2.Export(), raw) {
			t.Errorf("%s: re-import export != seed bytes", tb.Name)
		}
	}
	sch := mustLookup(TCost)
	if err := s.Append(TCost, costRow("CST-20260930-09", "2026-09-30", "10.00")); err != nil {
		t.Fatal(err)
	}
	if err := s.Append(TCost, costRow("CST-20260930-09", "2026-09-30", "11.00")); !errors.Is(err, ErrEditExisting) {
		t.Errorf("edit of existing row: err = %v, want ErrEditExisting", err)
	}
	rev := costRow("CST-20261001-01", "2026-10-01", "-10.00").With(sch, "description", "REVERSE CST-20260930-09")
	if err := s.AppendReversal(TCost, "CST-20260930-09", rev); err != nil {
		t.Errorf("reversing row rejected: %v", err)
	}
}

// §10.3 AEI correctness: 2026-09 derivation = 2.36 healthy; independent
// re-derivation of the published seed row agrees field-by-field.
func TestAcc3AEICorrectness(t *testing.T) {
	s := seededStore(t)
	res, err := DeriveAEI(s, "2026-09", 12000, "MBR-2026-09")
	if err != nil {
		t.Fatal(err)
	}
	if res.Revenue != 329.00 || res.CostD != 170.10 || res.CostL != 340.00 || res.CostA != 137.60 ||
		res.CostC != 647.70 || res.ValueV != 1529.00 || res.AEI != 2.36 || res.Status != "healthy" {
		t.Errorf("derivation = %+v", res)
	}
	sch, _ := Lookup(TAEI)
	td, _ := s.Table(TAEI)
	for _, r := range td.Rows {
		if r.Get(sch, "month") != "2026-09" {
			continue
		}
		ck, err := ReverifyAEI(s, "2026-09", 12000, r)
		if err != nil {
			t.Fatal(err)
		}
		if !ck.Agrees {
			t.Errorf("re-derivation disagrees: %v", ck.Diffs)
		}
		return
	}
	t.Fatal("no 2026-09 aei row in seeds")
}

// §10.4 Governance gate: T2 blocks pre-execution, human approval unblocks and
// logs; human-reserve action rejected even with T3 dual signers.
func TestAcc4GovernanceGate(t *testing.T) {
	s := seededStore(t)
	g := NewGate(s)
	a := Action{Description: "buy eval GPU bucket", Dept: "PE", RaisedBy: "ENG-00",
		OneTimeUSD: 300, External: true}
	d := g.Evaluate(a, "2026-09-23")
	if d.Decision != Block {
		t.Fatalf("T2 decision = %q, want block", d.Decision)
	}
	sch, _ := Lookup(TApprov)
	td, _ := s.Table(TApprov)
	var row Row
	for _, r := range td.Rows {
		if r.Get(sch, "item_id") == d.ItemID {
			row = r
		}
	}
	if row == nil || row.Get(sch, "status") != "pending" || row.Get(sch, "approver") != "HOB-00" {
		t.Fatalf("queue row = %v", row)
	}
	if err := g.Approve(d.ItemID, ApprovalEvent{Signers: []string{"ENG-00"}}, "2026-09-23"); !errors.Is(err, ErrWrongSigners) {
		t.Errorf("agent signer approved T2: err = %v, want ErrWrongSigners", err)
	}
	if err := g.Approve(d.ItemID, ApprovalEvent{Signers: []string{"HOB-00"}, Human: true}, "2026-09-23"); err != nil {
		t.Fatalf("human approval: %v", err)
	}
	td, _ = s.Table(TApprov)
	for _, r := range td.Rows {
		if r.Get(sch, "item_id") == d.ItemID &&
			(r.Get(sch, "status") != "approved" || r.Get(sch, "decision_date") != "2026-09-23") {
			t.Errorf("post-approval row = %v", r)
		}
	}
	res := Action{Description: "publish price list", Dept: "RG", RaisedBy: "REV-00",
		Reserve: ReservePriceList, External: true}
	dr := g.Evaluate(res, "2026-09-23")
	if dr.Decision != Reject || !strings.Contains(dr.Reason, ErrHumanReserve.Error()) {
		t.Errorf("reserve decision = %+v", dr)
	}
	rr, err := Raise(s, res, Tier3, "2026-09-23")
	if err != nil {
		t.Fatal(err)
	}
	g.raised[rr[0]] = res
	err = g.Approve(rr[0], ApprovalEvent{Signers: []string{"HOB-00", "RSK-01"}}, "2026-09-23")
	if !errors.Is(err, ErrNeedsHuman) {
		t.Errorf("T3 dual-sign on reserve: err = %v, want ErrNeedsHuman", err)
	}
}

// closeBody wires a §6 job-7 body: OPS-01 derives + appends the aei row and
// writes the close artifact; RSK-03 re-derives independently. The month closed
// is the month before the trigger's day-3 close date.
func closeBody(st *Store, d *Dispatcher, pipeline float64, t *testing.T) JobBody {
	return func(tc TriggerContext) error {
		month := tc.At.AddDate(0, 0, -3).Format("2006-01")
		sch, _ := Lookup(TAEI)
		res, err := DeriveAEI(st, month, pipeline, "MBR-"+month)
		if err != nil {
			return err
		}
		if err := AppendAeiRow(st, res); err != nil {
			return err
		}
		td, _ := st.Table(TAEI)
		var pub Row
		for _, r := range td.Rows {
			if r.Get(sch, "month") == month {
				pub = r
			}
		}
		ck, err := ReverifyAEI(st, month, pipeline, pub)
		if err != nil {
			return err
		}
		aei, _, _, err := LatestAEI(st)
		if err != nil {
			return err
		}
		br := ForcedResponse(aei)
		body := "RSK-03 agrees=" + boolStr(ck.Agrees) + " forced=" + br.Band
		for _, act := range br.Actions {
			body += "\n- " + act
		}
		_, err = d.WriteArtifact(mustDispatch(t, d, "OPS-01", []string{"close"}),
			strings.TrimPrefix(tc.Artifact, "records/"), "close-aei", body, tc.At)
		return err
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// §10.5 Cadence: 9 jobs exact cron+tz; manual trigger of jobs 1,2,7 writes
// artifacts; job 7 appends the AEI row + band action incl. forced AEI<1.0.
func TestAcc5Cadence(t *testing.T) {
	jobs := CadenceJobs()
	if len(jobs) != 9 {
		t.Fatalf("jobs = %d, want 9", len(jobs))
	}
	for i, j := range jobs {
		if j.Number != i+1 {
			t.Errorf("job order: #%d at %d", j.Number, i)
		}
		if _, err := j.NextRun(time.Now()); err != nil {
			t.Errorf("job %s cron %q: %v", j.Slug, j.Cron, err)
		}
	}
	if CadenceLoc().String() != "America/Los_Angeles" {
		t.Errorf("tz = %q", CadenceLoc().String())
	}
	fresh := func() *Store { // seeds minus the derived aei-monthly rows
		st := NewStore()
		for _, name := range TableNames() {
			if name == TAEI {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(seedDir, name+".csv"))
			if err != nil {
				t.Fatal(err)
			}
			if err := st.ImportTable(name, raw); err != nil {
				t.Fatal(err)
			}
		}
		return st
	}
	d := newTestDispatcher(t)
	reg := NewRegistry()
	writeBody := func(role, docType string) JobBody {
		return func(tc TriggerContext) error {
			_, err := d.WriteArtifact(mustDispatch(t, d, role, []string{"day", "digests"}),
				strings.TrimPrefix(tc.Artifact, "records/"), docType, docType+" body", tc.At)
			return err
		}
	}
	reg.SetBody("gridframe_day_plan", writeBody("EXE-01", "day-plan"))
	reg.SetBody("gridframe_principal_digest", writeBody("EXE-01", "digest"))
	at := time.Date(2026, 9, 23, 15, 0, 0, 0, time.UTC) // 08:00 PDT Wed
	for _, slug := range []string{"gridframe_day_plan", "gridframe_principal_digest"} {
		res, err := reg.Trigger(slug, at)
		if err != nil {
			t.Fatalf("%s: %v", slug, err)
		}
		if res.Runs[0] != "EXE-01" {
			t.Errorf("%s runs = %v", slug, res.Runs)
		}
	}
	for _, p := range []string{"day/2026-09-23.md", "digests/2026-09-23.md"} {
		if _, err := os.Stat(filepath.Join(d.base, p)); err != nil {
			t.Errorf("artifact %s: %v", p, err)
		}
	}

	st := fresh()
	reg7 := NewRegistry()
	reg7.SetBody("gridframe_close_aei", closeBody(st, d, 12000, t))
	res, err := reg7.Trigger("gridframe_close_aei", time.Date(2026, 10, 3, 16, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if res.Runs[0] != "OPS-01" || res.Runs[1] != "RSK-03" {
		t.Errorf("job7 runs = %v", res.Runs)
	}
	if res.Artifact != "records/close/2026-10-aei.md" {
		t.Errorf("job7 artifact = %q", res.Artifact)
	}
	if _, err := os.Stat(filepath.Join(d.base, "close/2026-10-aei.md")); err != nil {
		t.Errorf("job7 artifact file: %v", err)
	}
	td, _ := st.Table(TAEI)
	if len(td.Rows) != 1 || td.Rows[0][0] != "2026-09" || td.Rows[0][8] != "2.36" || td.Rows[0][9] != "healthy" {
		t.Errorf("appended aei row = %v", td.Rows)
	}
	if fr, err := SpendFreezeActive(st); err != nil || fr {
		t.Errorf("healthy close: SpendFreezeActive = %v (%v), want false", fr, err)
	}

	// Forced AEI<1.0: October has a 500.00 D cost, no revenue, pipeline 0.
	st2 := fresh()
	schC := mustLookup(TCost)
	if err := st2.Append(TCost, costRow("CST-20261002-01", "2026-10-02", "500.00").With(schC, "block", "D")); err != nil {
		t.Fatal(err)
	}
	reg8 := NewRegistry()
	reg8.SetBody("gridframe_close_aei", closeBody(st2, d, 0, t))
	if _, err := reg8.Trigger("gridframe_close_aei", time.Date(2026, 11, 3, 17, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	aei, status, _, err := LatestAEI(st2)
	if err != nil || aei != 0.00 || status != "survival" {
		t.Errorf("forced aei = %.2f %q (%v), want 0.00 survival", aei, status, err)
	}
	fr, err := SpendFreezeActive(st2)
	if err != nil || !fr {
		t.Errorf("SpendFreezeActive = %v (%v), want true", fr, err)
	}
	art, err := os.ReadFile(filepath.Join(d.base, "close/2026-11-aei.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(art), "freeze-t1-discretionary-spend") {
		t.Errorf("freeze action missing from artifact:\n%s", art)
	}
}
