package team

// broker_box_signin_urls_test.go pins the env-driven Box URL resolution:
// HIVEX_BOX_CLI_DOWNLOAD_URL, HIVEX_BOX_INSTALL_URL, and HIVEX_BOX_BASE_URL
// win at call time; the compile-time ascii.dev defaults apply only when no
// env var is set.

import (
	"strings"
	"testing"

	"github.com/Northlatch-Labs-LLC/hivex/internal/computer/box"
)

func TestBoxURLsDefaultWithoutEnv(t *testing.T) {
	t.Setenv("HIVEX_BOX_CLI_DOWNLOAD_URL", "")
	t.Setenv("HIVEX_BOX_INSTALL_URL", "")
	t.Setenv("HIVEX_BOX_BASE_URL", "")
	if got := boxCLIDownloadURL(); got != boxCLIDownloadDefault {
		t.Errorf("boxCLIDownloadURL() = %q, want default %q", got, boxCLIDownloadDefault)
	}
	if got := boxInstallURL(); got != boxInstallURLDefault {
		t.Errorf("boxInstallURL() = %q, want default %q", got, boxInstallURLDefault)
	}
	if got := boxBaseURL(); got != box.DefaultAPI {
		t.Errorf("boxBaseURL() = %q, want default %q", got, box.DefaultAPI)
	}
	cmd := boxInstallCommandText()
	if !strings.Contains(cmd, boxInstallURLDefault) {
		t.Errorf("boxInstallCommandText() = %q, want it to embed %q", cmd, boxInstallURLDefault)
	}
}

func TestBoxURLEnvOverridesResolvedAtCallTime(t *testing.T) {
	t.Setenv("HIVEX_BOX_CLI_DOWNLOAD_URL", "https://gridframes.app/api/box/cli/download")
	t.Setenv("HIVEX_BOX_INSTALL_URL", "https://gridframes.app/api/box/install")
	t.Setenv("HIVEX_BOX_BASE_URL", "https://gridframes.app/api/box/v1")
	if got := boxCLIDownloadURL(); !strings.HasPrefix(got, "https://gridframes.app") {
		t.Errorf("boxCLIDownloadURL() = %q, want gridframes.app override", got)
	}
	if got := boxInstallURL(); !strings.HasPrefix(got, "https://gridframes.app") {
		t.Errorf("boxInstallURL() = %q, want gridframes.app override", got)
	}
	if got := boxBaseURL(); !strings.HasPrefix(got, "https://gridframes.app") {
		t.Errorf("boxBaseURL() = %q, want gridframes.app override", got)
	}
	cmd := boxInstallCommandText()
	if !strings.Contains(cmd, "https://gridframes.app/api/box/install") {
		t.Errorf("boxInstallCommandText() = %q, want the overridden install URL", cmd)
	}
	if strings.Contains(cmd, "ascii.dev") {
		t.Errorf("boxInstallCommandText() = %q, must not embed the competitor default", cmd)
	}
	// Call-time resolution: changing the env var between calls is honored.
	t.Setenv("HIVEX_BOX_INSTALL_URL", "https://example.test/install")
	if got := boxInstallURL(); got != "https://example.test/install" {
		t.Errorf("boxInstallURL() after env change = %q, want %q", got, "https://example.test/install")
	}
}
