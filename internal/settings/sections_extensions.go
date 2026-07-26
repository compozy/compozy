package settings

import (
	"reflect"

	aghconfig "github.com/compozy/agh/internal/config"
)

func diffExtensionsSettings(current aghconfig.ExtensionsConfig, desired aghconfig.ExtensionsConfig) []string {
	var changed []string
	if current.Marketplace.Registry != desired.Marketplace.Registry {
		changed = append(changed, "extensions.marketplace.registry")
	}
	if current.Marketplace.BaseURL != desired.Marketplace.BaseURL {
		changed = append(changed, "extensions.marketplace.base_url")
	}
	if current.Marketplace.AllowUnverified != desired.Marketplace.AllowUnverified {
		changed = append(changed, "extensions.marketplace.allow_unverified")
	}
	if !reflect.DeepEqual(current.Resources.AllowedKinds, desired.Resources.AllowedKinds) {
		changed = append(changed, "extensions.resources.allowed_kinds")
	}
	if current.Resources.MaxScope != desired.Resources.MaxScope {
		changed = append(changed, "extensions.resources.max_scope")
	}
	if current.Resources.SnapshotRateLimit != desired.Resources.SnapshotRateLimit {
		changed = append(changed, "extensions.resources.snapshot_rate_limit")
	}
	if current.Resources.OperatorWriteRateLimit != desired.Resources.OperatorWriteRateLimit {
		changed = append(changed, "extensions.resources.operator_write_rate_limit")
	}
	return changed
}

func applyExtensionsSettings(editor *aghconfig.OverlayEditor, settings aghconfig.ExtensionsConfig) error {
	updates := []struct {
		path  []string
		value any
	}{
		{
			path:  []string{sectionsExtensionsKey, sectionsMarketplaceKey, "registry"},
			value: settings.Marketplace.Registry,
		},
		{
			path:  []string{sectionsExtensionsKey, sectionsMarketplaceKey, "base_url"},
			value: settings.Marketplace.BaseURL,
		},
		{
			path:  []string{sectionsExtensionsKey, sectionsMarketplaceKey, "allow_unverified"},
			value: settings.Marketplace.AllowUnverified,
		},
		{
			path:  []string{sectionsExtensionsKey, sectionsResourcesKey, "allowed_kinds"},
			value: resourceKindsToStrings(settings.Resources.AllowedKinds),
		},
		{
			path:  []string{sectionsExtensionsKey, sectionsResourcesKey, "max_scope"},
			value: string(settings.Resources.MaxScope),
		},
		{
			path:  []string{sectionsExtensionsKey, sectionsResourcesKey, sectionsSnapshotRateLimitKey, "requests"},
			value: settings.Resources.SnapshotRateLimit.Requests,
		},
		{
			path: []string{
				sectionsExtensionsKey,
				sectionsResourcesKey,
				sectionsSnapshotRateLimitKey,
				sectionsWindowKey,
			},
			value: settings.Resources.SnapshotRateLimit.Window.String(),
		},
		{
			path: []string{
				sectionsExtensionsKey,
				sectionsResourcesKey,
				sectionsSnapshotRateLimitKey,
				sectionsQueueKey,
			},
			value: settings.Resources.SnapshotRateLimit.Queue,
		},
		{
			path:  []string{sectionsExtensionsKey, sectionsResourcesKey, sectionsOperatorWriteRateLimitKey, "requests"},
			value: settings.Resources.OperatorWriteRateLimit.Requests,
		},
		{
			path: []string{
				sectionsExtensionsKey,
				sectionsResourcesKey,
				sectionsOperatorWriteRateLimitKey,
				sectionsWindowKey,
			},
			value: settings.Resources.OperatorWriteRateLimit.Window.String(),
		},
		{
			path: []string{
				sectionsExtensionsKey,
				sectionsResourcesKey,
				sectionsOperatorWriteRateLimitKey,
				sectionsQueueKey,
			},
			value: settings.Resources.OperatorWriteRateLimit.Queue,
		},
	}
	return applyValueUpdates(editor, updates)
}

func cloneExtensionsConfig(value aghconfig.ExtensionsConfig) aghconfig.ExtensionsConfig {
	return aghconfig.ExtensionsConfig{
		Marketplace: aghconfig.ExtensionsMarketplaceConfig{
			Registry:        value.Marketplace.Registry,
			BaseURL:         value.Marketplace.BaseURL,
			AllowUnverified: value.Marketplace.AllowUnverified,
		},
		Resources: aghconfig.ExtensionsResourcesConfig{
			AllowedKinds: cloneAllowedKinds(value.Resources.AllowedKinds),
			MaxScope:     value.Resources.MaxScope,
			SnapshotRateLimit: aghconfig.ExtensionsResourceRateLimitConfig{
				Requests: value.Resources.SnapshotRateLimit.Requests,
				Window:   value.Resources.SnapshotRateLimit.Window,
				Queue:    value.Resources.SnapshotRateLimit.Queue,
			},
			OperatorWriteRateLimit: aghconfig.ExtensionsResourceRateLimitConfig{
				Requests: value.Resources.OperatorWriteRateLimit.Requests,
				Window:   value.Resources.OperatorWriteRateLimit.Window,
				Queue:    value.Resources.OperatorWriteRateLimit.Queue,
			},
		},
	}
}
