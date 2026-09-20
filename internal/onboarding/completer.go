package onboarding

import (
	"context"

	"github.com/Northlatch-Labs-LLC/hivex/internal/provider"
)

type cliCompleter struct{}

func (c cliCompleter) Complete(ctx context.Context, prompt string) (string, error) {
	return provider.RunConfiguredOneShotCtx(ctx, "", prompt, "")
}
