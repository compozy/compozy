package subprocess

import (
	"runtime"
	"slices"
	"strings"
)

const subprocessWindowsKey = "windows"

// LookupEnv returns the last matching environment assignment using the target
// platform's key semantics. Windows environment keys are case-insensitive;
// Unix environment keys are case-sensitive.
func LookupEnv(env []string, key string) (string, bool) {
	return lookupEnvForPlatform(env, key, runtime.GOOS == subprocessWindowsKey)
}

// LookupEnvFunc snapshots env and returns a platform-aware lookup closure.
func LookupEnvFunc(env []string) func(string) (string, bool) {
	snapshot := append([]string(nil), env...)
	return func(key string) (string, bool) {
		return LookupEnv(snapshot, key)
	}
}

// SetEnvValue returns a copy of env with exactly one assignment for key.
func SetEnvValue(env []string, key string, value string) []string {
	return setEnvValueForPlatform(env, key, value, runtime.GOOS == subprocessWindowsKey)
}

func setEnvValueForPlatform(env []string, key string, value string, caseInsensitive bool) []string {
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" || strings.Contains(trimmedKey, "=") {
		return append([]string(nil), env...)
	}
	result := make([]string, 0, len(env)+1)
	for _, entry := range env {
		candidate, _, ok := strings.Cut(entry, "=")
		if ok && environmentKeyEqual(candidate, trimmedKey, caseInsensitive) {
			continue
		}
		result = append(result, entry)
	}
	return append(result, trimmedKey+"="+value)
}

func lookupEnvForPlatform(env []string, key string, caseInsensitive bool) (string, bool) {
	for _, entry := range slices.Backward(env) {
		candidate, value, ok := strings.Cut(entry, "=")
		if ok && environmentKeyEqual(candidate, key, caseInsensitive) {
			return value, true
		}
	}
	return "", false
}

func environmentKeyEqual(left string, right string, caseInsensitive bool) bool {
	if caseInsensitive {
		return strings.EqualFold(left, right)
	}
	return left == right
}
