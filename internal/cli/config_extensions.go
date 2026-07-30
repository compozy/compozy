package cli

import "strings"

const (
	extensionsTrustAllowUnverifiedPath = "extensions.trust.allow_unverified"
	extensionsGitHubEnabledPath        = "extensions.sources.github.enabled"
	extensionsGitHubBaseURLPath        = "extensions.sources.github.base_url"
)

func extensionConfigSetPathKinds() map[string]configSetValueKind {
	return map[string]configSetValueKind{
		extensionsTrustAllowUnverifiedPath:                        configSetBool,
		extensionsGitHubEnabledPath:                               configSetBool,
		extensionsGitHubBaseURLPath:                               configSetString,
		"extensions.sources.git.enabled":                          configSetBool,
		"extensions.dev.watch_interval":                           configSetDuration,
		"extensions.resources.allowed_kinds":                      configSetStringSlice,
		"extensions.resources.max_scope":                          configSetString,
		"extensions.resources.snapshot_rate_limit.requests":       configSetInt,
		"extensions.resources.snapshot_rate_limit.window":         configSetDuration,
		"extensions.resources.snapshot_rate_limit.queue":          configSetInt,
		"extensions.resources.operator_write_rate_limit.requests": configSetInt,
		"extensions.resources.operator_write_rate_limit.window":   configSetDuration,
		"extensions.resources.operator_write_rate_limit.queue":    configSetInt,
	}
}

func removedExtensionConfigSetReplacement(path string) (string, bool) {
	switch path {
	case "extensions.marketplace.allow_unverified":
		return extensionsTrustAllowUnverifiedPath, true
	case "extensions.marketplace.base_url":
		return extensionsGitHubBaseURLPath, true
	case "extensions.marketplace.registry":
		return extensionsGitHubEnabledPath, true
	default:
		if path == "extensions.marketplace" || strings.HasPrefix(path, "extensions.marketplace.") {
			return "extensions.trust or extensions.sources", true
		}
		return "", false
	}
}
