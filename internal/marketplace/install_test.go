package marketplace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallUninstallRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HIVEX_RUNTIME_HOME", dir)

	entry, ok := FindEntry("code-review")
	if !ok {
		t.Fatal("code-review not in catalog")
	}
	if err := Install(entry); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "wiki", "team", "skills", "code-review.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("installed skill missing: %v", err)
	}

	inst, err := Installed()
	if err != nil {
		t.Fatal(err)
	}
	if !inst["code-review"] {
		t.Fatalf("Installed() did not report code-review: %+v", inst)
	}

	if err := Uninstall("code-review"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("skill still present after uninstall: %v", err)
	}
	inst, _ = Installed()
	if inst["code-review"] {
		t.Fatal("Installed() still reports code-review after uninstall")
	}
}

func TestInstallUnknownEntry(t *testing.T) {
	t.Setenv("HIVEX_RUNTIME_HOME", t.TempDir())
	if err := Install(Entry{ID: "nope", Category: CategorySkill}); err == nil {
		t.Fatal("installing an unknown entry must fail")
	}
	if !ValidateEntryID("code-review") || ValidateEntryID("../evil") {
		t.Fatal("ValidateEntryID boundary broken")
	}
}
