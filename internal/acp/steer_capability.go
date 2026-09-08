package acp

import (
	"slices"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providers"
)

const steerExtensionMethod = "_session/steering"

func ResolveSteerCapability(
	provider string,
	announced []string,
	override config.SteerCapability,
) config.SteerCapability {
	if slices.Contains(announced, steerExtensionMethod) {
		return config.SteerCapabilityExtension
	}
	if override != "" && override.Validate("steer_capability") == nil {
		return override
	}
	return providers.SteerCapability(provider)
}

func steerCapabilityForStart(opts StartOpts) config.SteerCapability {
	var override config.SteerCapability
	if opts.ProviderConfig != nil {
		override = opts.ProviderConfig.SteerCapability
	}
	return ResolveSteerCapability(opts.ProviderName, nil, override)
}

func captureSteerCapability(meta map[string]any, fallback config.SteerCapability) config.SteerCapability {
	steering, ok := meta["steering"].(map[string]any)
	if ok && steering["supported"] == true {
		return config.SteerCapabilityExtension
	}
	return fallback
}
