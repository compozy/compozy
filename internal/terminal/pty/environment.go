package pty

import (
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/procutil"
)

var blockedEnvironmentPrefixes = []string{
	"ITERM_", "KITTY_", "TERM_PROGRAM", "TERM_SESSION_ID", "WEZTERM_", "WT_SESSION",
}

func environment(overrides map[string]string) []string {
	values := make(map[string]string)
	for _, entry := range procutil.FilteredDaemonEnv(nil) {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || blockedEnvironmentKey(key) {
			continue
		}
		values[key] = value
	}
	for key, value := range overrides {
		if blockedEnvironmentKey(key) {
			continue
		}
		values[key] = value
	}
	values["TERM"] = "xterm-256color"
	values["COLORTERM"] = "truecolor"
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result
}

func blockedEnvironmentKey(key string) bool {
	for _, prefix := range blockedEnvironmentPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}
