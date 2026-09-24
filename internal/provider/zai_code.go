package provider

// zai_code.go — Z.ai Code: the GLM Coding Plan driven through the Claude
// Code CLI engine. Z.ai's coding plan speaks the Anthropic protocol at
// https://api.z.ai/api/anthropic, so the claude-code runner executes the
// turns while inference (and billing) happen on the z.ai plan. The turn
// dispatcher tags the kind onto the turn context and the claude env builder
// routes ANTHROPIC_BASE_URL/ANTHROPIC_AUTH_TOKEN accordingly — the engine
// is shared, nothing forked.
//
// Activation is optional and probe-gated like every CLI runtime: the office
// shows it connected only when the claude CLI is installed AND a Z.ai key
// is configured (Settings → Credentials → API Keys).
func init() {
	Register(&Entry{
		Kind:       KindZAICode,
		StreamFn:   CreateClaudeCodeStreamFn,
		OneShot:    RunClaudeOneShot,
		OneShotCtx: RunClaudeOneShotCtx,
		Capabilities: Capabilities{
			PaneEligible:    false,
			SupportsOneShot: true,
		},
	})
}
