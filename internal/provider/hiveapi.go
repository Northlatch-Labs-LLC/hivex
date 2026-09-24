package provider

// hiveapi.go — the Hivex Gateway as a first-class office runtime: the
// product's own OpenAI-compatible inference distributor (the same
// deployment the customer portal sells). Turns on this kind flow through
// the HiveAPI Gateway's /v1 — chat completions, and the same auth the
// harness uses for metering (one of the account's gateway keys).
//
// Contract:
//
//	HIVEX_HIVEAPI_BASE_URL   (default http://localhost:18080/v1 — the
//	                          gateway's standard host port)
//	HIVEX_HIVEAPI_API_KEY    one of the account's gateway keys (Bearer)
//	HIVEX_HIVEAPI_MODEL      REQUIRED — the model id from the gateway's
//	                          catalog (GET /v1/models with the same key).
//	                          No default: the catalog depends on the
//	                          operator's upstream providers. Alternatively
//	                          ~/.hivex/config.json:
//	                          "provider_endpoints": { "hiveapi": { "model": "…" } }
//
// The Monthly tier is provider-locked to this kind (the portal entitlement's
// provider_locked flag) — the harness enforces the pin at turn dispatch.
const (
	defaultHiveAPIBaseURL = "http://localhost:18080/v1"
	defaultHiveAPIModel   = ""
)

func init() {
	Register(&Entry{
		Kind:         KindHiveAPI,
		StreamFn:     NewOpenAICompatStreamFn(KindHiveAPI, defaultHiveAPIBaseURL, defaultHiveAPIModel),
		OpenAICompat: true,
		Capabilities: Capabilities{
			// Headless HTTP runtime: no interactive pane.
			PaneEligible:    false,
			SupportsOneShot: false,
		},
	})
}
