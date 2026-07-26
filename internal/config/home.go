package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	privateDirMode  = 0o700
	privateFileMode = 0o600
)

const (
	// AgentsDirName is the directory used for persisted agent definitions.
	AgentsDirName = "agents"
	// SkillsDirName is the directory used for persisted user skills.
	SkillsDirName = "skills"
	// LoopsDirName is the directory used for persisted user loop definitions.
	LoopsDirName = "loops"
	// MemoryDirName is the directory used for persistent memory files.
	MemoryDirName = "memory"
	// SessionsDirName is the directory used for persisted session state.
	SessionsDirName = "sessions"
	// ToolArtifactsDirName is the directory used for retained oversized tool results.
	ToolArtifactsDirName = "tool-artifacts"
	// RestartsDirName is the directory used for persisted daemon restart operations.
	RestartsDirName = "restarts"
	// LogsDirName is the directory used for structured logs.
	LogsDirName = "logs"
	// DatabaseName is the global database filename.
	DatabaseName = "agh.db"
	// DaemonSocketName is the daemon UDS filename.
	DaemonSocketName = "daemon.sock"
	// DaemonLockName is the daemon file-lock name.
	DaemonLockName = "daemon.lock"
	// DaemonInfoName is the daemon metadata filename.
	DaemonInfoName = "daemon.json"
	// LogFileName is the structured daemon log filename.
	LogFileName = "agh.log"
	// NetworkAuditFileName is the append-only network audit filename.
	NetworkAuditFileName = "network.audit"
	// AgentDefinitionFileName is the canonical file name for persisted agent definitions.
	AgentDefinitionFileName = "AGENT.md"
	agentDefName            = AgentDefinitionFileName
)

// HomePaths captures the filesystem layout for the AGH home directory.
type HomePaths struct {
	HomeDir          string
	ConfigFile       string
	AgentsDir        string
	SkillsDir        string
	LoopsDir         string
	MemoryDir        string
	SessionsDir      string
	ToolArtifactsDir string
	RestartsDir      string
	LogsDir          string
	LogFile          string
	NetworkAuditFile string
	DatabaseFile     string
	DaemonSocket     string
	DaemonLock       string
	DaemonInfo       string
}

// ResolveHomeDir resolves the global AGH home directory, honoring AGH_HOME when present.
func ResolveHomeDir() (string, error) {
	return resolveHomeDir(processEnvLookup)
}

// ResolveOperatorHomeDir resolves the canonical operator user home directory for workspace defaults.
func ResolveOperatorHomeDir(homePaths HomePaths) (string, error) {
	return ResolveOperatorHomeDirWithLookup(homePaths, processEnvLookup)
}

// ResolveOperatorHomeDirWithLookup resolves the canonical operator user home directory with injectable env lookup.
func ResolveOperatorHomeDirWithLookup(
	homePaths HomePaths,
	lookup func(string) (string, bool),
) (string, error) {
	return resolveOperatorHomeDir(homePaths, lookup, os.UserHomeDir)
}

func resolveOperatorHomeDir(
	homePaths HomePaths,
	lookup func(string) (string, bool),
	lookupUserHome func() (string, error),
) (string, error) {
	if lookup != nil {
		if homeDir, ok := lookup("HOME"); ok && strings.TrimSpace(homeDir) != "" {
			return resolveCanonicalDirIfExists(homeDir)
		}
	}

	if lookupUserHome != nil {
		userHome, err := lookupUserHome()
		if err != nil {
			if fallback, ok := fallbackOperatorHomeDir(homePaths); ok {
				return fallback, nil
			}
			return "", fmt.Errorf("resolve user home directory: %w", err)
		}
		resolvedUserHome, resolveErr := resolveCanonicalDirIfExists(userHome)
		if resolveErr == nil && strings.TrimSpace(resolvedUserHome) != "" {
			return resolvedUserHome, nil
		}
		if fallback, ok := fallbackOperatorHomeDir(homePaths); ok {
			return fallback, nil
		}
		if resolveErr != nil {
			return "", fmt.Errorf("resolve user home directory: %w", resolveErr)
		}
		return "", errors.New("config: operator home directory is required")
	}

	if fallback, ok := fallbackOperatorHomeDir(homePaths); ok {
		return fallback, nil
	}
	return "", errors.New("config: operator home directory is required")
}

func fallbackOperatorHomeDir(homePaths HomePaths) (string, bool) {
	homeDir := strings.TrimSpace(homePaths.HomeDir)
	if homeDir == "" || filepath.Base(homeDir) != DirName {
		return "", false
	}

	parent := filepath.Dir(homeDir)
	if parent == "." || parent == homeDir || strings.TrimSpace(parent) == "" {
		return "", false
	}
	return parent, true
}

func resolveHomeDir(lookup envLookup) (string, error) {
	if lookup != nil {
		if override, ok := lookup("AGH_HOME"); ok && strings.TrimSpace(override) != "" {
			return resolveAbsoluteDir(override)
		}
	}

	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}

	return filepath.Join(userHome, DirName), nil
}

