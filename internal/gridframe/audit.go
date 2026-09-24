package gridframe

// Audit sampler (§4): RSK-03 samples ≥ 5% of executed T1+ actions monthly.
// Executed = approval-queue rows whose status is "approved" (the gate
// records the T1 dept-lead decision synchronously; T2/T3 flip only on the
// human approval that unblocks execution). Verdicts: "compliant" or
// "violation"; violations feed the responsible lead's scorecard (§5.5).

import "sort"

// MinSampleRate is the audit floor: at least 5% of executed T1+ actions per month.
const MinSampleRate = 0.05

// Verdict values.
const (
	VerdictCompliant = "compliant"
	VerdictViolation = "violation"
)

// Verdict is one sampled action with RSK-03's review verdict.
type Verdict struct {
	ItemID     string
	Agent      string // raised_by
	Dept       string
	Tier       string
	RaisedDate string
	Verdict    string // compliant | violation
	Note       string
}

var (
	ErrBadVerdict = errNew("gridframe: verdict must be compliant or violation")
	ErrNotSampled = errNew("gridframe: item is not in the audit sample")
)

// Sampler draws the monthly audit sample and holds verdict rows.
type Sampler struct {
	s        *Store
	sampled  map[string]bool // item_ids in the sample (all months)
	verdicts map[string]Verdict
}

func NewSampler(s *Store) *Sampler {
	return &Sampler{s: s, sampled: map[string]bool{}, verdicts: map[string]Verdict{}}
}

// ExecutedT1Plus lists approved T1/T2/T3 queue rows raised in month
// ("YYYY-MM"), ordered by item_id — the population §4 samples from.
func (sp *Sampler) ExecutedT1Plus(month string) []Row {
	sch, _ := Lookup(TApprov)
	td, _ := sp.s.Table(TApprov)
	var out []Row
	for _, r := range td.Rows {
		d := r.Get(sch, "raised_date")
		if len(d) < 7 || d[:7] != month {
			continue
		}
		t := r.Get(sch, "tier")
		if t != Tier1 && t != Tier2 && t != Tier3 {
			continue
		}
		if r.Get(sch, "status") != "approved" {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Get(sch, "item_id") < out[j].Get(sch, "item_id")
	})
	return out
}

// Sample draws the audit sample for month: ceil(5% of executed count)
// item_ids, at least 1 whenever anything executed. Deterministic: rows are
// taken evenly spaced from the item_id-ordered population, so re-running
// the same month yields the same sample.
func (sp *Sampler) Sample(month string) []string {
	rows := sp.ExecutedT1Plus(month)
	n := len(rows)
	if n == 0 {
		return nil
	}
	k := int(0.05 * float64(n))
	if float64(k) < MinSampleRate*float64(n) {
		k++
	}
	if k < 1 {
		k = 1
	}
	sch, _ := Lookup(TApprov)
	ids := make([]string, 0, k)
	for i := 0; i < k; i++ {
		idx := i * n / k
		ids = append(ids, rows[idx].Get(sch, "item_id"))
		sp.sampled[rows[idx].Get(sch, "item_id")] = true
	}
	return ids
}

// RecordVerdict records RSK-03's review verdict for a sampled item. Only
// sampled items can carry verdicts; unknown items are rejected fail-closed.
func (sp *Sampler) RecordVerdict(itemID, verdict, note string) (Verdict, error) {
	if !sp.sampled[itemID] {
		return Verdict{}, ErrNotSampled
	}
	if verdict != VerdictCompliant && verdict != VerdictViolation {
		return Verdict{}, ErrBadVerdict
	}
	sch, _ := Lookup(TApprov)
	td, _ := sp.s.Table(TApprov)
	for _, r := range td.Rows {
		if r.Get(sch, "item_id") != itemID {
			continue
		}
		v := Verdict{ItemID: itemID, Agent: r.Get(sch, "raised_by"),
			Dept: r.Get(sch, "dept"), Tier: r.Get(sch, "tier"),
			RaisedDate: r.Get(sch, "raised_date"), Verdict: verdict, Note: note}
		sp.verdicts[itemID] = v
		return v, nil
	}
	return Verdict{}, ErrExcNotFound
}

// Violations lists verdict rows marked violation — the §5.5 scorecard feed
// for the responsible leads.
func (sp *Sampler) Violations() []Verdict {
	out := make([]Verdict, 0, len(sp.verdicts))
	for _, v := range sp.verdicts {
		if v.Verdict == VerdictViolation {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ItemID < out[j].ItemID })
	return out
}
