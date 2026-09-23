package gridframe

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// CSV over the append-only store. Round-trip is byte-compatible with the
// §5.1 seeds, preserving each file's record separator and trailing newline
// (seeds are mixed CRLF/LF). Quoting mirrors Go encoding/csv writer rules,
// which reproduce the seed files' quoting exactly.

// quoteField mirrors encoding/csv fieldNeedsQuotes + escaping.
func quoteField(f string) string {
	if f == "" {
		return f
	}
	if f == `\.` || strings.ContainsAny(f, ",\"\r\n") {
		return `"` + strings.ReplaceAll(f, `"`, `""`) + `"`
	}
	r1, _ := utf8.DecodeRuneInString(f)
	if unicode.IsSpace(r1) {
		return `"` + strings.ReplaceAll(f, `"`, `""`) + `"`
	}
	return f
}

// Export renders header + rows as CSV bytes, joined with the separator
// captured at import (default "\n") and the captured trailing newline.
func (td *TableData) Export() []byte {
	sep := td.Sep
	if sep == "" {
		sep = "\n"
	}
	end := td.FinalNewline
	if end == "" {
		end = sep
	}
	recs := make([]string, 0, 1+len(td.Rows))
	recs = append(recs, strings.Join(td.Headers, ","))
	for _, r := range td.Rows {
		f := make([]string, len(r))
		for i, v := range r {
			f[i] = quoteField(v)
		}
		recs = append(recs, strings.Join(f, ","))
	}
	return []byte(strings.Join(recs, sep) + end)
}

var ErrHeaderMismatch = errNew("CSV header does not match the §5.1 schema")

// ParseTable parses CSV bytes into a TableData. The header must be
// byte-identical to the §5.1 schema for the table. The record separator and
// trailing newline are captured from the raw bytes for byte-export parity.
func ParseTable(name string, raw []byte) (*TableData, error) {
	sch, ok := Lookup(name)
	if !ok {
		return nil, errWrap(ErrUnknownTable, "%s", name)
	}
	recs, err := readRecords(raw)
	if err != nil || len(recs) == 0 {
		return nil, errBad("%s: empty or unparsable CSV: %v", name, err)
	}
	if got := joinRec(recs[0]); got != strings.Join(sch.Headers, ",") {
		return nil, errBad("%s header %q, want %q", name, got, strings.Join(sch.Headers, ","))
	}
	td := &TableData{Name: name, Headers: sch.Headers, Sep: detectSep(raw), FinalNewline: detectEnd(raw)}
	for _, rec := range recs[1:] {
		if len(rec) != len(sch.Headers) {
			return nil, errBad("%s row %q: %d values, want %d", name, rec, len(rec), len(sch.Headers))
		}
		td.Rows = append(td.Rows, Row(rec))
	}
	return td, nil
}

// ImportTable parses CSV bytes and appends every row into the named table
// through the store's append-only semantics.
func (s *Store) ImportTable(name string, raw []byte) error {
	td, err := ParseTable(name, raw)
	if err != nil {
		return err
	}
	dst, err := s.Table(name)
	if err != nil {
		return err
	}
	if len(dst.Rows) == 0 {
		dst.Sep, dst.FinalNewline = td.Sep, td.FinalNewline
	}
	for _, r := range td.Rows {
		if err := s.Append(name, r); err != nil {
			return err
		}
	}
	return nil
}

// --- helpers ---

func errNew(msg string) error { return errors.New(msg) }

func errWrap(err error, f string, a ...interface{}) error {
	return fmt.Errorf("%w: %s", err, fmt.Sprintf(f, a...))
}

func errBad(f string, a ...interface{}) error {
	return fmt.Errorf("%w: "+f, append([]interface{}{ErrBadRow}, a...)...)
}

func joinRec(rec []string) string { return strings.Join(rec, ",") }

// readRecords parses RFC-4180 CSV (accepts LF and CRLF, quoted fields).
func readRecords(raw []byte) ([][]string, error) {
	r := csv.NewReader(bytes.NewReader(raw))
	r.FieldsPerRecord = -1
	return r.ReadAll()
}

// detectSep returns the record separator used by the raw bytes: "\r\n" if
// any CRLF is present, else "\n".
func detectSep(raw []byte) string {
	if bytes.Contains(raw, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}

// detectEnd returns the exact trailing bytes of the file (the newline after
// the last record, or "" if none), so Export reproduces them byte-for-byte.
func detectEnd(raw []byte) string {
	if bytes.HasSuffix(raw, []byte("\r\n")) {
		return "\r\n"
	}
	if bytes.HasSuffix(raw, []byte("\n")) {
		return "\n"
	}
	return ""
}
