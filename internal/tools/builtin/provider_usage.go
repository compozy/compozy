package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

func providerUsageDescriptors() []toolspkg.Descriptor {
	return []toolspkg.Descriptor{
		nativeDescriptor(toolspkg.ToolIDProviderUsageList, "provider_usage_list", "Provider Usage List",
			"Read cached provider usage for the caller's profile (experimental). Never contacts providers.",
			`{"type":"object","properties":{"provider_id":{"type":"string"}},"additionalProperties":false}`,
			toolspkg.RiskRead, true, false, false, []toolspkg.ToolsetID{toolspkg.ToolsetIDProviderUsage},
			[]string{providerModelsProvidersKey, "usage"}, []string{"provider usage", "quota", "credits"}),
		nativeDescriptor(
			toolspkg.ToolIDProviderUsageRefresh,
			"provider_usage_refresh",
			"Provider Usage Refresh",
			"Refresh provider usage for the caller's profile with background intent (experimental). "+
				"Never reads Keychain; waits up to 15 seconds.",
			`{"type":"object","properties":{"provider_ids":{"type":"array","items":{"type":"string"}},"max_age_seconds":{"type":"integer","minimum":0}},"additionalProperties":false}`,
			toolspkg.RiskMutating,
			false,
			false,
			true,
			[]toolspkg.ToolsetID{toolspkg.ToolsetIDProviderUsage},
			[]string{providerModelsProvidersKey, "usage", "refresh"},
			[]string{"refresh usage", "refresh quota"},
		),
	}
}
