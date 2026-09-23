package marketplace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Northlatch-Labs-LLC/hivex/internal/config"
)

// wikiTeamDir returns <runtime home>/wiki/team — the install root.
func wikiTeamDir() (string, error) {
	home := config.RuntimeHomeDir()
	if home == "" {
		return "", fmt.Errorf("marketplace: runtime home not set")
	}
	return filepath.Join(home, "wiki", "team"), nil
}

// targetPath maps a catalog entry to its install path under the team wiki:
// skills → skills/<id>.md, experts → experts/<id>.md, plugins → plugins/<id>.json.
func targetPath(e Entry) (string, error) {
	root, err := wikiTeamDir()
	if err != nil {
		return "", err
	}
	switch e.Category {
	case CategorySkill:
		return filepath.Join(root, "skills", e.ID+".md"), nil
	case CategoryExpert:
		return filepath.Join(root, "experts", e.ID+".md"), nil
	case CategoryPlugin:
		return filepath.Join(root, "plugins", e.ID+".json"), nil
	default:
		return "", fmt.Errorf("marketplace: unknown category %q", e.Category)
	}
}

// Install materializes the catalog entry into the workspace. Overwrites
// an existing install of the same entry.
func Install(e Entry) error {
	if _, ok := FindEntry(e.ID); !ok {
		return fmt.Errorf("marketplace: unknown entry %q", e.ID)
	}
	path, err := targetPath(e)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(e.Payload), 0o644)
}

// Uninstall removes an installed entry. Removing an absent entry is a no-op.
func Uninstall(id string) error {
	e, ok := FindEntry(id)
	if !ok {
		return fmt.Errorf("marketplace: unknown entry %q", id)
	}
	path, err := targetPath(e)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Installed reports which catalog entries are currently installed.
func Installed() (map[string]bool, error) {
	out := map[string]bool{}
	for _, e := range Catalog() {
		path, err := targetPath(e)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(path); err == nil {
			out[e.ID] = true
		}
	}
	return out, nil
}

// ValidateEntryID guards broker input: a slug, nothing else.
func ValidateEntryID(id string) bool {
	id = strings.TrimSpace(id)
	return id != "" && !strings.ContainsAny(id, "/\\ \t")
}
