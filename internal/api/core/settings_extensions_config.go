package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/resources"
)

func settingsExtensionsConfigPayload(value compozyconfig.ExtensionsConfig) contract.SettingsExtensionsConfigPayload {
	return contract.SettingsExtensionsConfigPayload{
		Marketplace: contract.SettingsExtensionMarketplacePayload{
			Registry:        strings.TrimSpace(value.Marketplace.Registry),
			BaseURL:         strings.TrimSpace(value.Marketplace.BaseURL),
			AllowUnverified: value.Marketplace.AllowUnverified,
		},
		Resources: contract.SettingsExtensionResourcesPayload{
			AllowedKinds:           resourceKindsToStrings(value.Resources.AllowedKinds),
			MaxScope:               value.Resources.MaxScope,
			SnapshotRateLimit:      settingsExtensionRateLimitPayload(value.Resources.SnapshotRateLimit),
			OperatorWriteRateLimit: settingsExtensionRateLimitPayload(value.Resources.OperatorWriteRateLimit),
		},
	}
}

func settingsExtensionRateLimitPayload(
	value compozyconfig.ExtensionsResourceRateLimitConfig,
) contract.SettingsExtensionRateLimitPayload {
	return contract.SettingsExtensionRateLimitPayload{
		Requests: value.Requests,
		Window:   value.Window.String(),
		Queue:    value.Queue,
	}
}

func extensionsConfigFromPayload(
	payload contract.SettingsExtensionsConfigPayload,
) (compozyconfig.ExtensionsConfig, error) {
	snapshotRateLimit, err := extensionRateLimitConfigFromPayload(
		payload.Resources.SnapshotRateLimit,
		"hooks-extensions.config.resources.snapshot_rate_limit",
	)
	if err != nil {
		return compozyconfig.ExtensionsConfig{}, err
	}
	operatorWriteRateLimit, err := extensionRateLimitConfigFromPayload(
		payload.Resources.OperatorWriteRateLimit,
		"hooks-extensions.config.resources.operator_write_rate_limit",
	)
	if err != nil {
		return compozyconfig.ExtensionsConfig{}, err
	}

	var allowedKinds []resources.ResourceKind
	for _, value := range payload.Resources.AllowedKinds {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			allowedKinds = append(allowedKinds, resources.ResourceKind(trimmed))
		}
	}

	value := compozyconfig.ExtensionsConfig{
		Marketplace: compozyconfig.ExtensionsMarketplaceConfig{
			Registry:        strings.TrimSpace(payload.Marketplace.Registry),
			BaseURL:         strings.TrimSpace(payload.Marketplace.BaseURL),
			AllowUnverified: payload.Marketplace.AllowUnverified,
		},
		Resources: compozyconfig.ExtensionsResourcesConfig{
			AllowedKinds:           allowedKinds,
			MaxScope:               payload.Resources.MaxScope,
			SnapshotRateLimit:      snapshotRateLimit,
			OperatorWriteRateLimit: operatorWriteRateLimit,
		},
	}
	if err := value.Validate(); err != nil {
		return compozyconfig.ExtensionsConfig{}, NewSettingsValidationError(err)
	}
	return value, nil
}

func extensionRateLimitConfigFromPayload(
	payload contract.SettingsExtensionRateLimitPayload,
	path string,
) (compozyconfig.ExtensionsResourceRateLimitConfig, error) {
	window, err := time.ParseDuration(strings.TrimSpace(payload.Window))
	if err != nil && strings.TrimSpace(payload.Window) != "" {
		return compozyconfig.ExtensionsResourceRateLimitConfig{}, NewSettingsValidationError(
			fmt.Errorf("%s.window: %w", path, err),
		)
	}
	return compozyconfig.ExtensionsResourceRateLimitConfig{
		Requests: payload.Requests,
		Window:   window,
		Queue:    payload.Queue,
	}, nil
}

func resourceKindsToStrings(values []resources.ResourceKind) []string {
	if len(values) == 0 {
		return nil
	}
	payloads := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(string(value)); trimmed != "" {
			payloads = append(payloads, trimmed)
		}
	}
	return payloads
}
