package session

import aghconfig "github.com/compozy/agh/internal/config"

func providerConfigFromResolvedAgent(resolved aghconfig.ResolvedAgent) aghconfig.ProviderConfig {
	return aghconfig.ProviderConfig{
		Command:         resolved.Command,
		DisplayName:     resolved.DisplayName,
		Harness:         resolved.Harness,
		RuntimeProvider: resolved.RuntimeProvider,
		Transport:       resolved.Transport,
		BaseURL:         resolved.BaseURL,
		AuthMode:        resolved.AuthMode,
		EnvPolicy:       resolved.EnvPolicy,
		HomePolicy:      resolved.HomePolicy,
		NoneSecurity:    resolved.NoneSecurity,
		AuthStatusCmd:   resolved.AuthStatusCmd,
		AuthLoginCmd:    resolved.AuthLoginCmd,
		Models: aghconfig.ProviderModelsConfig{
			Reasoning: resolved.Reasoning,
		},
		CredentialSlots: append(
			[]aghconfig.ProviderCredentialSlot(nil),
			resolved.CredentialSlots...),
	}
}
