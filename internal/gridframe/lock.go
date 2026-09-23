package gridframe

// Month-lock (spec §5.1: "monthly close (day 3) locks the month — writes to
// locked months are rejected by the store"). A row's month is the YYYY-MM
// prefix of its DateCol value. Tables keyed by week/quarter (sprint-log,
// agent-scorecards) have no month and are unaffected.

import "regexp"

var ErrLockedMonth = errNew("write to locked month rejected")

var monthRe = regexp.MustCompile(`^\d{4}-\d{2}`)

// monthOf returns the YYYY-MM month a row belongs to, or "" if its key
// column is not a month/date value.
func monthOf(sch Table, r Row) string {
	if sch.DateCol == "" {
		return ""
	}
	v := r.Get(sch, sch.DateCol)
	if len(v) >= 7 && monthRe.MatchString(v[:7]) {
		return v[:7]
	}
	return ""
}

// LockMonth locks a month ("YYYY-MM"); writes to it are rejected.
func (s *Store) LockMonth(month string) {
	if s.locks == nil {
		s.locks = map[string]bool{}
	}
	s.locks[month] = true
}

// IsLockedMonth reports whether a month is locked.
func (s *Store) IsLockedMonth(month string) bool {
	return s.locks[month]
}

// LockedMonths returns all locked months.
func (s *Store) LockedMonths() []string {
	out := make([]string, 0, len(s.locks))
	for m := range s.locks {
		out = append(out, m)
	}
	return out
}

// checkLock rejects a row whose month is locked. A reversing row is judged by
// its OWN date (the write lands in that month); the original's month is not
// re-checked, matching the spec's wording verbatim.
func (s *Store) checkLock(name string, r Row) error {
	sch, ok := Lookup(name)
	if !ok {
		return errWrap(ErrUnknownTable, "%s", name)
	}
	if m := monthOf(sch, r); m != "" && s.locks[m] {
		return errWrap(ErrLockedMonth, "%s %s", name, m)
	}
	return nil
}
