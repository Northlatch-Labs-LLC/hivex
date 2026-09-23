package gridframe

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// AeiResult is the §5.3 monthly derivation. cost_c in the aei-monthly schema
// carries C = D + L + A (seed row 2026-09: 647.70), not a "block C" sum.
type AeiResult struct {
	Month               string
	Revenue, Pipeline   float64
	ValueV              float64 // revenue + 0.10 × pipeline
	CostD, CostL, CostA float64
	CostC               float64 // D + L + A
	AEI                 float64 // V / C; 0 when C = 0
	Status              string  // §5.4 band name
	ActionRef           string
}

// BandFor returns the §5.4 band for an AEI value.
func BandFor(aei float64) string {
	switch {
	case aei < 1.0:
		return "survival"
	case aei < 2.0:
		return "warning"
	case aei <= 4.0:
		return "healthy"
	default:
		return "investigate"
	}
}

func sumMonth(s *Store, table, month, col, block string) (float64, error) {
	td, err := s.Table(table)
	if err != nil {
		return 0, err
	}
	sch, _ := Lookup(table)
	var sum float64
	for _, r := range td.Rows {
		d := r.Get(sch, sch.DateCol)
		if month != "" && !strings.HasPrefix(d, month) {
			continue
		}
		if block != "" && r.Get(sch, "block") != block {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(r.Get(sch, col)), 64)
		if err != nil {
			return 0, fmt.Errorf("gridframe: %s %s=%q: %w", table, col, r.Get(sch, col), err)
		}
		sum += v
	}
	return math.Round(sum*100) / 100, nil
}

// DeriveAEI computes §5.3 for month "YYYY-MM" from the store's ledgers.
// pipeline is the monthly verified input field (REV-00, close pack).
func DeriveAEI(s *Store, month string, pipeline float64, actionRef string) (AeiResult, error) {
	rev, err := sumMonth(s, TRevenue, month, "amount_usd", "")
	if err != nil {
		return AeiResult{}, err
	}
	d, err := sumMonth(s, TCost, month, "amount_usd", "D")
	if err != nil {
		return AeiResult{}, err
	}
	l, err := sumMonth(s, TCost, month, "amount_usd", "L")
	if err != nil {
		return AeiResult{}, err
	}
	a, err := sumMonth(s, TCost, month, "amount_usd", "A")
	if err != nil {
		return AeiResult{}, err
	}
	c := math.Round((d+l+a)*100) / 100
	v := math.Round((rev+0.10*pipeline)*100) / 100
	aei := 0.0
	if c != 0 {
		aei = math.Round(v/c*100) / 100
	}
	return AeiResult{Month: month, Revenue: rev, Pipeline: pipeline,
		ValueV: v, CostD: d, CostL: l, CostA: a, CostC: c,
		AEI: aei, Status: BandFor(aei), ActionRef: actionRef}, nil
}

// AeiRow renders a §5.1 aei-monthly row from a derivation. It is only
// reachable from a DeriveAEI result — manual/hand-entered derived rows have
// no path; the store additionally rejects re-append of a month (edit).
func (r AeiResult) AeiRow() Row {
	sch, _ := Lookup("aei-monthly")
	f := func(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }
	row := make(Row, len(sch.Headers))
	row = row.With(sch, "month", r.Month)
	row = row.With(sch, "revenue", f(r.Revenue))
	row = row.With(sch, "pipeline", f(r.Pipeline))
	row = row.With(sch, "value_v", f(r.ValueV))
	row = row.With(sch, "cost_d", f(r.CostD))
	row = row.With(sch, "cost_l", f(r.CostL))
	row = row.With(sch, "cost_a", f(r.CostA))
	row = row.With(sch, "cost_c", f(r.CostC))
	row = row.With(sch, "aei", f(r.AEI))
	row = row.With(sch, "status", r.Status)
	row = row.With(sch, "action_ref", r.ActionRef)
	return row
}

// AppendAeiRow is the engine path: appends the derived row. A second append
// for the same month is rejected by the store as an edit of a derived row.
func AppendAeiRow(s *Store, r AeiResult) error {
	return s.Append("aei-monthly", r.AeiRow())
}

// AeiCheck is RSK-03's independent re-derivation verdict (§5.3: re-derives
// independently in the same job and records agreement/disagreement).
type AeiCheck struct {
	Month   string
	Agrees  bool
	Diffs   []string
	Rebuilt AeiResult
}

// ReverifyAEI re-derives from the ledgers and compares against the published
// row field by field. Comparing to the appended store row (not a passed-in
// result) keeps the path independent.
func ReverifyAEI(s *Store, month string, pipeline float64, published Row) (AeiCheck, error) {
	sch, _ := Lookup("aei-monthly")
	res, err := DeriveAEI(s, month, pipeline, published.Get(sch, "action_ref"))
	if err != nil {
		return AeiCheck{}, err
	}
	want := res.AeiRow()
	ck := AeiCheck{Month: month, Agrees: true, Rebuilt: res}
	for i, h := range sch.Headers {
		if h == "action_ref" {
			continue
		}
		if published[i] != want[i] {
			ck.Agrees = false
			ck.Diffs = append(ck.Diffs, fmt.Sprintf("%s: published %q != re-derived %q", h, published[i], want[i]))
		}
	}
	return ck, nil
}
