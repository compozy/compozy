package extensionpkg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/agh/internal/vault"
)

func resolveManifestCommand(
	rootDir string,
	value string,
	getenv func(string) string,
	aghExecutable func() (string, error),
) (string, error) {
	resolved, err := resolveManifestString(rootDir, value, getenv, aghExecutable)
	if err != nil {
		return "", err
	}
	if resolved == "" {
		return "", nil
	}
	if filepath.IsAbs(resolved) {
		return filepath.Clean(resolved), nil
	}
	if strings.ContainsRune(resolved, filepath.Separator) || strings.HasPrefix(resolved, ".") {
		return resolvePathWithinRoot(rootDir, resolved)
	}
	return resolved, nil
}

func resolveManifestStringSlice(
	rootDir string,
	values []string,
	getenv func(string) string,
	aghExecutable func() (string, error),
) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	resolved := make([]string, 0, len(values))
	for _, value := range values {
		item, err := resolveManifestString(rootDir, value, getenv, aghExecutable)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, item)
	}
	return resolved, nil
}

func resolveManifestStringMap(
	rootDir string,
	env map[string]string,
	getenv func(string) string,
	aghExecutable func() (string, error),
) (map[string]string, error) {
	if len(env) == 0 {
		return nil, nil
	}

	resolved := make(map[string]string, len(env))
	for key, value := range env {
		item, err := resolveManifestString(rootDir, value, getenv, aghExecutable)
		if err != nil {
			return nil, err
		}
		resolved[key] = item
	}
	return resolved, nil
}

func resolveManifestString(
	rootDir string,
	value string,
	getenv func(string) string,
	aghExecutable func() (string, error),
) (string, error) {
	resolved := strings.TrimSpace(value)
	if resolved == "" {
		return "", nil
	}

	resolved = strings.ReplaceAll(resolved, "{{config_dir}}", rootDir)
	if strings.Contains(resolved, "{{agh_executable}}") {
		executable, err := resolveAGHExecutable(aghExecutable)
		if err != nil {
			return "", err
		}
		resolved = strings.ReplaceAll(resolved, "{{agh_executable}}", executable)
	}
	for {
		start := strings.Index(resolved, "{{env:")
		if start < 0 {
			break
		}
		end := strings.Index(resolved[start:], "}}")
		if end < 0 {
			return "", fmt.Errorf("invalid env template %q", value)
		}
		end += start
		key := strings.TrimSpace(strings.TrimPrefix(resolved[start:end], "{{env:"))
		if vault.SecretLikeEnvName(key) {
			return "", fmt.Errorf("env template %q must use secret_env", key)
		}
		resolved = resolved[:start] + getenvValue(getenv, key) + resolved[end+2:]
	}
	return resolved, nil
}

func resolveAGHExecutable(aghExecutable func() (string, error)) (string, error) {
	if aghExecutable == nil {
		aghExecutable = os.Executable
	}
	executable, err := aghExecutable()
	if err != nil {
		return "", fmt.Errorf("resolve agh executable template: %w", err)
	}
	trimmed := strings.TrimSpace(executable)
	if trimmed == "" {
		return "", errors.New("resolve agh executable template: executable path is required")
	}
	if isGoTestBinary(trimmed) {
		return "", fmt.Errorf("resolve agh executable template: %q is a Go test binary, not an agh executable", trimmed)
	}
	return trimmed, nil
}

func isGoTestBinary(path string) bool {
	return strings.HasSuffix(filepath.Base(strings.TrimSpace(path)), ".test")
}

func getenvValue(getenv func(string) string, key string) string {
	if getenv == nil {
		return ""
	}
	return getenv(key)
}
