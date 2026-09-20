// Package workspace wipes hivebot's on-disk state for two distinct blast radii:
//
//   - Reset: narrow. Clears broker runtime state so a stuck office can restart
//     clean. Preserves task worktrees, team, company, office history, and
//     workflows. Equivalent to what `hivex shred` did before the verb swap.
//
//   - Shred: full. Everything Reset does, plus deletes the team roster, company
//     identity, the office's task receipts, saved workflows, logs, sessions,
//     provider state, calendar, and local markdown memory. The next load shows
//     the onboarding wizard.
//
// Preserved in both cases: office.pid, task-worktrees/, openclaw/, config.json.
// In-flight work remains on disk so branches and local changes inside task
// worktrees survive, and credentials/preferences stay available for the next
// launch.
//
// For managing the set of workspaces (list, create, switch, pause, resume),
// see internal/workspaces (plural).
package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Northlatch-Labs-LLC/hivex/internal/company"
	"github.com/Northlatch-Labs-LLC/hivex/internal/config"
	"github.com/Northlatch-Labs-LLC/hivex/internal/onboarding"
)

// Result reports which paths the operation actually removed and collects any
// non-fatal errors. A path is "removed" only if it existed before the call.
type Result struct {
	Removed []string `json:"removed"`
	Errors  []string `json:"errors,omitempty"`
}

// ClearRuntime performs a narrow reset on the user-default workspace tree at
// config.RuntimeHomeDir()/.hivex. It deletes the broker state file and the
// last-good snapshot. Safe to call when no broker is running. The live broker
// may keep using the same office.pid and team directory; callers that want to
// clear in-memory runtime should do so separately.
//
// Equivalent to ResetAt(<default hivex home>). Both honor
// HIVEX_BROKER_STATE_PATH when set.
func ClearRuntime() (Result, error) {
	home, err := hivexHome()
	if err != nil {
		return Result{}, err
	}
	return ResetAt(home)
}

// Shred performs a full workspace wipe on the user-default workspace tree at
// config.RuntimeHomeDir()/.hivex. Runs the same wipe set as ShredAt and, for
// parity with the previous implementation, additionally removes any env-
// overridden onboarding state path (onboarding.StatePath) and company manifest
// path (company.ManifestPath, which may resolve to HIVEX_COMPANY_FILE,
// the legacy company-file env var, or a CWD-local hivex.company.json) when those resolve
// outside the hivexHome tree.
func Shred() (Result, error) {
	home, err := hivexHome()
	if err != nil {
		return Result{}, err
	}
	res, err := ShredAt(home)
	if err != nil {
		return res, err
	}
	// Cover env-overridden locations that ShredAt cannot see because it is
	// scoped to a hivexHome tree. removeIfPresent is a no-op when the path
	// has already been removed via ShredAt or does not exist.
	res.removeIfPresent(onboarding.StatePath())
	res.removeIfPresent(company.ManifestPath())
	return res, nil
}

// ResetAt performs a narrow reset on an explicit workspace tree rooted at
// hivexHome (the .hivex subdirectory of a workspace's runtime home). It
// deletes the broker state file and the last-good snapshot. Honors
// HIVEX_BROKER_STATE_PATH when set, in which case the override path replaces
// the in-tree default.
//
// ClearRuntime delegates here using config.RuntimeHomeDir()/.hivex as the
// canonical wipe set.
func ResetAt(hivexHome string) (Result, error) {
	var res Result
	statePath := filepath.Join(hivexHome, "team", "broker-state.json")
	snapshotPath := statePath + ".last-good"
	if p := strings.TrimSpace(os.Getenv("HIVEX_BROKER_STATE_PATH")); p != "" {
		statePath = p
		snapshotPath = p + ".last-good"
	}
	res.removeIfPresent(statePath)
	res.removeIfPresent(snapshotPath)
	return res, nil
}

// ShredAt performs a full workspace wipe on an explicit workspace tree rooted
// at hivexHome (the .hivex subdirectory of a workspace's runtime home). It
// runs ResetAt first, then removes onboarded.json, company.json, and the
// directories holding office task receipts, workflows, logs, sessions,
// provider session state, codex-headless cache, wiki, wiki.bak, and the
// calendar JSON file.
//
// Shred delegates here using config.RuntimeHomeDir()/.hivex and additionally
// covers env-overridden onboarding/company paths that may resolve outside the
// hivexHome tree.
func ShredAt(hivexHome string) (Result, error) {
	res, err := ResetAt(hivexHome)
	if err != nil {
		return res, err
	}
	res.removeIfPresent(filepath.Join(hivexHome, "onboarded.json"))
	res.removeIfPresent(filepath.Join(hivexHome, "company.json"))
	res.removeIfPresent(filepath.Join(hivexHome, "office"))
	res.removeIfPresent(filepath.Join(hivexHome, "workflows"))
	res.removeIfPresent(filepath.Join(hivexHome, "logs"))
	res.removeIfPresent(filepath.Join(hivexHome, "sessions"))
	res.removeIfPresent(filepath.Join(hivexHome, "providers"))
	res.removeIfPresent(filepath.Join(hivexHome, "codex-headless"))
	res.removeIfPresent(filepath.Join(hivexHome, "wiki"))
	res.removeIfPresent(filepath.Join(hivexHome, "wiki.bak"))
	res.removeIfPresent(filepath.Join(hivexHome, "calendar.json"))
	return res, nil
}

// hivexHome returns the absolute path to ~/.hivex, honoring HIVEX_RUNTIME_HOME
// so tests and sandboxed runs stay isolated from the real user directory.
func hivexHome() (string, error) {
	home := config.RuntimeHomeDir()
	if home == "" {
		return "", errors.New("workspace: could not resolve home directory")
	}
	return filepath.Join(home, ".hivex"), nil
}

func (r *Result) removeIfPresent(path string) {
	if path == "" {
		return
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("stat %s: %v", path, err))
		return
	}
	var rmErr error
	if info.IsDir() {
		rmErr = os.RemoveAll(path)
	} else {
		rmErr = os.Remove(path)
	}
	if rmErr != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("remove %s: %v", path, rmErr))
		return
	}
	r.Removed = append(r.Removed, path)
}
