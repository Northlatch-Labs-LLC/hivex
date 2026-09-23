package gridframe

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Agent is one roster entry (roster.json, spec §2 attributes).
type Agent struct {
	ID         string  `json:"id"`
	Callsign   string  `json:"callsign"`
	Department string  `json:"department"`
	ReportsTo  *string `json:"reports_to"`
	ModelTier  string  `json:"model_tier"`
	Autonomy   string  `json:"autonomy"`
	KPI        string  `json:"kpi"`
	RoleCard   *string `json:"role_card_path"`
}

type rosterFile struct {
	Principal Agent   `json:"principal"`
	Agents    []Agent `json:"agents"`
}

// LoadRoster reads roster.json (25 agents + HOB-00 principal).
func LoadRoster(path string) (map[string]Agent, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("gridframe: roster: %w", err)
	}
	var rf rosterFile
	if err := json.Unmarshal(b, &rf); err != nil {
		return nil, fmt.Errorf("gridframe: roster %s: %w", path, err)
	}
	m := map[string]Agent{}
	if rf.Principal.ID != "" {
		m[rf.Principal.ID] = rf.Principal
	}
	for _, a := range rf.Agents {
		m[a.ID] = a
	}
	return m, nil
}

// AutonomyCeiling parses an autonomy string ("L1/L2" -> 2, "L0" -> 0).
// Unknown strings return -1.
func AutonomyCeiling(a string) int {
	max := -1
	for _, f := range strings.FieldsFunc(a, func(r rune) bool { return r == '/' || r == ' ' }) {
		f = strings.ToUpper(f)
		if len(f) == 2 && f[0] == 'L' && f[1] >= '0' && f[1] <= '3' {
			if v := int(f[1] - '0'); v > max {
				max = v
			}
		}
	}
	return max
}

// Dispatch is one role-card dispatch (Appendix B): role loaded, tier set,
// autonomy ceiling enforced, write scopes granted. One role per dispatch.
type Dispatch struct {
	Role    Agent
	Task    string
	Tier    string   // model tier set from roster
	Ceiling int      // autonomy ceiling (L0 = propose-only, never executes)
	Write   []string // allowed records subdirs, e.g. {"day","digests"}
}

// Dispatcher resolves role cards and writes records-store artifacts (§7).
type Dispatcher struct {
	roster map[string]Agent
	base   string // records store root (GRIDFRAME/records)
}

func NewDispatcher(rosterPath, recordsBase string) (*Dispatcher, error) {
	m, err := LoadRoster(rosterPath)
	if err != nil {
		return nil, err
	}
	return &Dispatcher{roster: m, base: recordsBase}, nil
}

// Dispatch spawns one role (Appendix B). Cross-role jobs call it once per
// role with disjoint Write scopes. L0 roles dispatch (propose-only) but
// cannot write artifacts.
func (d *Dispatcher) Dispatch(roleID, task string, write []string) (Dispatch, error) {
	if strings.ContainsAny(roleID, " ,/+") || roleID == "" {
		return Dispatch{}, fmt.Errorf("gridframe: dispatch %q: exactly one role per dispatch", roleID)
	}
	a, ok := d.roster[roleID]
	if !ok {
		return Dispatch{}, fmt.Errorf("gridframe: dispatch: unknown role %q", roleID)
	}
	c := AutonomyCeiling(a.Autonomy)
	if c < 0 {
		return Dispatch{}, fmt.Errorf("gridframe: role %s: bad autonomy %q", roleID, a.Autonomy)
	}
	return Dispatch{Role: a, Task: task, Tier: a.ModelTier, Ceiling: c, Write: write}, nil
}

// WriteArtifact writes a §7 record: every file begins with date, author agent
// ID, type. relPath is records-relative (e.g. "day/2026-09-23.md"). The
// dispatch's first Write scope must prefix the path; L0 dispatches are
// rejected (propose-only, spec §2).
func (d *Dispatcher) WriteArtifact(dp Dispatch, relPath, docType, body string, at time.Time) (string, error) {
	if dp.Ceiling < 1 {
		return "", fmt.Errorf("gridframe: %s is L0 propose-only; artifact write refused", dp.Role.ID)
	}
	relPath = filepath.ToSlash(filepath.Clean(relPath))
	okScope := false
	for _, s := range dp.Write {
		if s != "" && (relPath == s || strings.HasPrefix(relPath, s+"/")) {
			okScope = true
		}
	}
	if !okScope {
		return "", fmt.Errorf("gridframe: %s write scope %v excludes %q", dp.Role.ID, dp.Write, relPath)
	}
	full := filepath.Join(d.base, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	hdr := fmt.Sprintf("Date: %s\nAuthor: %s\nType: %s\n\n", at.In(CadenceLoc()).Format("2006-01-02"), dp.Role.ID, docType)
	if err := os.WriteFile(full, []byte(hdr+body+"\n"), 0o644); err != nil {
		return "", err
	}
	return full, nil
}
