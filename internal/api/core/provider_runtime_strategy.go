package core

import (
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
)

func providerRuntimeStrategy(
	providerID string,
	runtimeProvider string,
) contract.ProviderRuntimeStrategy {
	effectiveProvider := strings.TrimSpace(runtimeProvider)
	if effectiveProvider == "" {
		effectiveProvider = strings.TrimSpace(providerID)
	}
	return contract.ProviderRuntimeStrategy(session.RuntimeStrategyForProvider(effectiveProvider))
}
