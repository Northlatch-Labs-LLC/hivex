package gridframe

// G5 acceptance tests 6-9 (spec §10). Test 10 (docs) is a shell check.

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// §10.6 Exception path: SEV2 declare -> open exception-log row; SEV1
// declaration -> surfaces on the Principal digest (notification mock).
func TestAcc6ExceptionPath(t *testing.T) {
	api := newTestAPI(t)
	defer api.srv.Close()
	sch, _ := Lookup(TExc)
	r2, err := api.s.Declare(Exception{Date: "2026-09-23", Severity: Sev2,
		Dept: "PE", DeclaredBy: "ENG-04", Description: "staging env down"}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if r2.Get(sch, "status") != "open" || r2.Get(sch, "exc_id") == "" {
		t.Errorf("SEV2 row = %v", r2)
	}
	r1, err := api.s.Declare(Exception{Date: "2026-09-23", Severity: Sev1,
		Dept: "PE", DeclaredBy: "ENG-04", Description: "prod data loss"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(r1.Get(sch, "exc_id")) == 0 {
		t.Fatal("SEV1 row missing id")
	}
	// Principal surface (mock): the digest's open_exceptions carries both rows
	// at once — notification latency 0, within the 30-minute bound.
	res, out := call(t, "GET", api.srv.URL+"/gridframe/digest", nil)
	if res.StatusCode != 200 {
		t.Fatalf("digest status %d", res.StatusCode)
	}
	seen := map[string]bool{}
	for _, e := range out["open_exceptions"].([]any) {
		m := e.(map[string]any)
		seen[m["exc_id"].(string)] = true
		if m["status"] != "open" {
			t.Errorf("digest exception status = %v", m["status"])
		}
	}
	if !seen[r1.Get(sch, "exc_id")] || !seen[r2.Get(sch, "exc_id")] {
		t.Errorf("digest open_exceptions = %v, want both %s and %s",
			seen, r1.Get(sch, "exc_id"), r2.Get(sch, "exc_id"))
	}
}

// §10.7 Board: add-row form persists; revenue CSV export matches store
// contents; approval decision writes the ledger row AND updates the digest.
func TestAcc7Board(t *testing.T) {
	api := newTestAPI(t)
	defer api.srv.Close()
	body := map[string]any{"values": map[string]string{
		"txn_id": "TXN-20260923-07", "date": "2026-09-23", "customer": "AccTest Co",
		"segment": "company", "plan": "scale-pilot", "mrr_usd": "150.00",
		"amount_usd": "150.00", "status": "paid", "invoice_ref": "INV-ACC1",
		"owner": "Vector"}}
	res, _ := call(t, "POST", api.srv.URL+"/gridframe/register/revenue-ledger/rows", body)
	if res.StatusCode != 201 {
		t.Fatalf("add-row status %d", res.StatusCode)
	}
	// Persistence: list rows round-trips the added row.
	res, out := call(t, "GET", api.srv.URL+"/gridframe/register/revenue-ledger", nil)
	if res.StatusCode != 200 {
		t.Fatalf("list status %d", res.StatusCode)
	}
	found := false
	for _, r := range out["rows"].([]any) {
		if r.(map[string]any)["txn_id"] == "TXN-20260923-07" {
			found = true
		}
	}
	if !found {
		t.Error("added row not persisted in listing")
	}
	// CSV parity: export bytes == store export bytes, contains the new row.
	req, _ := http.NewRequest("GET", api.srv.URL+"/gridframe/register/revenue-ledger/export", nil)
	cres, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var csv bytes.Buffer
	csv.ReadFrom(cres.Body)
	td, _ := api.s.Table(TRevenue)
	if !bytes.Equal(csv.Bytes(), td.Export()) {
		t.Error("export != store export bytes")
	}
	if !bytes.Contains(csv.Bytes(), []byte("TXN-20260923-07")) {
		t.Error("export missing added row")
	}
	// Decision writes ledger row + digest update: raise T2, approve via API,
	// row flips approved and leaves the digest's pending decisions.
	d := api.g.Evaluate(Action{Dept: "PE", RaisedBy: "ENG-01", Description: "GPU bucket",
		OneTimeUSD: 300}, "2026-09-23")
	if d.Decision != Block {
		t.Fatalf("T2 decision = %q", d.Decision)
	}
	res, _ = call(t, "POST", api.srv.URL+"/gridframe/approvals/decide", map[string]any{
		"item_id": d.ItemID, "decision": "approve", "signers": []string{"HOB-00"}, "human": true})
	if res.StatusCode != 200 {
		t.Fatalf("decide status %d", res.StatusCode)
	}
	asch, _ := Lookup(TApprov)
	atd, _ := api.s.Table(TApprov)
	flipped := false
	for _, r := range atd.Rows {
		if r.Get(asch, "item_id") == d.ItemID {
			flipped = r.Get(asch, "status") == "approved" && r.Get(asch, "decision_date") == todayPT()
		}
	}
	if !flipped {
		t.Error("approval-queue row not approved/dated")
	}
	_, out = call(t, "GET", api.srv.URL+"/gridframe/digest", nil)
	for _, dec := range out["decisions"].([]any) {
		m := dec.(map[string]any)["row"].(map[string]any)
		if m["item_id"] == d.ItemID {
			t.Error("decided item still pending in digest")
		}
	}
}

// §10.8 State: CMP-0005 done -> state formation.ein updates, milestone drops,
// compliance-calendar item flips status; document round-trips via Save/Load.
func TestAcc8StateMilestone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	b, err := os.ReadFile(filepath.Join(seedDir, "..", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Formation.EIN != "pending" {
		t.Fatalf("seed ein = %q, want pending", st.Formation.EIN)
	}
	if !st.MilestoneDone("CMP-0005", "12-3456789") {
		t.Fatal("CMP-0005 not in next_milestones")
	}
	if st.Formation.EIN != "12-3456789" {
		t.Errorf("ein = %q, want 12-3456789", st.Formation.EIN)
	}
	for _, m := range st.NextMilestones {
		if m.ID == "CMP-0005" {
			t.Error("CMP-0005 still listed")
		}
	}
	if err := st.Save(path); err != nil {
		t.Fatal(err)
	}
	st2, err := LoadState(path)
	if err != nil || st2.Formation.EIN != "12-3456789" {
		t.Errorf("reload ein = %q (%v)", st2.Formation.EIN, err)
	}
	// Companion compliance flip (caller-side, state.go:99-101): CMP-0005 done.
	api := &testAPI{s: seededStore(t), g: nil}
	api.g = NewGate(api.s)
	mux := http.NewServeMux()
	RegisterRoutes(mux, api.s, api.g, nil)
	api.srv = httptest.NewServer(mux)
	defer api.srv.Close()
	csch, _ := Lookup(TCompl)
	ctd, _ := api.s.Table(TCompl)
	flipped := false
	for i, r := range ctd.Rows {
		if r.Get(csch, "item_id") == "CMP-0005" {
			ctd.Rows[i] = r.With(csch, "status", "done")
			flipped = true
		}
	}
	if !flipped {
		t.Fatal("CMP-0005 not in compliance seeds")
	}
	_, out := call(t, "GET", api.srv.URL+"/gridframe/compliance", nil)
	for _, it := range out["items"].([]any) {
		m := it.(map[string]any)
		row := m["row"].(map[string]any)
		if row["item_id"] == "CMP-0005" {
			if row["status"] != "done" || m["overdue"] != false {
				t.Errorf("CMP-0005 status=%v overdue=%v", row["status"], m["overdue"])
			}
		}
	}
}

// §10.9 Audit: after test actions the sampler lists >=1 sampled action with a
// verdict; scorecard composite computes for a test agent.
func TestAcc9AuditAndScorecard(t *testing.T) {
	s := seededStore(t)
	g := NewGate(s)
	// Test actions: two executed T1s (auto-approved) + one approved T2.
	g.Evaluate(Action{Dept: "PE", RaisedBy: "ENG-01", Description: "dns record",
		RecurringUSD: 20, External: true}, "2026-09-23")
	g.Evaluate(Action{Dept: "RG", RaisedBy: "REV-01", Description: "draft post",
		PublicDraft: true}, "2026-09-23")
	d := g.Evaluate(Action{Dept: "OF", RaisedBy: "OPS-02", Description: "vendor tool",
		OneTimeUSD: 200, External: true}, "2026-09-23")
	if d.Decision != Block || g.Approve(d.ItemID, ApprovalEvent{Signers: []string{"HOB-00"}, Human: true}, "2026-09-23") != nil {
		t.Fatal("T2 raise+approve failed")
	}
	sp := NewSampler(s)
	ids := sp.Sample("2026-09")
	if len(ids) < 1 {
		t.Fatal("sample empty; want >=1")
	}
	pop := sp.ExecutedT1Plus("2026-09")
	if float64(len(ids)) < MinSampleRate*float64(len(pop)) {
		t.Errorf("sample %d of %d executed < 5%%", len(ids), len(pop))
	}
	v, err := sp.RecordVerdict(ids[0], VerdictViolation, "no playbook ref")
	if err != nil {
		t.Fatal(err)
	}
	if v.Verdict != VerdictViolation || sp.Violations()[0].ItemID != ids[0] {
		t.Errorf("verdict = %+v", v)
	}
	if _, err := sp.RecordVerdict("APP-20990101-99", VerdictCompliant, "x"); err == nil {
		t.Error("unsampled verdict accepted")
	}
	sc := Score(ScorecardInput{Cycle: "2026-Q4", AgentID: "ENG-01",
		KpiPct: 92, Quality: 4.5, CostEff: 1.8, Collab: 4.0, Incidents: 1})
	if sc.AgentID != "ENG-01" || sc.Composite <= 0 || sc.Grade == "" || sc.Action == "" {
		t.Errorf("scorecard = %+v", sc)
	}
	if err := AppendScorecardRow(s, ScorecardInput{Cycle: "2026-Q4", AgentID: "ENG-01",
		KpiPct: 92, Quality: 4.5, CostEff: 1.8, Collab: 4.0, Incidents: 1}, sc); err != nil {
		t.Fatal(err)
	}
	ssch, _ := Lookup(TScore)
	std, _ := s.Table(TScore)
	found := false
	for _, r := range std.Rows {
		if r.Get(ssch, "agent_id") == "ENG-01" && r.Get(ssch, "cycle") == "2026-Q4" {
			found = true
		}
	}
	if !found {
		t.Error("scorecard row not appended")
	}
}
