package gridframe

import (
	"fmt"
	"strings"
	"time"

	"github.com/Northlatch-Labs-LLC/hivex/internal/calendar"
)

// CadenceLocation is the normative timezone for all 9 jobs (spec §6).
const CadenceLocation = "America/Los_Angeles"

// JobSpec is one of the 9 cadence jobs (spec §6). Cross-role jobs (#4,#7,#9)
// carry one role per dispatch, in order (Appendix B).
type JobSpec struct {
	Number   int
	Slug     string
	Label    string
	Cron     string // 5-field, evaluated in CadenceLocation
	When     string
	Roles    []string
	Artifact string
	OutNote  string
}

// JobBody is the executable part of a job (artifact write + ledger appends).
type JobBody func(tc TriggerContext) error

// TriggerContext is handed to an attached job body.
type TriggerContext struct {
	Job      *JobSpec
	At       time.Time
	Artifact string
}

// TriggerResult is the manual-trigger API's outcome.
type TriggerResult struct {
	Job      string
	Label    string
	At       string
	Artifact string
	Runs     []string // one entry per dispatched role
}

// CadenceJobs returns exactly the 9 scheduled jobs, in spec §6 order.
func CadenceJobs() []*JobSpec {
	return []*JobSpec{
		{1, "gridframe_day_plan", "Day plan", "0 8 * * 1-5", "Mon-Fri 08:00",
			[]string{"EXE-01"}, "records/day/{{.Date}}.md",
			"priorities vs sprint goal, approval-queue preview (flag T2/T3), open exceptions"},
		{2, "gridframe_principal_digest", "Principal digest", "30 17 * * 1-5", "Mon-Fri 17:30",
			[]string{"EXE-01"}, "records/digests/{{.Date}}.md",
			"KPI snapshot, changes, <=10 decisions w/ cost-of-delay, tomorrow's first act; ALSO notification surface"},
		{3, "gridframe_sprint_planning", "Sprint planning", "0 10 * * 1", "Mon 10:00",
			[]string{"EXE-00"}, "records/sprints/S-{{.Year}}-W{{.Week}}-plan.md",
			"one KPI-linked goal; >=1 item per dept; cost-weighted capacity (A day = 3x C); 20% exception reserve"},
		{4, "gridframe_release_train", "Release train", "0 10 * * 4", "Thu 10:00",
			[]string{"ENG-00", "ENG-03"}, "records/sprints/S-{{.Year}}-W{{.Week}}-release.md",
			"gate per DoD; blocked -> exception row (SEV3 unless customer-facing)"},
		{5, "gridframe_sprint_review", "Sprint review + retro", "0 15 * * 5", "Fri 15:00",
			[]string{"EXE-00"}, "records/sprints/S-{{.Year}}-W{{.Week}}-review.md",
			"goal met?, done/carried, demo, <=3 retro actions w/ owners; APPEND sprint-log row"},
		{6, "gridframe_close_kickoff", "Close kickoff", "0 9 1 * *", "monthly day 1, 09:00",
			[]string{"OPS-01"}, "records/close/{{.Year}}-{{.Month2}}.md",
			"checklist status per 07-ledgers §3; gaps -> SEV3 exceptions"},
		{7, "gridframe_close_aei", "Close + AEI publish", "0 9 3 * *", "monthly day 3, 09:00",
			[]string{"OPS-01", "RSK-03"}, "records/close/{{.Year}}-{{.Month2}}-aei.md",
			"AEI engine run (§5.3), append aei-monthly row, math shown; trigger §5.4 forced response"},
		{8, "gridframe_mbr_pack", "MBR pack", "0 9 4 * *", "monthly day 4, 09:00",
			[]string{"EXE-00"}, "records/mbr/{{.Year}}-{{.Month2}}.md",
			"KPIs vs targets, AEI + action, rev/cost summary, scorecard deltas, risks, targets, <=10 decisions"},
		{9, "gridframe_compliance_risk", "Compliance & risk", "0 9 5 * *", "monthly day 5, 09:00",
			[]string{"RSK-01", "RSK-00"}, "records/mbr/{{.Year}}-{{.Month2}}-compliance.md",
			"verify compliance-calendar next-90-days owners; overdue -> SEV2 IMMEDIATELY; reconcile milestones into state"},
	}
}

// FindJob resolves a job by slug (manual-trigger entry point).
func FindJob(slug string) (*JobSpec, error) {
	for _, j := range CadenceJobs() {
		if j.Slug == slug {
			return j, nil
		}
	}
	return nil, fmt.Errorf("gridframe: unknown cadence job %q", slug)
}

// CadenceLoc returns the jobs' timezone (tz-naive cron evaluated here).
func CadenceLoc() *time.Location {
	loc, err := time.LoadLocation(CadenceLocation)
	if err != nil {
		return time.UTC
	}
	return loc
}

// RenderArtifact fills the job's artifact template for wall-clock at,
// rendered in CadenceLocation. {{.Date}}=YYYY-MM-DD, {{.Year}}, {{.Month2}}
// zero-padded, {{.Week}} ISO week (§7 S-YYYY-Www naming).
func RenderArtifact(tpl string, at time.Time) string {
	t := at.In(CadenceLoc())
	_, w := t.ISOWeek()
	r := strings.NewReplacer(
		"{{.Date}}", t.Format("2006-01-02"),
		"{{.Year}}", t.Format("2006"),
		"{{.Month2}}", t.Format("01"),
		"{{.Week}}", fmt.Sprintf("%02d", w),
	)
	return r.Replace(tpl)
}

// NextRun computes the next fire time on/after at, in CadenceLocation.
func (j *JobSpec) NextRun(at time.Time) (time.Time, error) {
	sched, err := calendar.ParseCron(j.Cron)
	if err != nil {
		return time.Time{}, fmt.Errorf("gridframe: job %s cron %q: %w", j.Slug, j.Cron, err)
	}
	return sched.Next(at.In(CadenceLoc())), nil
}

// Registry is a manually triggerable set of jobs with optional bodies
// (bodies are attached in later G3 steps).
type Registry struct{ bodies map[string]JobBody }

func NewRegistry() *Registry { return &Registry{bodies: map[string]JobBody{}} }

func (r *Registry) SetBody(slug string, b JobBody) { r.bodies[slug] = b }

// Trigger is the manual-trigger API: resolve slug, render the artifact path,
// execute the attached body if any. Runs carries one entry per dispatched
// role, in order (one role per dispatch, Appendix B).
func (r *Registry) Trigger(slug string, at time.Time) (TriggerResult, error) {
	j, err := FindJob(slug)
	if err != nil {
		return TriggerResult{}, err
	}
	art := RenderArtifact(j.Artifact, at)
	res := TriggerResult{Job: j.Slug, Label: j.Label,
		At: at.In(CadenceLoc()).Format(time.RFC3339), Artifact: art}
	for _, role := range j.Roles {
		res.Runs = append(res.Runs, role)
	}
	if b := r.bodies[slug]; b != nil {
		if err := b(TriggerContext{Job: j, At: at.In(CadenceLoc()), Artifact: art}); err != nil {
			return res, fmt.Errorf("gridframe: job %s body: %w", slug, err)
		}
	}
	return res, nil
}
