package session

import compozyconfig "github.com/compozy/compozy/internal/config"

func providerConfigFromResolvedAgent(resolved compozyconfig.ResolvedAgent) compozyconfig.ProviderConfig {
	return compozyconfig.ProviderConfig{
		Command:             resolved.Command,
		CommandIsBuiltin:    resolved.CommandIsBuiltin,
		DisplayName:         resolved.DisplayName,
		Harness:             resolved.Harness,
		RuntimeProvider:     resolved.RuntimeProvider,
		Transport:           resolved.Transport,
		BaseURL:             resolved.BaseURL,
		AuthMode:            resolved.AuthMode,
		EnvPolicy:           resolved.EnvPolicy,
		HomePolicy:          resolved.HomePolicy,
		NoneSecurity:        resolved.NoneSecurity,
		AuthStatusCmd:       resolved.AuthStatusCmd,
		AuthLoginCmd:        resolved.AuthLoginCmd,
		AuthACPMethod:       resolved.AuthACPMethod,
		SessionMCPTransport: sessionMCPTransport(resolved.SessionMCPOverHTTP),
		Models: compozyconfig.ProviderModelsConfig{
			Reasoning: resolved.Reasoning,
		},
		CredentialSlots: append(
			[]compozyconfig.ProviderCredentialSlot(nil),
			resolved.CredentialSlots...),
	}
}

func sessionMCPTransport(overHTTP bool) compozyconfig.MCPServerTransport {
	if overHTTP {
		return compozyconfig.MCPServerTransportHTTP
	}
	return ""
}
