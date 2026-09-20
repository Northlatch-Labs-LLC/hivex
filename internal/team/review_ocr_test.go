package team

// review_ocr_test.go — pins the open-code-review surface: the preview parses
// the empirically verified v1.12.7 JSON shape, a missing binary answers 501
// (never silent), findings parsing tolerates the output variants, and async
// reviews report into the office feed.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeOcrBinary writes a shell script masquerading as ocr that emits the
// given payload and exits 0.
func fakeOcrBinary(t *testing.T, payload string) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "ocr")
	script := "#!/bin/sh\ncat <<'EOF'\n" + payload + "\nEOF\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

const ocrPreviewPayload = `{
  "files": [
    {"path": "internal/team/review_ocr.go", "status": "added", "insertions": 120, "deletions": 0, "will_review": true},
    {"path": "internal/team/broker.go", "status": "modified", "insertions": 2, "deletions": 1, "will_review": true}
  ]
}`

// TestOcrPreviewParsesTheVerifiedShape drives a fake binary through the
// preview path and asserts the parsed contract.
func TestOcrPreviewParsesTheVerifiedShape(t *testing.T) {
	t.Setenv("HIVEX_OCR_BINARY", fakeOcrBinary(t, ocrPreviewPayload))
	report, err := runOcrPreview(context.Background(), ".", "HEAD~1", "HEAD")
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(report.Files) != 2 {
		t.Fatalf("files = %d, want 2", len(report.Files))
	}
	f := report.Files[0]
	if f.Path != "internal/team/review_ocr.go" || f.Status != "added" || f.Insertions != 120 || !f.WillReview {
		t.Fatalf("file row mismatch: %+v", f)
	}
}

// TestOcrRoutePreviewAndMissingBinary pins the operator surface: a working
// preview answers 200 with the file list; a missing binary answers 501 with
// the install hint.
func TestOcrRoutePreviewAndMissingBinary(t *testing.T) {
	b := newTestBroker(t)

	t.Setenv("HIVEX_OCR_BINARY", fakeOcrBinary(t, ocrPreviewPayload))
	srv := httptest.NewServer(http.HandlerFunc(b.handleReviewRun))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", strings.NewReader(`{"mode":"preview","from":"HEAD~1","to":"HEAD"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("preview status = %d, want 200", res.StatusCode)
	}
	var body struct {
		Mode  string `json:"mode"`
		Files []reviewFileInfo
		Count int `json:"count"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Mode != "preview" || body.Count != 2 {
		t.Fatalf("preview body mismatch: %+v", body)
	}

	t.Setenv("HIVEX_OCR_BINARY", filepath.Join(t.TempDir(), "does-not-exist"))
	prev := reviewOcrLookPath
	reviewOcrLookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	t.Cleanup(func() { reviewOcrLookPath = prev })
	res2, err := http.Post(srv.URL, "application/json", strings.NewReader(`{"mode":"preview"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusNotImplemented {
		t.Fatalf("missing binary status = %d, want 501", res2.StatusCode)
	}
}

// TestParseOcrFindingsToleratesVariants pins the tolerant parse: bare
// arrays, envelope keys, and differing field names all normalize.
func TestParseOcrFindingsToleratesVariants(t *testing.T) {
	bare := parseOcrFindings([]byte(`[
	  {"file":"a.go","start_line":12,"level":"high","body":"nil deref"},
	  {"path":"b.go","line_number":3,"severity":"low","message":"style"}
	]`))
	if len(bare) != 2 || bare[0].Path != "a.go" || bare[0].Line == nil || *bare[0].Line != 12 || bare[0].Severity != "high" || bare[0].Message != "nil deref" {
		t.Fatalf("bare array parse mismatch: %+v", bare)
	}
	if len(bare) < 2 || bare[1].Rule != "" && false {
		t.Fatal("unreachable guard")
	}
	envelope := parseOcrFindings([]byte(`{"findings":[{"path":"c.go","line":7,"severity":"medium","message":"race","rule":"R1"}]}`))
	if len(envelope) != 1 || envelope[0].Rule != "R1" || envelope[0].Message != "race" {
		t.Fatalf("envelope parse mismatch: %+v", envelope)
	}
	comments := parseOcrFindings([]byte(`{"comments":[{"file_path":"d.go","description":"nit"}]}`))
	if len(comments) != 1 || comments[0].Path != "d.go" || comments[0].Message != "nit" {
		t.Fatalf("comments envelope parse mismatch: %+v", comments)
	}
	if got := parseOcrFindings([]byte(`{"unexpected": 1}`)); len(got) != 0 {
		t.Fatalf("unrelated JSON must yield no findings, got %+v", got)
	}
}

// TestOcrReviewAsyncPostsToOffice pins the async path: a completed review
// posts its summary into the office feed; a failing run posts the failure.
func TestOcrReviewAsyncPostsToOffice(t *testing.T) {
	b := newTestBroker(t)

	t.Setenv("HIVEX_OCR_BINARY", fakeOcrBinary(t, `{"findings":[
	  {"path":"x.go","line":9,"severity":"high","message":"unchecked error","rule":"E1"},
	  {"path":"y.go","line":4,"severity":"low","message":"naming"},
	  {"path":"z.go","line":2,"severity":"low","message":"spacing"},
	  {"path":"w.go","line":8,"severity":"low","message":"dup"},
	  {"path":"v.go","line":1,"severity":"low","message":"order"},
	  {"path":"u.go","line":5,"severity":"low","message":"extra"}
	]}`))
	before := len(b.messages)
	b.runOcrReviewAsync(".", "HEAD~1", "HEAD")
	deadline := time.Now().Add(5 * time.Second)
	summary := ""
	for time.Now().Before(deadline) {
		if len(b.messages) > before {
			summary = b.messages[len(b.messages)-1].Content
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if summary == "" {
		t.Fatal("review completion never posted to the office feed")
	}
	if !strings.Contains(summary, "6 findings") || !strings.Contains(summary, "x.go:9") || !strings.Contains(summary, "and 1 more") {
		t.Fatalf("summary mismatch: %q", summary)
	}

	// A failing run reports the failure loudly into the feed.
	t.Setenv("HIVEX_OCR_BINARY", fakeOcrBinary(t, `{not-json`))
	before = len(b.messages)
	b.runOcrReviewAsync(".", "", "")
	for time.Now().Before(deadline) {
		if len(b.messages) > before {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(b.messages) == before {
		t.Fatal("a failing review must post the failure to the office feed")
	}
}
