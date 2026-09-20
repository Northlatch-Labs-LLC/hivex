package team

// review_ocr.go — the open-code-review surface (alibaba/open-code-review,
// Apache-2.0, `ocr` CLI): deterministic file-selection previews run
// synchronously; full LLM-backed reviews run asynchronously and report into
// the office feed. The binary is optional — absent means the route answers
// 501 with the install hint, never a silent failure.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Northlatch-Labs-LLC/hivex/internal/config"
)

// Binary seams for tests (same indirection pattern as headless codex).
var (
	reviewOcrLookPath       = exec.LookPath
	reviewOcrCommandContext = exec.CommandContext
)

// reviewOcrPreviewTimeout bounds the deterministic preview; the LLM-backed
// full review gets the async budget below.
const (
	reviewOcrPreviewTimeout = 60 * time.Second
	reviewOcrReviewTimeout  = 15 * time.Minute
)

// reviewOcrBinary resolves the ocr binary: HIVEX_OCR_BINARY wins over PATH
// (and must itself exist — a stale override is "not installed", not a 500).
func reviewOcrBinary() (string, error) {
	if v := strings.TrimSpace(config.Getenv("HIVEX_OCR_BINARY")); v != "" {
		if _, err := os.Stat(v); err == nil {
			return v, nil
		}
	}
	return reviewOcrLookPath("ocr")
}

// reviewFileInfo is one file the reviewer will look at — the empirically
// verified `ocr review --preview --format json` shape (v1.12.7).
type reviewFileInfo struct {
	Path       string `json:"path"`
	Status     string `json:"status"`
	Insertions int    `json:"insertions"`
	Deletions  int    `json:"deletions"`
	WillReview bool   `json:"will_review"`
}

type reviewPreviewReport struct {
	Files []reviewFileInfo `json:"files"`
}

// ocrRangeArgs appends the review range flags when provided.
func ocrRangeArgs(args []string, from, to string) []string {
	if strings.TrimSpace(from) != "" {
		args = append(args, "--from", strings.TrimSpace(from))
	}
	if strings.TrimSpace(to) != "" {
		args = append(args, "--to", strings.TrimSpace(to))
	}
	return args
}

// runOcrPreview runs the deterministic file-selection preview and parses
// the verified JSON shape. Fails loud on a non-2xx-exit run.
func runOcrPreview(ctx context.Context, dir, from, to string) (reviewPreviewReport, error) {
	bin, err := reviewOcrBinary()
	if err != nil {
		return reviewPreviewReport{}, fmt.Errorf("ocr binary not found: %w (install with: npm i -g @alibaba-group/open-code-review)", err)
	}
	args := ocrRangeArgs([]string{"review", "--preview", "--format", "json"}, from, to)
	ctx, cancel := context.WithTimeout(ctx, reviewOcrPreviewTimeout)
	defer cancel()
	cmd := reviewOcrCommandContext(ctx, bin, args...)
	cmd.Dir = dir
	raw, err := cmd.CombinedOutput()
	if err != nil {
		return reviewPreviewReport{}, fmt.Errorf("ocr preview: %w: %s", err, truncate(string(raw), 300))
	}
	var report reviewPreviewReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return reviewPreviewReport{}, fmt.Errorf("ocr preview: unparseable output: %w", err)
	}
	return report, nil
}

// reviewFinding is one LLM review finding. Field mapping is tolerant across
// the reviewer's output variants (path|file, line_number|start_line,
// severity|level, message|description|body, rule|rule_id|name).
type reviewFinding struct {
	Path     string `json:"path"`
	Line     *int   `json:"line,omitempty"`
	Severity string `json:"severity,omitempty"`
	Message  string `json:"message,omitempty"`
	Rule     string `json:"rule,omitempty"`
}

