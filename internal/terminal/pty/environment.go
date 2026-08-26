package pty

import (
	"runtime"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/procutil"
)

var blockedEnvironmentPrefixes = []string{
	"ITERM_", "KITTY_", "TERM_PROGRAM", "TERM_SESSION_ID", "WEZTERM_", "WT_SESSION",
}

func environment(overrides map[string]string) []string {
	type environmentValue struct {
		key   string
		value string
	}
	values := make(map[string]environmentValue)
	for _, entry := range procutil.FilteredDaemonEnv(nil) {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || blockedEnvironmentKey(key) {
			continue
		}
		values[environmentIdentity(key)] = environmentValue{key: key, value: value}
	}
	for key, value := range overrides {
		if blockedEnvironmentKey(key) {
			continue
		}
		values[environmentIdentity(key)] = environmentValue{key: key, value: value}
	}
	values[environmentIdentity("TERM")] = environmentValue{key: "TERM", value: "xterm-256color"}
	values[environmentIdentity("COLORTERM")] = environmentValue{key: "COLORTERM", value: "truecolor"}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		entry := values[key]
		result = append(result, entry.key+"="+entry.value)
	}
	return result
}

func blockedEnvironmentKey(key string) bool {
	key = environmentIdentity(key)
	for _, prefix := range blockedEnvironmentPrefixes {
		if strings.HasPrefix(key, environmentIdentity(prefix)) {
			return true
		}
	}
	return false
}

func environmentIdentity(key string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(key)
	}
	return key
}
