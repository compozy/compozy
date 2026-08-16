package agentplugin

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"unicode"
)

const (
	pluginRootVariable    = "PLUGIN_ROOT"
	pluginDataVariable    = "PLUGIN_DATA"
	pluginRootPlaceholder = "${PLUGIN_ROOT}"
	pluginDataPlaceholder = "${PLUGIN_DATA}"
)

func normalizeStdioPaths(
	root string,
	dataDir string,
	command string,
	cwd *string,
	args []string,
	env map[string]string,
) (string, string, []string, map[string]string, error) {
	canonicalRoot, err := canonicalExistingPrefix(root)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("resolve package root: %w", err)
	}
	if dataDir == "" || !filepath.IsAbs(dataDir) {
		return "", "", nil, nil, errors.New("PLUGIN_DATA must be an absolute path")
	}
	canonicalData, err := canonicalExistingPrefix(dataDir)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("resolve data root: %w", err)
	}
	normalizedCommand, err := normalizeCommand(command, canonicalRoot)
	if err != nil {
		return "", "", nil, nil, err
	}
	normalizedCWD, err := normalizeCWD(cwd, canonicalRoot, canonicalData)
	if err != nil {
		return "", "", nil, nil, err
	}
	replacer := strings.NewReplacer(
		pluginRootPlaceholder,
		canonicalRoot,
		pluginDataPlaceholder,
		canonicalData,
	)
	normalizedArgs := make([]string, len(args))
	for index, argument := range args {
		normalizedArgs[index] = replacer.Replace(argument)
	}
	normalizedEnv := make(map[string]string, len(env)+2)
	for name, value := range env {
		if reservedEnvironmentName(name) {
			return "", "", nil, nil, fmt.Errorf("env cannot override reserved variable %q", name)
		}
		normalizedEnv[name] = replacer.Replace(value)
	}
	normalizedEnv[pluginRootVariable] = canonicalRoot
	normalizedEnv[pluginDataVariable] = canonicalData
	return normalizedCommand, normalizedCWD, normalizedArgs, normalizedEnv, nil
}

func normalizeCommand(command string, root string) (string, error) {
	if command == "" || strings.ContainsFunc(command, unicode.IsSpace) || strings.Contains(command, "\\") ||
		strings.Contains(command, ":") ||
		strings.Contains(command, "${PLUGIN_") {
		return "", errors.New("command must be a bare executable name or a contained ./ path")
	}
	if !strings.Contains(command, "/") {
		return command, nil
	}
	if !strings.HasPrefix(command, "./") || command == "./" {
		return "", errors.New("command must be a bare executable name or a contained ./ path")
	}
	resolved, err := resolveContained(filepath.FromSlash(strings.TrimPrefix(command, "./")), root)
	if err != nil {
		return "", errors.New("command ./ path must remain inside the package root")
	}
	return resolved, nil
}

func normalizeCWD(cwd *string, root string, dataRoot string) (string, error) {
	if cwd == nil {
		return root, nil
	}
	value := *cwd
	var base string
	var suffix string
	switch {
	case value == "./":
		base = root
	case strings.HasPrefix(value, "./"):
		base, suffix = root, strings.TrimPrefix(value, "./")
	case value == pluginRootPlaceholder:
		base = root
	case strings.HasPrefix(value, pluginRootPlaceholder+"/"):
		base, suffix = root, strings.TrimPrefix(value, pluginRootPlaceholder+"/")
	case value == pluginDataPlaceholder:
		base = dataRoot
	case strings.HasPrefix(value, pluginDataPlaceholder+"/"):
		base, suffix = dataRoot, strings.TrimPrefix(value, pluginDataPlaceholder+"/")
	default:
		return "", errors.New("cwd must be a contained ./, ${PLUGIN_ROOT}, or ${PLUGIN_DATA} path")
	}
	if strings.Contains(suffix, "\\") || strings.Contains(suffix, "${PLUGIN_") {
		return "", errors.New("cwd must not mix path roots")
	}
	resolved, err := resolveContained(filepath.FromSlash(suffix), base)
	if err != nil {
		return "", errors.New("cwd must remain inside its selected root")
	}
	return resolved, nil
}

func canonicalExistingPrefix(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("make path absolute: %w", err)
	}
	existing := filepath.Clean(absolute)
	missing := make([]string, 0)
	for {
		resolved, resolveErr := filepath.EvalSymlinks(existing)
		if resolveErr == nil {
			for _, component := range slices.Backward(missing) {
				resolved = filepath.Join(resolved, component)
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(resolveErr, os.ErrNotExist) {
			return "", resolveErr
		}
		if info, lstatErr := os.Lstat(existing); lstatErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("path %q contains an unresolved symlink", path)
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return "", resolveErr
		}
		missing = append(missing, filepath.Base(existing))
		existing = parent
	}
}

func resolveContained(path string, root string) (string, error) {
	candidate := path
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	resolved, err := canonicalExistingPrefix(candidate)
	if err != nil {
		return "", err
	}
	canonicalRoot, err := canonicalExistingPrefix(root)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(canonicalRoot, resolved)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("path %q escapes root %q", resolved, canonicalRoot)
	}
	return resolved, nil
}

func reservedEnvironmentName(name string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(name, pluginRootVariable) || strings.EqualFold(name, pluginDataVariable)
	}
	return name == pluginRootVariable || name == pluginDataVariable
}
