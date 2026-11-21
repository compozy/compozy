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

// agyValueFlags lists Antigravity CLI global flags that take their value as the next argument,
// so that value is never mistaken for the subcommand.
var agyValueFlags = map[string]struct{}{
	"--gemini-dir":   {},
	"--app-data-dir": {},
	"--agent":        {},
	"--ide-mcp-port": {},
	"--log-file":     {},
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
	return IsNativeAgyACPLaunch(parsed.Executable, parsed.Args)
}

// IsNativeAgyACPLaunch identifies an already-split native CLI launch: an Antigravity CLI
// executable whose first subcommand, after global flags and their values, is `acp`.
func IsNativeAgyACPLaunch(executable string, args []string) bool {
	name := strings.ToLower(filepath.Base(strings.ReplaceAll(strings.TrimSpace(executable), `\`, "/")))
	for _, suffix := range []string{".exe", ".cmd", ".bat"} {
		name = strings.TrimSuffix(name, suffix)
	}
	if _, ok := agyNativeExecutableNames[name]; !ok {
		return false
	}
	for i := 0; i < len(args); i++ {
		trimmed := strings.TrimSpace(args[i])
		if strings.HasPrefix(trimmed, "-") {
			if _, takesValue := agyValueFlags[strings.ToLower(trimmed)]; takesValue {
				i++
			}
			continue
		}
		return strings.EqualFold(trimmed, "acp")
	}
	return false
}
