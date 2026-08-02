package core

import (
	"github.com/compozy/compozy/internal/api/contract"
	authproviders "github.com/compozy/compozy/internal/providers"
)

func providerLoginDescriptorPayload(
	value authproviders.ProviderLoginDescriptor,
) contract.ProviderLoginDescriptorPayload {
	return contract.ProviderLoginDescriptorPayload{
		Configured:        value.Configured,
		Source:            contract.ProviderLoginSource(value.Source),
		Executable:        value.Executable,
		Presence:          contract.ProviderLoginPresence(value.Presence),
		RecommendedAction: contract.ProviderRecommendedAction(value.RecommendedAction),
	}
}