// ResolveHomePaths resolves the canonical AGH home layout.
func ResolveHomePaths() (HomePaths, error) {
	return resolveHomePaths(processEnvLookup)
}

// ResolveHomePathsForWorkspace resolves the canonical AGH home layout while
// honoring AGH_HOME from the supplied workspace .env when the process env omits it.
func ResolveHomePathsForWorkspace(workspaceRoot string) (HomePaths, error) {
	workspaceRoot, err := resolveWorkspaceRoot(workspaceRoot)
	if err != nil {
		return HomePaths{}, err
	}
	lookup := processEnvLookup
	dotenvLookup, err := loadDotEnvLookup(workspaceRoot)
	if err != nil {
		return HomePaths{}, err
	}
	if dotenvLookup != nil {
		lookup = layeredEnvLookup(processEnvLookup, dotenvLookup)
	}
	return resolveHomePaths(lookup)
}

func resolveHomePaths(lookup envLookup) (HomePaths, error) {
	homeDir, err := resolveHomeDir(lookup)
	if err != nil {
		return HomePaths{}, err
	}

	return ResolveHomePathsFrom(homeDir)
}

// ResolveHomePathsFrom resolves the canonical AGH home layout from an explicit directory.
func ResolveHomePathsFrom(homeDir string) (HomePaths, error) {
	root, err := resolveAbsoluteDir(homeDir)
	if err != nil {
		return HomePaths{}, err
	}

	return HomePaths{
		HomeDir:          root,
		ConfigFile:       filepath.Join(root, ConfigName),
		AgentsDir:        filepath.Join(root, AgentsDirName),
		SkillsDir:        filepath.Join(root, SkillsDirName),
		LoopsDir:         filepath.Join(root, LoopsDirName),
		MemoryDir:        filepath.Join(root, MemoryDirName),
		SessionsDir:      filepath.Join(root, SessionsDirName),
		ToolArtifactsDir: filepath.Join(root, ToolArtifactsDirName),
		RestartsDir:      filepath.Join(root, RestartsDirName),
		LogsDir:          filepath.Join(root, LogsDirName),
		LogFile:          filepath.Join(root, LogsDirName, LogFileName),
		NetworkAuditFile: filepath.Join(root, LogsDirName, NetworkAuditFileName),
		DatabaseFile:     filepath.Join(root, DatabaseName),
		DaemonSocket:     filepath.Join(root, DaemonSocketName),
		DaemonLock:       filepath.Join(root, DaemonLockName),
		DaemonInfo:       filepath.Join(root, DaemonInfoName),
	}, nil
}

// EnsureHomeLayout creates the directories required by the AGH home layout.
func EnsureHomeLayout(paths HomePaths) error {
	for _, dir := range []string{
		paths.HomeDir,
		paths.AgentsDir,
		paths.SkillsDir,
		paths.LoopsDir,
		paths.MemoryDir,
		paths.SessionsDir,
		paths.ToolArtifactsDir,
		paths.RestartsDir,
		paths.LogsDir,
	} {
		if strings.TrimSpace(dir) == "" {
			return errors.New("config: home path is required")
		}
		if err := os.MkdirAll(dir, privateDirMode); err != nil {
			return fmt.Errorf("create agh directory %q: %w", dir, err)
		}
		if err := os.Chmod(dir, privateDirMode); err != nil {
			return fmt.Errorf("secure agh directory %q: %w", dir, err)
		}
	}

	return nil
}

func resolveAbsoluteDir(path string) (string, error) {
	absPath, err := ResolvePath(path)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(absPath) == "" {
		return "", errors.New("config: path is required")
	}
	return absPath, nil
}

func resolveCanonicalDirIfExists(path string) (string, error) {
	absPath, err := resolveAbsoluteDir(path)
	if err != nil {
		return "", err
	}

	canonicalPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return absPath, nil
		}
		return "", fmt.Errorf("resolve canonical path %q: %w", path, err)
	}

	canonicalPath, err = filepath.Abs(canonicalPath)
	if err != nil {
		return "", fmt.Errorf("resolve canonical path %q: %w", canonicalPath, err)
	}
	return canonicalPath, nil
}

// ResolvePath expands `~`-prefixed paths and returns an absolute path.
func ResolvePath(path string) (string, error) {
	expanded, err := expandUserPath(path)
	if err != nil {
		return "", err
	}

	clean := strings.TrimSpace(expanded)
	if clean == "" {
		return "", nil
	}

	absPath, err := filepath.Abs(clean)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path %q: %w", path, err)
	}

	return absPath, nil
}

func expandUserPath(path string) (string, error) {
	clean := strings.TrimSpace(path)
	if clean == "" {
		return "", nil
	}
	if clean == "~" {
		return os.UserHomeDir()
	}
	if !strings.HasPrefix(clean, "~/") {
		return clean, nil
	}

	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}

	return filepath.Join(userHome, clean[2:]), nil
}
