package team

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// One table over the shared validator: each case names exactly one boundary,
// so a failure points at one rule in sandbox_html_policy.go (epicenter
// testing per docs/CODE-QUALITY.md).
func testSandboxPolicy() sandboxHTMLPolicy {
	return sandboxHTMLPolicy{
		label: "test",
		blockedElement: func(tag string) (string, bool) {
			if tag == "form" {
				return "form is blocked by this policy", true
			}
			return "", false
		},
		newErr: func(format string, args ...any) error {
			return errors.New(fmt.Sprintf(format, args...))
		},
	}
}

func TestValidateSandboxHTML(t *testing.T) {
	p := testSandboxPolicy()
	cases := []struct {
		name    string
		raw     string
		wantErr string // empty means accepted
	}{
		{"accepts plain markup", "<div><p>hello</p></div>", ""},
		{"accepts fragment href", `<a href="#section">jump</a>`, ""},
		{"accepts data url", `<img src="data:image/png;base64,AAAA">`, ""},
		{"accepts system-font style attr", `<div style="font-family: Georgia, serif;">x</div>`, ""},
		{"accepts data url in style tag", "<style>div { background: url(\"data:image/png;base64,AAAA\"); }</style>", ""},
		{"rejects blocked element", "<form></form>", "html element <form> is not allowed"},
		{"rejects on-handler", `<div onclick="evil()">x</div>`, "html attribute onclick on <div> is not allowed"},
		{"rejects srcset", `<img srcset="http://x 1x">`, "html attribute srcset on <img> is not allowed"},
		{"rejects external script src", `<script src="http://x"></script>`, "external script src is not allowed"},
		{"rejects meta refresh", `<meta http-equiv="refresh" content="5">`, "meta refresh is not allowed"},
		{"rejects external href", `<a href="http://x">x</a>`, "must use a data/blob URL or fragment reference"},
		{"rejects css import", `<div style="@import 'x'">x</div>`, "css @import is not allowed"},
		{"rejects css expression", "<style>div { color: expression(alert(1)); }</style>", "css expression() is not allowed"},
		{"rejects malformed css url", "<style>div { background: url(; }</style>", "css url() is malformed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSandboxHTML(tc.raw, p)
			if tc.wantErr == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}
