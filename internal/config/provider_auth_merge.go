package config

func applyProviderAuthModeOverride(merged *ProviderConfig, override ProviderConfig) {
	if override.AuthMode == "" {
		return
	}

	merged.AuthMode = override.AuthMode
	switch override.AuthMode {
	case ProviderAuthModeNone, ProviderAuthModeBoundSecret:
		merged.AuthStatusCmd = ""
		merged.AuthLoginCmd = ""
		merged.AuthACPMethod = ""
		merged.CredentialSlots = nil
	case ProviderAuthModeNativeCLI:
		merged.CredentialSlots = nil
	}
}
