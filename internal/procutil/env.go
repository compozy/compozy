package procutil

import (
	"os"
	"strings"
)

// FilteredDaemonEnv returns process environment entries safe to pass to child processes.
func FilteredDaemonEnv(base []string) []string {
	env := append([]string(nil), base...)
	if len(env) == 0 {
		env = os.Environ()
	}

	filtered := make([]string, 0, len(env))
	for _, entry := range env {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if SensitiveEnvName(parts[0]) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

// IsolatedDaemonEnv returns only the fixed operational environment allowlist
// needed to launch local subprocesses. It intentionally drops all non-allowlisted
// daemon variables, including provider CLI credentials.
func IsolatedDaemonEnv(base []string) []string {
	env := append([]string(nil), base...)
	if len(env) == 0 {
		env = os.Environ()
	}

	filtered := make([]string, 0, len(env))
	for _, entry := range env {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.ToUpper(strings.TrimSpace(parts[0]))
		if SensitiveEnvName(name) || !safeDaemonEnvName(name) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

// SensitiveEnvName reports whether an environment variable name commonly carries credentials.
func SensitiveEnvName(name string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(name))
	if normalized == "" {
		return false
	}
	return credentialShapedEnvName(normalized)
}

func credentialShapedEnvName(normalized string) bool {
	for _, marker := range []string{
		"API_KEY",
		"APIKEY",
		"ACCESS_KEY",
		"PRIVATE_KEY",
		"TOKEN",
		"SECRET",
		"PASSWORD",
		"PASSWD",
		"CREDENTIAL",
		"AUTH",
		"COOKIE",
		"SESSION",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return strings.HasSuffix(normalized, "_KEY")
}

func safeDaemonEnvName(name string) bool {
	switch name {
	case "PATH",
		"HOME",
		"USER",
		"LOGNAME",
		"SHELL",
		"TMPDIR",
		"TMP",
		"TEMP",
		"LANG",
		"LC_ALL",
		"LC_CTYPE",
		"TERM",
		"COLORTERM",
		"NO_COLOR",
		"FORCE_COLOR",
		"AGH_HOME",
		"AGH_CONFIG",
		"AGH_LOG_LEVEL",
		"PROVIDER_HOME",
		"PROVIDER_CODEX_HOME":
		return true
	default:
		return strings.HasPrefix(name, "LC_") || strings.HasPrefix(name, "XDG_")
	}
}
