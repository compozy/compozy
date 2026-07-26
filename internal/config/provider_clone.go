package config

// CloneProviderConfig returns a deep copy safe for ownership boundaries and caches.
func CloneProviderConfig(provider ProviderConfig) ProviderConfig {
	return cloneProvider(provider)
}

// CloneProviderConfigs returns a deep copy of a provider registry.
func CloneProviderConfigs(providers map[string]ProviderConfig) map[string]ProviderConfig {
	return cloneProviders(providers)
}
