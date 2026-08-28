package core

import (
	"strings"

	"github.com/compozy/compozy/internal/acp"
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
	switch session.RuntimeStrategyForProvider(effectiveProvider) {
	case acp.RuntimeApplicationLaunchArg:
		return contract.ProviderRuntimeStrategyLaunchArg
	case acp.RuntimeApplicationProviderManaged:
		return contract.ProviderRuntimeStrategyProviderManaged
	case acp.RuntimeApplicationSessionConfig:
		return contract.ProviderRuntimeStrategySessionConfig
	default:
		return contract.ProviderRuntimeStrategySessionConfig
	}
}
