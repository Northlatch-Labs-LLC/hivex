package gridframe

// G4 API handler tests: digest, decide, add-row, CSV export, compliance.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type testAPI struct {
	srv *httptest.Server
	s   *Store
	g   *Gate
}

func newTestAPI(t *testing.T) *testAPI {
	t.Helper()
	s := NewStore()
	g := NewGate(s)
	mux := http.NewServeMux()
	RegisterRoutes(mux, s, g, nil)
	return &testAPI{srv: httptest.NewServer(mux), s: s, g: g}
}

func call(t *testing.T, method, url string, body any) (*http.Response, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, _ := http.NewRequest(method, url, &buf)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if res.Header.Get("Content-Type") == "application/json" {
		_ = json.NewDecoder(res.Body).Decode(&out)
	}
	return res, out
}

func TestDigestListsPendingT2T3WithCostOfDelay(t *testing.T) {
	api := newTestAPI(t)
	defer api.srv.Close()

	// T2 spend row (blocks) + T1 (auto-approved, must NOT appear).
	d := api.g.Evaluate(Action{Dept: "PE", RaisedBy: "ENG-01", Description: "GPU", OneTimeUSD: 300}, "2026-09-01")
	if d.Decision != Block {
		t.Fatalf("T2 should block, got %s", d.Decision)
	}
	api.g.Evaluate(Action{Dept: "PE", RaisedBy: "ENG-01", Description: "log", RecurringUSD: 50}, "2026-09-01")

	res, out := call(t, "GET", api.srv.URL+"/gridframe/digest", nil)
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
	dec := out["decisions"].([]any)
	if len(dec) != 1 {
		t.Fatalf("want 1 pending T2 decision, got %d", len(dec))
	}
	item := dec[0].(map[string]any)
	if item["cost_of_delay_usd"] != 30.0 { // 10% of $300
		t.Fatalf("cost_of_delay = %v", item["cost_of_delay_usd"])
	}
}

func blocked(t *testing.T, api *testAPI) string {
	t.Helper()
	d := api.g.Evaluate(Action{Dept: "RL", RaisedBy: "RSK-00", Description: "renewal", OneTimeUSD: 120}, "2026-09-01")
	if d.Decision != Block {
		t.Fatalf("want block, got %s", d.Decision)
	}
	return d.ItemID
}

func TestDecideRejectEnforcesSigners(t *testing.T) {
	api := newTestAPI(t)
	defer api.srv.Close()
	item := blocked(t, api)
	base := api.srv.URL

	res, _ := call(t, "POST", base+"/gridframe/approvals/decide",
		map[string]any{"item_id": item, "decision": "reject", "signers": []string{"RSK-01"}})
	if res.StatusCode != 403 { // T2 needs HOB-00 (§4)
		t.Fatalf("want 403, got %d", res.StatusCode)
	}
	res, out := call(t, "POST", base+"/gridframe/approvals/decide",
		map[string]any{"item_id": item, "decision": "reject", "signers": []string{"HOB-00"}})
	if res.StatusCode != 200 || out["status"] != "rejected" {
		t.Fatalf("reject: %d %v", res.StatusCode, out)
	}
	res, _ = call(t, "POST", base+"/gridframe/approvals/decide",
		map[string]any{"item_id": item, "decision": "approve", "signers": []string{"HOB-00"}})
	if res.StatusCode != 409 { // already decided → conflict
		t.Fatalf("re-decide: want 409, got %d", res.StatusCode)
	}
}

func TestDecideApproveAndDefer(t *testing.T) {
	api := newTestAPI(t)
	defer api.srv.Close()
	base := api.srv.URL

	// T2 approve via gate unblock path.
	item := blocked(t, api)
	res, out := call(t, "POST", base+"/gridframe/approvals/decide",
		map[string]any{"item_id": item, "decision": "approve", "signers": []string{"HOB-00"}})
	if res.StatusCode != 200 || out["status"] != "approved" {
		t.Fatalf("approve: %d %v", res.StatusCode, out)
	}

	// T2 defer with the right signer.
	item2 := blocked(t, api)
	res, out = call(t, "POST", base+"/gridframe/approvals/decide",
		map[string]any{"item_id": item2, "decision": "defer", "signers": []string{"HOB-00"}})
	if res.StatusCode != 200 || out["status"] != "deferred" {
		t.Fatalf("defer: %d %v", res.StatusCode, out)
	}
	// Unknown item → 404.
	res, _ = call(t, "POST", base+"/gridframe/approvals/decide",
		map[string]any{"item_id": "APP-19990101-01", "decision": "approve", "signers": []string{"HOB-00"}})
	if res.StatusCode != 404 {
		t.Fatalf("unknown item: want 404, got %d", res.StatusCode)
	}
}

