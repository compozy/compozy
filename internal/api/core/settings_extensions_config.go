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
		Trust: contract.SettingsExtensionTrustPayload{
			AllowUnverified: value.Trust.AllowUnverified,
		},
		Sources: contract.SettingsExtensionSourcesPayload{
			GitHub: contract.SettingsExtensionGitHubSourcePayload{
				Enabled: value.Sources.GitHub.Enabled,
				BaseURL: strings.TrimSpace(value.Sources.GitHub.BaseURL),
			},
			Git: contract.SettingsExtensionGitSourcePayload{
				Enabled: value.Sources.Git.Enabled,
			},
		},
		Dev: contract.SettingsExtensionDevPayload{
			WatchInterval: value.Dev.WatchInterval.String(),
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
		Trust: compozyconfig.ExtensionsTrustConfig{
			AllowUnverified: payload.Trust.AllowUnverified,
		},
		Sources: compozyconfig.ExtensionsSourcesConfig{
			GitHub: compozyconfig.ExtensionsGitHubSourceConfig{
				Enabled: payload.Sources.GitHub.Enabled,
				BaseURL: strings.TrimSpace(payload.Sources.GitHub.BaseURL),
			},
			Git: compozyconfig.ExtensionsGitSourceConfig{
				Enabled: payload.Sources.Git.Enabled,
			},
		},
		Resources: compozyconfig.ExtensionsResourcesConfig{
			AllowedKinds:           allowedKinds,
			MaxScope:               payload.Resources.MaxScope,
			SnapshotRateLimit:      snapshotRateLimit,
			OperatorWriteRateLimit: operatorWriteRateLimit,
		},
	}
	value.Dev.WatchInterval, err = parseOptionalDuration(
		payload.Dev.WatchInterval,
		"hooks-extensions.config.dev.watch_interval",
	)
	if err != nil {
		return compozyconfig.ExtensionsConfig{}, err
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
