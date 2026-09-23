package gridframe

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// stateFixture is the §8 example document, verbatim shape.
const stateFixture = `{
  "company": "Gridframe Systems, LLC",
  "state": "formation-pending",
  "formation": {"bizee_order":"541511","filed":"2026-09-22","certificate":"pending",
                "ein":"pending","operating_agreement":"drafted-awaiting-signature","bank":"pending"},
  "infrastructure": {"ovh":"ordered-pending-build",
    "control_plane":"ADVANCE-1 (EPYC 4245P, 6c/32G, 2x960G NVMe)",
    "env_node":"Scale class 32c/256G NVMe (exact SKU at cart)"},
  "current_sprint": "S-2026-W39",
  "sprint_goal": "Formation complete + OVH provisioned + first paying self-serve cohort, zero SEV1/SEV2",
  "next_milestones": [
    {"id":"CMP-0005","what":"EIN via IRS (free)","due":"2026-09-24","owner":"Principal + Counsel prep"},
    {"id":"CMP-0006","what":"Sign operating agreement","due":"2026-09-26","owner":"Principal"},
    {"id":"CMP-0007","what":"Bank account","due":"2026-09-30","owner":"Principal"},
    {"id":"OVH-PROV","what":"Provision bare metal on delivery","due":"on delivery","owner":"ENG-04 Bastion"}],
  "reappraisal": {"next_cycle":"2026-Q4","method":"S/A/B/C/D + marginal-AEI rebalancing"}
}`

func writeState(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(p, []byte(stateFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Acceptance test 8 (§10): CMP-0005 marked done → formation.ein updates
// and the milestone leaves next_milestones.
func TestMilestoneDoneFlipsFormationField(t *testing.T) {
	p := writeState(t)
	st, err := LoadState(p)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if st.Formation.EIN != "pending" || len(st.NextMilestones) != 4 {
		t.Fatalf("fixture loaded wrong: ein=%q milestones=%d", st.Formation.EIN, len(st.NextMilestones))
	}
	if !st.MilestoneDone("CMP-0005", "12-3456789") {
		t.Fatal("MilestoneDone reported milestone not listed")
	}
	if st.Formation.EIN != "12-3456789" {
		t.Fatalf("formation.ein = %q, want 12-3456789", st.Formation.EIN)
	}
	if len(st.NextMilestones) != 3 {
		t.Fatalf("milestones = %d, want 3", len(st.NextMilestones))
	}
	for _, m := range st.NextMilestones {
		if m.ID == "CMP-0005" {
			t.Fatal("CMP-0005 still in next_milestones")
		}
	}
}

// Save → Load round-trip preserves the whole document.
func TestStateSaveLoadRoundTrip(t *testing.T) {
	p := writeState(t)
	st, _ := LoadState(p)
	_ = st.MilestoneDone("CMP-0006", "signed")
	if err := st.Save(p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	st2, err := LoadState(p)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if st2.Formation.OperatingAgreement != "signed" || len(st2.NextMilestones) != 3 {
		t.Fatalf("round-trip lost data: oa=%q milestones=%d",
			st2.Formation.OperatingAgreement, len(st2.NextMilestones))
	}
	b, _ := os.ReadFile(p)
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("saved file not valid JSON: %v", err)
	}
}

// Unknown milestone: no formation change, no error, not-listed result false.
func TestMilestoneDoneUnknown(t *testing.T) {
	st, _ := LoadState(writeState(t))
	if st.MilestoneDone("CMP-9999", "x") {
		t.Fatal("unknown milestone reported as listed")
	}
	if st.Formation.EIN != "pending" || len(st.NextMilestones) != 4 {
		t.Fatal("unknown milestone mutated state")
	}
}
