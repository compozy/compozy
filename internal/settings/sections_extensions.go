package settings

import (
	"reflect"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func diffExtensionsSettings(current compozyconfig.ExtensionsConfig, desired compozyconfig.ExtensionsConfig) []string {
	var changed []string
	if current.Trust.AllowUnverified != desired.Trust.AllowUnverified {
		changed = append(changed, "extensions.trust.allow_unverified")
	}
	if current.Sources.GitHub.Enabled != desired.Sources.GitHub.Enabled {
		changed = append(changed, "extensions.sources.github.enabled")
	}
	if current.Sources.GitHub.BaseURL != desired.Sources.GitHub.BaseURL {
		changed = append(changed, "extensions.sources.github.base_url")
	}
	if current.Sources.Git.Enabled != desired.Sources.Git.Enabled {
		changed = append(changed, "extensions.sources.git.enabled")
	}
	if current.Dev.WatchInterval != desired.Dev.WatchInterval {
		changed = append(changed, "extensions.dev.watch_interval")
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

func applyExtensionsSettings(editor *compozyconfig.OverlayEditor, settings compozyconfig.ExtensionsConfig) error {
	updates := []struct {
		path  []string
		value any
	}{
		{
			path:  []string{sectionsExtensionsKey, sectionsTrustKey, "allow_unverified"},
			value: settings.Trust.AllowUnverified,
		},
		{
			path:  []string{sectionsExtensionsKey, sectionsSourcesKey, sectionsGitHubKey, sectionsEnabledKey},
			value: settings.Sources.GitHub.Enabled,
		},
		{
			path:  []string{sectionsExtensionsKey, sectionsSourcesKey, sectionsGitHubKey, "base_url"},
			value: settings.Sources.GitHub.BaseURL,
		},
		{
			path:  []string{sectionsExtensionsKey, sectionsSourcesKey, sectionsGitKey, "enabled"},
			value: settings.Sources.Git.Enabled,
		},
		{
			path:  []string{sectionsExtensionsKey, sectionsDevKey, "watch_interval"},
			value: settings.Dev.WatchInterval.String(),
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

func cloneExtensionsConfig(value compozyconfig.ExtensionsConfig) compozyconfig.ExtensionsConfig {
	return compozyconfig.ExtensionsConfig{
		Trust: compozyconfig.ExtensionsTrustConfig{
			AllowUnverified: value.Trust.AllowUnverified,
		},
		Sources: compozyconfig.ExtensionsSourcesConfig{
			GitHub: compozyconfig.ExtensionsGitHubSourceConfig{
				Enabled: value.Sources.GitHub.Enabled,
				BaseURL: value.Sources.GitHub.BaseURL,
			},
			Git: compozyconfig.ExtensionsGitSourceConfig{
				Enabled: value.Sources.Git.Enabled,
			},
		},
		Dev: compozyconfig.ExtensionsDevConfig{WatchInterval: value.Dev.WatchInterval},
		Resources: compozyconfig.ExtensionsResourcesConfig{
			AllowedKinds: cloneAllowedKinds(value.Resources.AllowedKinds),
			MaxScope:     value.Resources.MaxScope,
			SnapshotRateLimit: compozyconfig.ExtensionsResourceRateLimitConfig{
				Requests: value.Resources.SnapshotRateLimit.Requests,
				Window:   value.Resources.SnapshotRateLimit.Window,
				Queue:    value.Resources.SnapshotRateLimit.Queue,
			},
			OperatorWriteRateLimit: compozyconfig.ExtensionsResourceRateLimitConfig{
				Requests: value.Resources.OperatorWriteRateLimit.Requests,
				Window:   value.Resources.OperatorWriteRateLimit.Window,
				Queue:    value.Resources.OperatorWriteRateLimit.Queue,
			},
		},
	}
}
