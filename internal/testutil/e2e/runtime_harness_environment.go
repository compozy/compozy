package e2e

import (
	"errors"
	"fmt"

	"net/http"

	"os"

	"path/filepath"
	"runtime"

	"strings"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/testutil"
)

func runtimeEnv(homePaths aghconfig.HomePaths, extra map[string]string) []string {
	base := testutil.HermeticProcessEnv(os.Environ())
	base = setEnvValue(base, "AGH_HOME", homePaths.HomeDir)
	base = setEnvValue(base, "HOME", homePaths.HomeDir)

	keys := make([]string, 0, len(extra))
	for key := range extra {
		keys = append(keys, key)
	}
	sortStrings(keys)
	for _, key := range keys {
		if reservedRuntimeEnvKey(key) {
			continue
		}
		base = setEnvValue(base, key, extra[key])
	}
	base = setEnvValue(base, "AGH_HOME", homePaths.HomeDir)
	base = setEnvValue(base, "HOME", homePaths.HomeDir)
	return base
}

func reservedRuntimeEnvKey(key string) bool {
	switch strings.TrimSpace(key) {
	case "AGH_HOME", "HOME":
		return true
	default:
		return false
	}
}

func withRuntimeCLIEnv(
	homePaths aghconfig.HomePaths,
	env []string,
	binaryPath string,
) ([]string, error) {
	trimmedBinaryPath := strings.TrimSpace(binaryPath)
	if trimmedBinaryPath == "" {
		return env, nil
	}

	shimPath, err := installRuntimeCLI(homePaths, trimmedBinaryPath)
	if err != nil {
		return nil, err
	}
	withPath := setEnvValue(env, "PATH", prependPath(filepath.Dir(shimPath), lookupEnvValue(env, "PATH")))
	withPath = setEnvValue(withPath, "AGH_E2E_CLI_BIN", shimPath)
	withPath = setEnvValue(withPath, daemonBinaryEnvVar, trimmedBinaryPath)
	return withPath, nil
}

func installRuntimeCLI(homePaths aghconfig.HomePaths, binaryPath string) (string, error) {
	binDir := filepath.Join(homePaths.HomeDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir runtime cli dir %q: %w", binDir, err)
	}

	targetName := "agh"
	if runtime.GOOS == windowsGOOS {
		targetName = "agh.exe"
	}
	targetPath := filepath.Join(binDir, targetName)
	if err := os.Remove(targetPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("remove existing runtime cli target %q: %w", targetPath, err)
	}
	if runtime.GOOS != windowsGOOS {
		if err := os.Symlink(binaryPath, targetPath); err == nil {
			return targetPath, nil
		}
	}
	if err := os.Link(binaryPath, targetPath); err == nil {
		return targetPath, nil
	}
	if err := copyFile(binaryPath, targetPath); err != nil {
		return "", fmt.Errorf("copy runtime cli binary to %q: %w", targetPath, err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(targetPath, 0o755); err != nil {
			return "", fmt.Errorf("chmod runtime cli target %q: %w", targetPath, err)
		}
	}
	return targetPath, nil
}

func setEnvValue(env []string, key string, value string) []string {
	targetKey := strings.TrimSpace(key)
	if targetKey == "" {
		return env
	}
	entry := targetKey + "=" + value
	updated := make([]string, 0, len(env)+1)
	replaced := false
	for _, current := range env {
		existingKey, _, ok := strings.Cut(current, "=")
		if ok && existingKey == targetKey {
			if !replaced {
				updated = append(updated, entry)
				replaced = true
			}
			continue
		}
		updated = append(updated, current)
	}
	if !replaced {
		updated = append(updated, entry)
	}
	return updated
}

func lookupEnvValue(env []string, key string) string {
	targetKey := strings.TrimSpace(key)
	for _, current := range env {
		existingKey, existingValue, ok := strings.Cut(current, "=")
		if ok && existingKey == targetKey {
			return existingValue
		}
	}
	return ""
}

func prependPath(prefix string, current string) string {
	trimmedPrefix := strings.TrimSpace(prefix)
	trimmedCurrent := strings.TrimSpace(current)
	switch {
	case trimmedPrefix == "":
		return trimmedCurrent
	case trimmedCurrent == "":
		return trimmedPrefix
	default:
		return trimmedPrefix + string(os.PathListSeparator) + trimmedCurrent
	}
}

func closeIdleConnections(client *http.Client) {
	if client == nil || client.Transport == nil {
		return
	}
	if closer, ok := client.Transport.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}
