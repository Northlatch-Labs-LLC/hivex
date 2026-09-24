package provider

// zai.go — Z.ai as a first-class office runtime over the Anthropic
// Messages protocol (https://api.z.ai/api/anthropic — the same protocol
// the GLM Coding Plan serves to Claude Code CLIs). Turns on this kind flow
// through z.ai's own inference; metering and per-agent binding work like
// every other headless HTTP runtime.
//
// Contract:
//
//	HIVEX_ZAI_BASE_URL   (default https://api.z.ai/api/anthropic — the
//	                      coding plan's Anthropic-protocol endpoint)
//	HIVEX_ZAI_API_KEY    the z.ai API key (x-api-key + Bearer). Alternatively
//	                      Settings → Credentials → API Keys, stored in
//	                      config.json as "zai_api_key" (write-only over HTTP,
//	                      surfaced as the zai_key_set flag).
//	HIVEX_ZAI_MODEL      (default GLM-5.3 — the plan's current flagship;
//	                      overridable per binding or via config
//	                      provider_endpoints.zai.model)
const (
	defaultZaiBaseURL = "https://api.z.ai/api/anthropic"
	defaultZaiModel   = "GLM-5.3"
)

func init() {
	Register(&Entry{
		Kind:      KindZAI,
		StreamFn:  NewAnthropicMessagesStreamFn(KindZAI, defaultZaiBaseURL, defaultZaiModel),
		Transport: TransportAnthropic,
		Capabilities: Capabilities{
			// Headless HTTP runtime, same shape as hiveapi.
			PaneEligible:    false,
			SupportsOneShot: false,
		},
	})
}
