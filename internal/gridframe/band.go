package gridframe

import (
	"fmt"
	"math"
	"strconv"
)

// BandResponse is a §5.4 forced response — an automated action set, not advice.
type BandResponse struct {
	Band           string   // §5.4 band name (matches BandFor)
	FreezeT1Spend  bool     // survival: freeze T1+ discretionary spend
	Actions        []string // machine-readable forced-action IDs
	Owners         []string // roles on the hook
	DeadlineHours  int      // survival: recovery plan to Principal
	NextSprintOwed bool     // warning: dept efficiency actions next sprint
}

// ForcedResponse maps an AEI value to its §5.4 forced response.
func ForcedResponse(aei float64) BandResponse {
	switch b := BandFor(aei); b {
	case "survival":
		return BandResponse{Band: b, FreezeT1Spend: true,
			Actions: []string{"freeze-t1-discretionary-spend", "recovery-plan-to-principal"},
			Owners:  []string{"EXE-00"}, DeadlineHours: 48}
	case "warning":
		return BandResponse{Band: b,
			Actions: []string{"dept-efficiency-actions"},
			Owners:  []string{"dept-leads"}, NextSprintOwed: true}
	case "healthy":
		return BandResponse{Band: b, Actions: []string{"continue"}}
	default: // investigate
		return BandResponse{Band: b,
			Actions: []string{"demand-investment-proposal"},
			Owners:  []string{"PPL-00", "REV-00"}}
	}
}

// LatestAEI returns the most recent aei-monthly row's AEI value + status.
func LatestAEI(s *Store) (aei float64, status string, month string, err error) {
	td, err := s.Table(TAEI)
	if err != nil {
		return 0, "", "", err
	}
	if len(td.Rows) == 0 {
		return 0, "", "", fmt.Errorf("gridframe: no aei-monthly rows")
	}
	sch, _ := Lookup(TAEI)
	// rows are append-ordered; find max month to be safe
	var best Row
	for _, r := range td.Rows {
		if best == nil || r.Get(sch, "month") > best.Get(sch, "month") {
			best = r
		}
	}
	v, err := strconv.ParseFloat(best.Get(sch, "aei"), 64)
	if err != nil {
		return 0, "", "", fmt.Errorf("gridframe: aei %q: %w", best.Get(sch, "aei"), err)
	}
	return v, best.Get(sch, "status"), best.Get(sch, "month"), nil
}

// SpendFreezeActive reports whether the survival-band T1+ discretionary
// spend freeze is currently in force (latest aei-monthly band = survival).
// The governance gate consults this before executing T1+ discretionary spend.
func SpendFreezeActive(s *Store) (bool, error) {
	aei, _, _, err := LatestAEI(s)
	if err != nil {
		return false, err
	}
	return BandFor(aei) == "survival", nil
}

// ScorecardInput is one agent's §5.5 inputs.
type ScorecardInput struct {
	Cycle     string // e.g. "2026-Q4"
	AgentID   string
	KpiPct    float64 // 0-100
	Quality   float64 // audit sample 1-5
	CostEff   float64 // attributable value ÷ inference cost
	Collab    float64 // peer feedback 1-5
	Incidents int
}

// Scorecard is the §5.5 outcome. Composite formula (spec gives the inputs and
// the S/A/B/C/D scale but no formula; this is the build's defined formula):
// composite = 0.40*(kpi/100) + 0.20*(quality/5) + 0.20*(collab/5)
//   - 0.20*min(cost_eff/2, 1) - 0.05*incidents, floored at 0.
type Scorecard struct {
	Cycle, AgentID, Grade string
	Composite             float64
	Action                string
	RequiresT3            bool // D: retire/merge needs T3 human approval
}

func Score(sc ScorecardInput) Scorecard {
	c := 0.40*(sc.KpiPct/100) + 0.20*(sc.Quality/5) + 0.20*(sc.Collab/5) + 0.20*math.Min(sc.CostEff/2, 1)
	c -= 0.05 * float64(sc.Incidents)
	if c < 0 {
		c = 0
	}
	c = math.Round(c*100) / 100
	var grade, action string
	var t3 bool
	switch {
	case c >= 0.90:
		grade, action = "S", "expand autonomy"
	case c >= 0.75:
		grade, action = "A", "maintain"
	case c >= 0.60:
		grade, action = "B", "coach 30-day re-check"
	case c >= 0.40:
		grade, action = "C", "retune/model-downshift 60-day"
	default:
		grade, action, t3 = "D", "retire/merge", true
	}
	return Scorecard{Cycle: sc.Cycle, AgentID: sc.AgentID, Grade: grade,
		Composite: c, Action: action, RequiresT3: t3}
}

// AppendScorecardRow appends the §5.1 agent-scorecards row for one agent.
// The row records the mandate only; executing a D-grade retirement still
// goes through the T3 gate (RequiresT3 on the Scorecard).
func AppendScorecardRow(s *Store, in ScorecardInput, sc Scorecard) error {
	sch, _ := Lookup(TScore)
	row := make(Row, len(sch.Headers))
	row = row.With(sch, "cycle", in.Cycle)
	row = row.With(sch, "agent_id", in.AgentID)
	row = row.With(sch, "kpi_pct", fmt.Sprintf("%.0f", in.KpiPct))
	row = row.With(sch, "quality", fmt.Sprintf("%.1f", in.Quality))
	row = row.With(sch, "cost_eff", fmt.Sprintf("%.2f", in.CostEff))
	row = row.With(sch, "collab", fmt.Sprintf("%.1f", in.Collab))
	row = row.With(sch, "incidents", fmt.Sprintf("%d", in.Incidents))
	row = row.With(sch, "composite", fmt.Sprintf("%.2f", sc.Composite))
	row = row.With(sch, "action", sc.Action)
	return s.Append(TScore, row)
}
