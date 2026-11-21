package config

import (
	"path/filepath"
	"strings"
)

// agyNativeExecutableNames lists the basenames Google ships the Antigravity CLI under: the
// installer's `agy` and the release archive's `antigravity` binary.
var agyNativeExecutableNames = map[string]struct{}{
	providerAgyKey: {},
	"antigravity":  {},
}

// RequiresAdvertisedACPMode keeps authored AGY modes from being skipped as derived defaults.
func RequiresAdvertisedACPMode(provider string) bool {
	return strings.EqualFold(strings.TrimSpace(provider), providerAgyKey)
}

// IsNativeAgyACPCommand identifies the native CLI without conflating authored standalone or wrapper routes.
func IsNativeAgyACPCommand(command string) bool {
	parsed, err := ParseLaunchCommand(strings.TrimSpace(command))
	if err != nil {
		return false
	}
	name := strings.ToLower(filepath.Base(strings.ReplaceAll(parsed.Executable, `\`, "/")))
	for _, suffix := range []string{".exe", ".cmd", ".bat"} {
		name = strings.TrimSuffix(name, suffix)
	}
	if _, ok := agyNativeExecutableNames[name]; !ok {
		return false
	}
	for _, arg := range parsed.Args {
		trimmed := strings.TrimSpace(arg)
		if strings.HasPrefix(trimmed, "-") {
			continue
		}
		return strings.EqualFold(trimmed, "acp")
	}
	return false
}