func TestAddRowAppendOnlyAndEngineOnly(t *testing.T) {
	api := newTestAPI(t)
	defer api.srv.Close()
	base := api.srv.URL

	body := map[string]any{"values": map[string]string{
		"txn_id": "TXN-20260901-01", "date": "2026-09-01", "customer": "Acme",
		"segment": "SMB", "plan": "pro", "mrr_usd": "50.00", "amount_usd": "50.00",
		"status": "paid", "invoice_ref": "INV-1", "owner": "REV-01",
	}}
	res, out := call(t, "POST", base+"/gridframe/register/revenue-ledger/rows", body)
	if res.StatusCode != 201 {
		t.Fatalf("add-row: %d %v", res.StatusCode, out)
	}
	// Same ID again → edit attempt → 409 (append-only).
	res, _ = call(t, "POST", base+"/gridframe/register/revenue-ledger/rows", body)
	if res.StatusCode != 409 {
		t.Fatalf("re-append: want 409, got %d", res.StatusCode)
	}
	// aei-monthly is engine-only → 403.
	res, _ = call(t, "POST", base+"/gridframe/register/aei-monthly/rows",
		map[string]any{"values": map[string]string{"month": "2026-09"}})
	if res.StatusCode != 403 {
		t.Fatalf("aei-monthly: want 403, got %d", res.StatusCode)
	}
	// Unknown table → 404; reversal via reverse_of.
	res, _ = call(t, "POST", base+"/gridframe/register/nope/rows", body)
	if res.StatusCode != 404 {
		t.Fatalf("unknown table: want 404, got %d", res.StatusCode)
	}
	rev := map[string]any{"reverse_of": "TXN-20260901-01", "values": map[string]string{
		"txn_id": "TXN-20260901-02", "date": "2026-09-02", "customer": "Acme",
		"segment": "SMB", "plan": "pro", "mrr_usd": "-50.00", "amount_usd": "-50.00",
		"status": "reversal", "invoice_ref": "INV-1", "owner": "REV-01",
	}}
	res, _ = call(t, "POST", base+"/gridframe/register/revenue-ledger/rows", rev)
	if res.StatusCode != 201 {
		t.Fatalf("reversal: want 201, got %d", res.StatusCode)
	}
}

func TestExportByteParity(t *testing.T) {
	api := newTestAPI(t)
	defer api.srv.Close()
	td, _ := api.s.Table(TCost)
	td.Rows = append(td.Rows, Row{"CST-20260901-01", "2026-09-01", "eng", "compute", "pool", "10.00", "Vendor,x", ""})
	want := td.Export()

	res, err := http.Get(api.srv.URL + "/gridframe/register/cost-ledger/export")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	got := make([]byte, len(want))
	if _, err := res.Body.Read(got); err != nil && len(got) != len(want) {
		t.Fatalf("read export: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("export mismatch:\n got %q\nwant %q", got, want)
	}
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestComplianceOverdueAndExceptions(t *testing.T) {
	api := newTestAPI(t)
	defer api.srv.Close()
	td, _ := api.s.Table(TCompl)
	td.Rows = append(td.Rows,
		Row{"CMP-1", "2026-01-01", "late obligation", "IRS", "Counsel", "planned", "annual", ""},
		Row{"CMP-2", "2027-01-01", "future obligation", "IRS", "Counsel", "planned", "annual", ""},
		Row{"CMP-3", "2026-01-01", "done obligation", "IRS", "Counsel", "done", "annual", ""})
	if _, err := api.s.Declare(Exception{Date: "2026-09-20", Severity: "SEV2", Dept: "RL", DeclaredBy: "RSK-01", Description: "late compliance"}, time.Time{}); err != nil {
		t.Fatal(err)
	}
	res, out := call(t, "GET", api.srv.URL+"/gridframe/compliance", nil)
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
	items := out["items"].([]any)
	overdue := 0
	for _, it := range items {
		m := it.(map[string]any)
		if m["overdue"] == true {
			overdue++
		}
	}
	if overdue != 1 { // CMP-1 only: CMP-2 future, CMP-3 done
		t.Fatalf("overdue = %d, want 1", overdue)
	}
}

func TestListRowsRoundTripAndExportHeaders(t *testing.T) {
	api := newTestAPI(t)
	defer api.srv.Close()
	base := api.srv.URL

	body := map[string]any{"values": map[string]string{
		"txn_id": "TXN-20260923-01", "date": "2026-09-23", "customer": "Acme",
		"segment": "SMB", "plan": "pro", "mrr_usd": "50.00", "amount_usd": "50.00",
		"status": "paid", "invoice_ref": "INV-9", "owner": "REV-01",
	}}
	if res, _ := call(t, "POST", base+"/gridframe/register/revenue-ledger/rows", body); res.StatusCode != 201 {
		t.Fatalf("add-row: %d", res.StatusCode)
	}
	res, out := call(t, "GET", base+"/gridframe/register/revenue-ledger", nil)
	if res.StatusCode != 200 {
		t.Fatalf("list: %d", res.StatusCode)
	}
	if len(out["rows"].([]any)) != 1 {
		t.Fatalf("rows = %v", out["rows"])
	}
	// §5.1 headers byte-exact.
	if got, _ := json.Marshal(out["headers"]); string(got) != `["txn_id","date","customer","segment","plan","mrr_usd","amount_usd","status","invoice_ref","owner"]` {
		t.Fatalf("headers = %s", got)
	}
	res, _ = call(t, "GET", base+"/gridframe/register/aei-monthly", nil)
	if res.StatusCode != 200 {
		t.Fatalf("aei list should be readable: %d", res.StatusCode)
	}
}
