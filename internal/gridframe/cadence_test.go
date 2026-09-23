package gridframe

import (
	"testing"
	"time"
)

func TestCadenceNineJobsExactSchedules(t *testing.T) {
	js := CadenceJobs()
	if len(js) != 9 {
		t.Fatalf("got %d jobs, want 9", len(js))
	}
	want := []struct {
		n, slug, cron, role string
	}{
		{"1", "gridframe_day_plan", "0 8 * * 1-5", "EXE-01"},
		{"2", "gridframe_principal_digest", "30 17 * * 1-5", "EXE-01"},
		{"3", "gridframe_sprint_planning", "0 10 * * 1", "EXE-00"},
		{"4", "gridframe_release_train", "0 10 * * 4", "ENG-00"},
		{"5", "gridframe_sprint_review", "0 15 * * 5", "EXE-00"},
		{"6", "gridframe_close_kickoff", "0 9 1 * *", "OPS-01"},
		{"7", "gridframe_close_aei", "0 9 3 * *", "OPS-01"},
		{"8", "gridframe_mbr_pack", "0 9 4 * *", "EXE-00"},
		{"9", "gridframe_compliance_risk", "0 9 5 * *", "RSK-01"},
	}
	for i, w := range want {
		j := js[i]
		if j.Slug != w.slug || j.Cron != w.cron || j.Roles[0] != w.role {
			t.Errorf("job %s: got {%s %s %v}, want {%s %s %s}",
				w.slug, j.Slug, j.Cron, j.Roles, w.slug, w.cron, w.role)
		}
		if _, err := j.NextRun(time.Now()); err != nil {
			t.Errorf("job %s NextRun: %v", j.Slug, err)
		}
	}
}
func TestTriggerRendersPacificTime(t *testing.T) {
	reg := NewRegistry()
	// 2026-09-23 01:30 UTC == 2026-09-22 18:30 PDT.
	res, err := reg.Trigger("gridframe_principal_digest", time.Date(2026, 9, 23, 1, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if res.Artifact != "records/digests/2026-09-22.md" {
		t.Errorf("artifact %q, want records/digests/2026-09-22.md", res.Artifact)
	}
	if res.At != "2026-09-22T18:30:00-07:00" {
		t.Errorf("At %q, want 2026-09-22T18:30:00-07:00", res.At)
	}
	if len(res.Runs) != 1 || res.Runs[0] != "EXE-01" {
		t.Errorf("Runs %v, want [EXE-01]", res.Runs)
	}
}
func TestTriggerCrossRoleAndErrors(t *testing.T) {
	res, err := NewRegistry().Trigger("gridframe_close_aei", time.Date(2026, 10, 3, 16, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Runs) != 2 || res.Runs[0] != "OPS-01" || res.Runs[1] != "RSK-03" {
		t.Errorf("Runs %v, want [OPS-01 RSK-03]", res.Runs)
	}
	if res.Artifact != "records/close/2026-10-aei.md" {
		t.Errorf("artifact %q, want records/close/2026-10-aei.md", res.Artifact)
	}
	if _, err := NewRegistry().Trigger("nope", time.Now()); err == nil {
		t.Error("unknown slug: want error")
	}
}

func TestTriggerBodyExecuted(t *testing.T) {
	reg := NewRegistry()
	var got string
	reg.SetBody("gridframe_day_plan", func(tc TriggerContext) error {
		got = tc.Artifact
		return nil
	})
	res, err := reg.Trigger("gridframe_day_plan", time.Date(2026, 9, 23, 15, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if res.Artifact != "records/day/2026-09-23.md" || got != res.Artifact {
		t.Errorf("body artifact %q / %q, want records/day/2026-09-23.md", got, res.Artifact)
	}
}
