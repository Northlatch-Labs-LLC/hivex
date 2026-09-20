package config

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
)

// The failure this guards is SILENT. A variable that stops being read does not
// raise an error, it falls through to a default: someone's CI stays green while
// running against the wrong home directory, endpoint, or model. So the fallback
// has to work, and when it fires it has to say so unmissably.
//
// The shipped configuration has NO legacy prefixes today — the list is empty
// and every setting lives under exactly one name. The mechanism exists for the
// next rename, so these tests install a synthetic legacy generation
// ("HIVEXOLD_") for their own duration and restore the empty list after.

// withSyntheticLegacyPrefix swaps in a synthetic legacy prefix generation and
// restores the shipped (empty) list on cleanup.
func withSyntheticLegacyPrefix(t *testing.T, prefix string) {
	t.Helper()
	prev := LegacyEnvPrefixes
	LegacyEnvPrefixes = []string{prefix}
	t.Cleanup(func() { LegacyEnvPrefixes = prev })
}

func TestLookupEnvPrefersCurrentPrefix(t *testing.T) {
	withSyntheticLegacyPrefix(t, "HIVEXOLD_")
	t.Setenv(EnvPrefix+"RENAME_PROBE", "current")
	t.Setenv("HIVEXOLD_"+"RENAME_PROBE", "legacy")

	got, ok := LookupEnv(EnvPrefix + "RENAME_PROBE")
	if !ok || got != "current" {
		t.Fatalf("current prefix must win: got %q ok=%v", got, ok)
	}
}

// The migration case: the operator has only ever set the OLD spelling. Their
// setting must still be honoured.
func TestLookupEnvFallsBackToLegacyPrefix(t *testing.T) {
	withSyntheticLegacyPrefix(t, "HIVEXOLD_")
	os.Unsetenv(EnvPrefix + "RENAME_PROBE_LEGACY")
	t.Setenv("HIVEXOLD_"+"RENAME_PROBE_LEGACY", "from-legacy")

	got, ok := LookupEnv(EnvPrefix + "RENAME_PROBE_LEGACY")
	if !ok || got != "from-legacy" {
		t.Fatalf("legacy spelling must still be read: got %q ok=%v", got, ok)
	}
}

// A caller may hand in the bare suffix or any generation's spelling; all three
// resolve the same setting. That is what lets the later mechanical sweep flip
// the literal at a call site without changing behaviour.
func TestLookupEnvAcceptsAnyGenerationOfTheName(t *testing.T) {
	withSyntheticLegacyPrefix(t, "HIVEXOLD_")
	t.Setenv(EnvPrefix+"RENAME_PROBE_SHAPE", "v")

	for _, spelling := range []string{
		"RENAME_PROBE_SHAPE",
		EnvPrefix + "RENAME_PROBE_SHAPE",
		"HIVEXOLD_" + "RENAME_PROBE_SHAPE",
	} {
		if got, ok := LookupEnv(spelling); !ok || got != "v" {
			t.Errorf("%s did not resolve: got %q ok=%v", spelling, got, ok)
		}
	}
}

func TestLookupEnvUnsetIsNotAnError(t *testing.T) {
	withSyntheticLegacyPrefix(t, "HIVEXOLD_")
	os.Unsetenv(EnvPrefix + "RENAME_PROBE_ABSENT")
	os.Unsetenv("HIVEXOLD_" + "RENAME_PROBE_ABSENT")

	if got, ok := LookupEnv(EnvPrefix + "RENAME_PROBE_ABSENT"); ok || got != "" {
		t.Fatalf("absent must report unset, got %q ok=%v", got, ok)
	}
}

// The warning is the entire mitigation for a silent failure, so it must name
// BOTH spellings and the release that stops honouring the old one. Naming only
// the old one leaves the reader guessing the new spelling.
func TestLegacyEnvWarningNamesBothNamesAndARemovalVersion(t *testing.T) {
	withSyntheticLegacyPrefix(t, "HIVEXOLD_")
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)

	warnedEnvFallback.Delete("HIVEXOLD_" + "RENAME_PROBE_WARN")
	os.Unsetenv(EnvPrefix + "RENAME_PROBE_WARN")
	t.Setenv("HIVEXOLD_"+"RENAME_PROBE_WARN", "x")

	if _, ok := LookupEnv(EnvPrefix + "RENAME_PROBE_WARN"); !ok {
		t.Fatal("expected the legacy value to be read")
	}

	out := buf.String()
	for _, want := range []string{
		"HIVEXOLD_" + "RENAME_PROBE_WARN", // the name they set
		EnvPrefix + "RENAME_PROBE_WARN",   // the name they should set
		EnvFallbackRemovedIn,              // when the old one stops working
	} {
		if !strings.Contains(out, want) {
			t.Errorf("deprecation warning must mention %q; got: %s", want, out)
		}
	}
}

// A resolver on a hot path must not turn the log into a stream, or the warning
// becomes noise people filter out.
func TestLegacyEnvWarningFiresOncePerVariable(t *testing.T) {
	withSyntheticLegacyPrefix(t, "HIVEXOLD_")
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)

	warnedEnvFallback.Delete("HIVEXOLD_" + "RENAME_PROBE_ONCE")
	os.Unsetenv(EnvPrefix + "RENAME_PROBE_ONCE")
	t.Setenv("HIVEXOLD_"+"RENAME_PROBE_ONCE", "x")

	for i := 0; i < 5; i++ {
		LookupEnv(EnvPrefix + "RENAME_PROBE_ONCE")
	}

	if n := strings.Count(buf.String(), "RENAME_PROBE_ONCE is set"); n != 1 {
		t.Fatalf("expected exactly one warning across five reads, got %d:\n%s", n, buf.String())
	}
}

// internal/runtimebin injects its own getenv for testing; the fallback has to
// work through that indirection too, not only through os.LookupEnv.
func TestLookupEnvFuncHonoursAnInjectedGetenv(t *testing.T) {
	withSyntheticLegacyPrefix(t, "HIVEXOLD_")
	env := map[string]string{"HIVEXOLD_" + "RENAME_PROBE_INJECT": "injected"}
	getenv := func(k string) string { return env[k] }

	got, ok := LookupEnvFunc(getenv, EnvPrefix+"RENAME_PROBE_INJECT")
	if !ok || got != "injected" {
		t.Fatalf("injected legacy lookup failed: got %q ok=%v", got, ok)
	}
}
