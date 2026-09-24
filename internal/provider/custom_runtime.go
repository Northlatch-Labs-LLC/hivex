package provider

import (
	"github.com/Northlatch-Labs-LLC/hivex/internal/config"
)

// RegisterCustomProviders registers one Entry per Settings-managed custom
// provider so its ID is a bindable kind at dispatch time. Base URL, model,
// and API key resolve at stream time from config (openai_compat.go), so a
// provider added or edited in Settings needs no restart — only a kind that
// appears AFTER startup needs this call re-run (the office calls it on
// provider-add).
func RegisterCustomProviders() {
	cfg, err := config.Load()
	if err != nil {
		return
	}
	for _, cp := range cfg.CustomProviders {
		if !cp.Enabled {
			continue
		}
		kind := cp.ID
		Replace(&Entry{
			Kind:      kind,
			StreamFn:  NewOpenAICompatStreamFn(kind, "", ""),
			Transport: TransportOpenAICompat,
			Capabilities: Capabilities{
				// Headless HTTP runtime, same shape as hiveapi.
				PaneEligible:    false,
				SupportsOneShot: false,
			},
		})
	}
}