func reviewString(row map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := row[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// parseOcrFindings extracts findings from the full-review JSON, tolerating
// a bare array or {findings|issues|comments|insights: [...]} envelopes.
func parseOcrFindings(raw []byte) []reviewFinding {
	var rows []map[string]any
	trimmed := strings.TrimSpace(string(raw))
	for _, key := range []string{"findings", "issues", "comments", "insights", "reviews", "results"} {
		prefix := `{"` + key + `"`
		if strings.HasPrefix(trimmed, prefix) {
			var wrapped map[string][]map[string]any
			if err := json.Unmarshal(raw, &wrapped); err == nil && len(wrapped[key]) > 0 {
				rows = wrapped[key]
				break
			}
		}
	}
	if rows == nil {
		if err := json.Unmarshal(raw, &rows); err != nil {
			return nil
		}
	}
	var out []reviewFinding
	for _, row := range rows {
		f := reviewFinding{
			Path:     reviewString(row, "path", "file", "file_path"),
			Severity: reviewString(row, "severity", "level", "priority", "kind"),
			Message:  reviewString(row, "message", "description", "body", "comment", "summary", "content"),
			Rule:     reviewString(row, "rule", "rule_id", "id", "name", "rule_name"),
		}
		for _, key := range []string{"line", "line_number", "start_line"} {
			if n, ok := row[key].(float64); ok {
				ln := int(n)
				f.Line = &ln
				break
			}
		}
		if f.Path == "" && f.Message == "" {
			continue
		}
		out = append(out, f)
	}
	return out
}

// runOcrReviewAsync launches the LLM-backed full review on a goroutine; the
// findings (or the failure) land in the office feed via the automation
// sender. The caller never waits on the model.
func (b *Broker) runOcrReviewAsync(dir, from, to string) {
	bin, err := reviewOcrBinary()
	if err != nil {
		b.postReviewOfficeMessage(fmt.Sprintf("review: ocr binary unavailable (%v) — install with: npm i -g @alibaba-group/open-code-review", err))
		return
	}
	go func() {
		args := ocrRangeArgs([]string{"review", "--format", "json", "--audience", "agent"}, from, to)
		ctx, cancel := context.WithTimeout(context.Background(), reviewOcrReviewTimeout)
		defer cancel()
		cmd := reviewOcrCommandContext(ctx, bin, args...)
		cmd.Dir = dir
		raw, err := cmd.CombinedOutput()
		if err != nil {
			b.postReviewOfficeMessage(fmt.Sprintf("review: run failed: %v: %s", err, truncate(string(raw), 300)))
			return
		}
		findings := parseOcrFindings(raw)
		summary := fmt.Sprintf("review complete: %d findings", len(findings))
		for i, f := range findings {
			if i >= 5 {
				summary += fmt.Sprintf("\n- …and %d more", len(findings)-5)
				break
			}
			where := f.Path
			if f.Line != nil {
				where = fmt.Sprintf("%s:%d", f.Path, *f.Line)
			}
			summary += fmt.Sprintf("\n- %s [%s] %s", where, f.Severity, truncate(f.Message, 160))
		}
		b.postReviewOfficeMessage(summary)
	}()
}

// postReviewOfficeMessage reports review outcomes into the office feed.
func (b *Broker) postReviewOfficeMessage(text string) {
	if b == nil {
		return
	}
	_, _, _ = b.PostAutomationMessage("hive", "", "Code review", text, "", "", "", nil, "")
}

// handleReviewRun is the operator surface: POST /review/run with
// {mode: "preview"|"review", from, to, dir}. Preview is deterministic and
// synchronous; the full review is accepted asynchronously and reports to
// the office. Broker-token gated — a bot never triggers spend-grade runs.
func (b *Broker) handleReviewRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Mode string `json:"mode"`
		From string `json:"from"`
		To   string `json:"to"`
		Dir  string `json:"dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	dir := strings.TrimSpace(body.Dir)
	if dir == "" {
		dir = "."
	}
	switch strings.ToLower(strings.TrimSpace(body.Mode)) {
	case "", "preview":
		report, err := runOcrPreview(r.Context(), dir, body.From, body.To)
		if err != nil {
			status := http.StatusInternalServerError
			if strings.Contains(err.Error(), "ocr binary not found") {
				status = http.StatusNotImplemented
			}
			http.Error(w, err.Error(), status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"mode": "preview", "files": report.Files, "count": len(report.Files)})
	case "review":
		if _, err := reviewOcrBinary(); err != nil {
			http.Error(w, fmt.Sprintf("ocr binary not found: %v (install with: npm i -g @alibaba-group/open-code-review)", err), http.StatusNotImplemented)
			return
		}
		b.runOcrReviewAsync(dir, body.From, body.To)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"mode": "review", "accepted": true, "note": "findings will land in the office feed when the review completes"})
	default:
		http.Error(w, "mode must be preview or review", http.StatusBadRequest)
	}
}
