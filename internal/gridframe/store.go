package gridframe

import (
	"errors"
	"fmt"
)

// Spec §5.1: append-only; edits of existing rows are rejected; corrections
// are NEW reversing rows referencing the original ID; IDs are never reused.

var (
	ErrEditExisting       = errors.New("gridframe: edit of existing row rejected (append-only)")
	ErrIDReuse            = errors.New("gridframe: row ID already used; IDs are never reused")
	ErrUnknownTable       = errors.New("gridframe: unknown table")
	ErrBadRow             = errors.New("gridframe: row does not match table schema")
	ErrUnknownOriginal    = errors.New("gridframe: reversing row references unknown original ID")
	ErrReversalOfReversal = errors.New("gridframe: cannot reverse a reversing row")
)

// Row is one ledger row: values aligned positionally with the table Headers.
type Row []string

// Get returns the value in the named column ("" if absent).
func (r Row) Get(t Table, col string) string {
	for i, h := range t.Headers {
		if h == col && i < len(r) {
			return r[i]
		}
	}
	return ""
}

// Reversal links a reversing row to the row it corrects.
type Reversal struct {
	OriginalID string
	ReverseID  string
}

// TableData is the append-only content of one ledger table.
type TableData struct {
	Name         string
	Headers      []string
	Rows         []Row
	Reversals    []Reversal
	Sep          string // record separator captured at import ("" = "\n")
	FinalNewline string // trailing bytes after the last record, captured at import
}

// Store is the in-memory append-only ledger store for all 8 tables.
// Persistence (CSV) lives in csv.go; month-lock in lock.go.
type Store struct {
	tables map[string]*TableData
	locks  map[string]bool // locked months "YYYY-MM" (lock.go)
	// persistDir, when armed via PersistTo (persist.go), is the directory
	// every mutation is flushed to as byte-exact CSV.
	persistDir string
}

// NewStore returns an empty store with all 8 tables initialized from schema.
func NewStore() *Store {
	s := &Store{tables: map[string]*TableData{}, locks: map[string]bool{}}
	for _, t := range Tables() {
		s.tables[t.Name] = &TableData{Name: t.Name, Headers: t.Headers}
	}
	return s
}

// Table returns the TableData for a table name.
func (s *Store) Table(name string) (*TableData, error) {
	td, ok := s.tables[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownTable, name)
	}
	return td, nil
}

// hasID reports whether id is used as a row ID or either side of a reversal.
func (s *Store) hasID(name, id string) bool {
	sch, _ := Lookup(name)
	td, _ := s.Table(name)
	if td == nil || sch.IDCol == "" {
		return false
	}
	for _, r := range td.Rows {
		if r.Get(sch, sch.IDCol) == id {
			return true
		}
	}
	for _, rev := range td.Reversals {
		if rev.ReverseID == id || rev.OriginalID == id {
			return true
		}
	}
	return false
}

func (s *Store) isReversal(name, id string) bool {
	td, _ := s.Table(name)
	if td == nil {
		return false
	}
	for _, rev := range td.Reversals {
		if rev.ReverseID == id {
			return true
		}
	}
	return false
}

func validate(name string, r Row) error {
	sch, ok := Lookup(name)
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownTable, name)
	}
	if len(r) != len(sch.Headers) {
		return fmt.Errorf("%w: %s: %d values, want %d", ErrBadRow, name, len(r), len(sch.Headers))
	}
	return nil
}

// keyOf is the edit-detection key for tables without an ID column
// (aei-monthly: month; agent-scorecards: cycle|agent_id).
func keyOf(sch Table, r Row) string {
	if len(r) == 0 || len(sch.Headers) == 0 {
		return ""
	}
	k := r[0]
	if i := indexOf(sch.Headers, "agent_id"); i >= 0 && i < len(r) {
		k += "|" + r[i]
	}
	return k
}

func indexOf(hs []string, h string) int {
	for i, v := range hs {
		if v == h {
			return i
		}
	}
	return -1
}

func (td *TableData) hasKey(sch Table, key string) bool {
	for _, r := range td.Rows {
		if keyOf(sch, r) == key {
			return true
		}
	}
	return false
}

// Append adds a new row to a table. A row whose ID (when the table has an
// IDCol) is already present is an edit attempt and is rejected. For tables
// without an IDCol, a re-append with the same date|first-col key is rejected.
func (s *Store) Append(name string, r Row) error {
	if err := validate(name, r); err != nil {
		return err
	}
	if err := s.checkLock(name, r); err != nil {
		return err
	}
	sch, _ := Lookup(name)
	td, err := s.Table(name)
	if err != nil {
		return err
	}
	if sch.IDCol != "" {
		if id := r.Get(sch, sch.IDCol); s.hasID(name, id) {
			return fmt.Errorf("%w: %s %s", ErrEditExisting, name, id)
		}
	} else if k := keyOf(sch, r); k != "" && td.hasKey(sch, k) {
		return fmt.Errorf("%w: %s key %q", ErrEditExisting, name, k)
	}
	td.Rows = append(td.Rows, r)
	s.persistTable(name)
	return nil
}

// AppendReversal appends a NEW correcting row reversing originalID (spec §5.1:
// "corrections are new reversing rows referencing the original ID"). rev must
// carry a fresh never-used ID; the original must exist and not be a reversal.
func (s *Store) AppendReversal(name, originalID string, rev Row) error {
	if err := validate(name, rev); err != nil {
		return err
	}
	if err := s.checkLock(name, rev); err != nil {
		return err
	}
	sch, _ := Lookup(name)
	if sch.IDCol == "" {
		return fmt.Errorf("%w: %s has no ID column", ErrBadRow, name)
	}
	if !s.hasID(name, originalID) {
		return fmt.Errorf("%w: %s %s", ErrUnknownOriginal, name, originalID)
	}
	if s.isReversal(name, originalID) {
		return fmt.Errorf("%w: %s %s", ErrReversalOfReversal, name, originalID)
	}
	revID := rev.Get(sch, sch.IDCol)
	if s.hasID(name, revID) {
		return fmt.Errorf("%w: %s %s", ErrIDReuse, name, revID)
	}
	td, err := s.Table(name)
	if err != nil {
		return err
	}
	td.Rows = append(td.Rows, rev)
	td.Reversals = append(td.Reversals, Reversal{OriginalID: originalID, ReverseID: revID})
	s.persistTable(name)
	return nil
}

// With returns a copy of r with the named column set to value.
func (r Row) With(t Table, col, value string) Row {
	out := make(Row, len(t.Headers))
	copy(out, r)
	for i, h := range t.Headers {
		if h == col && i < len(out) {
			out[i] = value
		}
	}
	return out
}
