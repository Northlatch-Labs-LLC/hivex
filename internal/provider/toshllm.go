package provider

// toshllm.go — the ToshLLM engine as the local runtime: a Metal-backed
// (GPU) inference server on :8080 speaking OpenAI-compatible
// /v1/chat/completions. Replaces ollama as the local default on this
// install — ollama burns CPU without GPU acceleration and starves the
// machine when other work is in flight; ToshLLM keeps inference on the
// graphics card.
//
// Contract:
//
//	HIVEX_TOSHLLM_BASE_URL  (default http://127.0.0.1:8080/v1)
//	HIVEX_TOSHLLM_MODEL     no default: the engine's loaded model is
//	                        resolved automatically at request time (its
//	                        /models lists full-path model ids).
func init() {
	Register(&Entry{
		Kind:      KindToshllm,
		StreamFn:  NewOpenAICompatStreamFn(KindToshllm, defaultToshllmBaseURL, ""),
		Transport: TransportOpenAICompat,
		Capabilities: Capabilities{
			PaneEligible:    false,
			SupportsOneShot: false,
		},
	})
}

const defaultToshllmBaseURL = "http://127.0.0.1:8080/v1"
