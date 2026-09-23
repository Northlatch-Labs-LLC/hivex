package gridframe

import (
	"os"
	"strings"
	"testing"
	"time"
)

const testRoster = "../../hivex-home/.hivex/GRIDFRAME/roster.json"

func TestLoadRoster25PlusPrincipal(t *testing.T) {
	m, err := LoadRoster(testRoster)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 26 {
		t.Fatalf("roster %d entries, want 26 (25 agents + HOB-00)", len(m))
	}
	a := m["EXE-00"]
	if a.ModelTier != "A" || a.Callsign != "Atlas" || a.Department != "GOV" {
		t.Errorf("EXE-00 = %+v", a)
	}
	if m["RSK-00"].ReportsTo == nil || *m["RSK-00"].ReportsTo != "HOB-00" {
		t.Error("RSK-00 must report to HOB-00 (independent line)")
	}
	if AutonomyCeiling("L1/L2") != 2 || AutonomyCeiling("L0") != 0 || AutonomyCeiling("L2/L3") != 3 || AutonomyCeiling("x") != -1 {
		t.Error("AutonomyCeiling parsing")
	}
}

func TestDispatchOneRoleTierAndCeiling(t *testing.T) {
	d, err := NewDispatcher(testRoster, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dp, err := d.Dispatch("OPS-01", "run close", []string{"close"})
	if err != nil {
		t.Fatal(err)
	}
	if dp.Tier != "A" || dp.Ceiling != 1 || dp.Role.Callsign != "Tally" {
		t.Errorf("OPS-01 dispatch = %+v", dp)
	}
	if _, err := d.Dispatch("EXE-00 EXE-01", "two", nil); err == nil {
		t.Error("two roles in one dispatch: want error")
	}
	if _, err := d.Dispatch("NOPE-00", "x", nil); err == nil {
		t.Error("unknown role: want error")
	}
}
func TestWriteArtifactHeaderScopeAndL0(t *testing.T) {
	base := t.TempDir()
	d, err := NewDispatcher(testRoster, base)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 23, 15, 0, 0, 0, time.UTC) // 08:00 PDT
	dp, _ := d.Dispatch("EXE-01", "day plan", []string{"day", "digests"})
	full, err := d.WriteArtifact(dp, "day/2026-09-23.md", "day-plan", "1. top priority", at)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(b), "Date: 2026-09-23\nAuthor: EXE-01\nType: day-plan\n\n") {
		t.Errorf("header wrong: %q", string(b))
	}
	if !strings.HasSuffix(string(b), "1. top priority\n") {
		t.Errorf("body wrong: %q", string(b))
	}
	if _, err := d.WriteArtifact(dp, "mbr/2026-09.md", "mbr", "x", at); err == nil {
		t.Error("write outside granted scope: want error")
	}
	l0, _ := d.Dispatch("RSK-01", "counsel", []string{"day"})
	if _, err := d.WriteArtifact(l0, "day/x.md", "day-plan", "x", at); err == nil {
		t.Error("L0 propose-only write: want error")
	}
}
