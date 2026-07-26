package providerenv

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
)

var errProviderDirSymlink = errors.New("isolated provider directory contains symlink")

// ApplyHomePolicy updates provider launch environment according to the provider
// home policy. Operator home intentionally leaves the incoming env untouched.
func ApplyHomePolicy(
	homePaths aghconfig.HomePaths,
	providerName string,
	homePolicy aghconfig.ProviderHomePolicy,
	env []string,
) ([]string, error) {
	if homePolicy != aghconfig.ProviderHomePolicyIsolated {
		return env, nil
	}
	trimmedProvider, providerHome, err := isolatedProviderHome(homePaths, providerName)
	if err != nil {
		return nil, err
	}
	managedRoot := filepath.Clean(homePaths.HomeDir)
	for _, dir := range []string{
		providerHome,
		filepath.Join(providerHome, ".config"),
		filepath.Join(providerHome, ".local", "share"),
		filepath.Join(providerHome, ".cache"),
	} {
		if err := ensurePrivateDirUnder(managedRoot, dir); err != nil {
			return nil, err
		}
	}

	if err := ensureKnownProviderHomeDirs(trimmedProvider, managedRoot, providerHome); err != nil {
		return nil, err
	}
	return setProviderHomeEnvValues(trimmedProvider, providerHome, env), nil
}

// ResolveHomeEnv returns the provider home environment for display or
// diagnostics without creating provider-owned directories.
func ResolveHomeEnv(
	homePaths aghconfig.HomePaths,
	providerName string,
	homePolicy aghconfig.ProviderHomePolicy,
	env []string,
) ([]string, error) {
	if homePolicy != aghconfig.ProviderHomePolicyIsolated {
		return env, nil
	}
	trimmedProvider, providerHome, err := isolatedProviderHome(homePaths, providerName)
	if err != nil {
		return nil, err
	}
	return setProviderHomeEnvValues(trimmedProvider, providerHome, env), nil
}

// ApplyPiAgentDirPolicy points native Pi auth at the same isolated home used by
// both session launch and provider auth commands.
func ApplyPiAgentDirPolicy(
	homePaths aghconfig.HomePaths,
	providerName string,
	homePolicy aghconfig.ProviderHomePolicy,
	env []string,
) ([]string, error) {
	if homePolicy != aghconfig.ProviderHomePolicyIsolated {
		return env, nil
	}
	_, providerHome, err := isolatedProviderHome(homePaths, providerName)
	if err != nil {
		return nil, err
	}
	agentDir := filepath.Join(providerHome, ".pi", "agent")
	if err := ensurePrivateDirUnder(filepath.Clean(homePaths.HomeDir), agentDir); err != nil {
		return nil, err
	}
	return SetEnvValue(env, "PI_CODING_AGENT_DIR", agentDir), nil
}

// ResolvePiAgentDirEnv returns the Pi native-auth environment for display or
// diagnostics without creating provider-owned directories.
func ResolvePiAgentDirEnv(
	homePaths aghconfig.HomePaths,
	providerName string,
	homePolicy aghconfig.ProviderHomePolicy,
	env []string,
) ([]string, error) {
	if homePolicy != aghconfig.ProviderHomePolicyIsolated {
		return env, nil
	}
	_, providerHome, err := isolatedProviderHome(homePaths, providerName)
	if err != nil {
		return nil, err
	}
	return SetEnvValue(env, "PI_CODING_AGENT_DIR", filepath.Join(providerHome, ".pi", "agent")), nil
}

// EnsurePrivateDir creates or tightens an AGH-owned provider state directory.
func EnsurePrivateDir(path string) error {
	cleanPath := filepath.Clean(path)
	return ensurePrivateDirUnder(filepath.Dir(cleanPath), cleanPath)
}

// EnsurePrivateDirUnder creates or tightens an AGH-owned provider state
// directory while proving the resolved path remains below root.
func EnsurePrivateDirUnder(root string, path string) error {
	return ensurePrivateDirUnder(root, path)
}

