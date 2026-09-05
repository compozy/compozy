package providers

import "github.com/compozy/compozy/internal/config"

// Only binary-proven capabilities belong in this table; announcements take precedence at initialization.
var steerCapabilities = map[string]config.SteerCapability{
	"claude": config.SteerCapabilityNone,
	"codex":  config.SteerCapabilityNone,
	"cursor": config.SteerCapabilityNone,
}

func SteerCapability(provider string) config.SteerCapability {
	if capability, ok := steerCapabilities[config.CanonicalProviderName(provider)]; ok {
		return capability
	}
	return config.SteerCapabilityNone
}
