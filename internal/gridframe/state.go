package gridframe

import (
	"encoding/json"
	"os"
)

// State service (spec §8): the single GRIDFRAME/state.json document, kept
// current by cadence jobs and milestone events.

// Milestone is one next_milestones entry (§8).
type Milestone struct {
	ID    string `json:"id"`
	What  string `json:"what"`
	Due   string `json:"due"`
	Owner string `json:"owner"`
}

// Formation is the formation block (§8).
type Formation struct {
	BizeeOrder         string `json:"bizee_order"`
	Filed              string `json:"filed"`
	Certificate        string `json:"certificate"`
	EIN                string `json:"ein"`
	OperatingAgreement string `json:"operating_agreement"`
	Bank               string `json:"bank"`
}

// Infrastructure is the infrastructure block (§8).
type Infrastructure struct {
	OVH          string `json:"ovh"`
	ControlPlane string `json:"control_plane"`
	EnvNode      string `json:"env_node"`
}

// Reappraisal is the reappraisal block (§8).
type Reappraisal struct {
	NextCycle string `json:"next_cycle"`
	Method    string `json:"method"`
}

// State is the full state.json document (§8 shape).
type State struct {
	Company        string         `json:"company"`
	StateStatus    string         `json:"state"`
	Formation      Formation      `json:"formation"`
	Infrastructure Infrastructure `json:"infrastructure"`
	CurrentSprint  string         `json:"current_sprint"`
	SprintGoal     string         `json:"sprint_goal"`
	NextMilestones []Milestone    `json:"next_milestones"`
	Reappraisal    Reappraisal    `json:"reappraisal"`
}

// LoadState reads state.json from path.
func LoadState(path string) (*State, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var st State
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// Save writes the state document to path (atomic via temp+rename).
func (st *State) Save(path string) error {
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// formationField maps a milestone ID to the state.json field it advances
// (acceptance test 8: "CMP-0005 marked done → formation.ein updates").
// CMP-0006→operating_agreement and CMP-0007→bank follow from the §8
// next_milestones list (IDs are permanent, spec §2). Infrastructure has
// no milestone field mapping yet; OVH-PROV is tracked below.
type stateField struct {
	set func(st *State, v string)
}

var milestoneFields = map[string]stateField{
	"CMP-0005": {func(st *State, v string) { st.Formation.EIN = v }},
	"CMP-0006": {func(st *State, v string) { st.Formation.OperatingAgreement = v }},
	"CMP-0007": {func(st *State, v string) { st.Formation.Bank = v }},
	"OVH-PROV": {func(st *State, v string) { st.Infrastructure.OVH = v }},
}

// MilestoneDone records a milestone as done: sets the mapped state field
// (if any and value is non-empty), drops the milestone from next_milestones,
// and returns true if the milestone was listed there. The companion
// compliance-calendar status flip is a ledger append performed by the
// caller (cadence job 9 / service wiring), not by the state document.
func (st *State) MilestoneDone(id, value string) bool {
	if f, ok := milestoneFields[id]; ok && value != "" {
		f.set(st, value)
	}
	before := len(st.NextMilestones)
	kept := st.NextMilestones[:0:0]
	for _, m := range st.NextMilestones {
		if m.ID != id {
			kept = append(kept, m)
		}
	}
	st.NextMilestones = kept
	return len(kept) != before
}