func ensurePrivateDirUnder(root string, path string) error {
	cleanRoot := filepath.Clean(root)
	cleanPath := filepath.Clean(path)
	if err := ensurePathUnder(cleanRoot, cleanPath); err != nil {
		return err
	}
	if err := rejectSymlinkComponents(cleanRoot, cleanPath); err != nil {
		return err
	}
	if err := os.MkdirAll(cleanPath, 0o700); err != nil {
		return fmt.Errorf("create isolated provider directory %q: %w", cleanPath, err)
	}
	if err := rejectSymlinkComponents(cleanRoot, cleanPath); err != nil {
		return err
	}
	if err := verifyResolvedUnder(cleanRoot, cleanPath); err != nil {
		return err
	}
	if err := os.Chmod(cleanPath, 0o700); err != nil {
		return fmt.Errorf("secure isolated provider directory %q: %w", cleanPath, err)
	}
	if err := rejectSymlinkComponents(cleanRoot, cleanPath); err != nil {
		return err
	}
	return verifyResolvedUnder(cleanRoot, cleanPath)
}

func ensurePathUnder(root string, path string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return fmt.Errorf("resolve isolated provider directory %q under %q: %w", path, root, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("isolated provider directory %q escapes managed root %q", path, root)
	}
	return nil
}

func rejectSymlinkComponents(root string, path string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return fmt.Errorf("resolve isolated provider directory %q under %q: %w", path, root, err)
	}
	current := root
	if rel == "." {
		return rejectSymlinkComponent(current)
	}
	for part := range strings.SplitSeq(rel, string(os.PathSeparator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		if err := rejectSymlinkComponent(current); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
	}
	return nil
}

func rejectSymlinkComponent(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return err
		}
		return fmt.Errorf("inspect isolated provider directory %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: %q", errProviderDirSymlink, path)
	}
	if !info.IsDir() {
		return fmt.Errorf("isolated provider path %q is not a directory", path)
	}
	return nil
}

func verifyResolvedUnder(root string, path string) error {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve isolated provider root %q: %w", root, err)
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("resolve isolated provider directory %q: %w", path, err)
	}
	return ensurePathUnder(resolvedRoot, resolvedPath)
}

// SetEnvValue returns env with exactly one KEY=value entry.
func SetEnvValue(env []string, key string, value string) []string {
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" || strings.Contains(trimmedKey, "=") {
		return env
	}
	entry := trimmedKey + "=" + value
	filtered := env[:0]
	for _, current := range env {
		if strings.HasPrefix(current, trimmedKey+"=") {
			continue
		}
		filtered = append(filtered, current)
	}
	return append(filtered, entry)
}

// SafeProviderHomeSegment reports whether a provider name can be used as one
// path segment below $AGH_HOME/providers.
func SafeProviderHomeSegment(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func isolatedProviderHome(homePaths aghconfig.HomePaths, providerName string) (string, string, error) {
	trimmedProvider := strings.TrimSpace(providerName)
	if !SafeProviderHomeSegment(trimmedProvider) {
		return "", "", fmt.Errorf("provider %q cannot use isolated home policy", trimmedProvider)
	}
	if strings.TrimSpace(homePaths.HomeDir) == "" {
		return "", "", errors.New("AGH home is required for isolated provider home policy")
	}
	return trimmedProvider, filepath.Join(homePaths.HomeDir, "providers", trimmedProvider), nil
}

func setProviderHomeEnvValues(providerName string, providerHome string, env []string) []string {
	env = SetEnvValue(env, "PROVIDER_HOME", providerHome)
	env = SetEnvValue(env, "HOME", providerHome)
	env = SetEnvValue(env, "XDG_CONFIG_HOME", filepath.Join(providerHome, ".config"))
	env = SetEnvValue(env, "XDG_DATA_HOME", filepath.Join(providerHome, ".local", "share"))
	env = SetEnvValue(env, "XDG_CACHE_HOME", filepath.Join(providerHome, ".cache"))
	for key, dir := range knownProviderHomeDirs(providerName, providerHome) {
		env = SetEnvValue(env, key, dir)
	}
	return env
}

func ensureKnownProviderHomeDirs(
	providerName string,
	managedRoot string,
	providerHome string,
) error {
	for _, dir := range knownProviderHomeDirs(providerName, providerHome) {
		if err := ensurePrivateDirUnder(managedRoot, dir); err != nil {
			return err
		}
	}
	return nil
}

func knownProviderHomeDirs(providerName string, providerHome string) map[string]string {
	knownDirs := map[string]map[string]string{
		"claude": {
			"CLAUDE_CONFIG_DIR": filepath.Join(providerHome, "claude"),
		},
		"codex": {
			"CODEX_HOME":          filepath.Join(providerHome, "codex"),
			"PROVIDER_CODEX_HOME": filepath.Join(providerHome, "codex"),
		},
		"opencode": {
			"OPENCODE_CONFIG_DIR": filepath.Join(providerHome, "opencode"),
		},
	}
	return knownDirs[providerName]
}
