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
	LoopsDirName = LoopsConfigKey
	// MemoryDirName is the directory used for persistent memory files.
	MemoryDirName = "memory"
	// ProfilesDirName is the directory used for profile-owned state.
	ProfilesDirName = "profiles"
	// DefaultProfileDirName is the permanent default profile directory.
	DefaultProfileDirName = "default"
	// SessionsDirName is the directory used for persisted session state.
	SessionsDirName = "sessions"
	// ToolArtifactsDirName is the directory used for retained oversized tool results.
	ToolArtifactsDirName = "tool-artifacts"
	// SessionAttachmentsDirName is the directory used for persisted session attachments.
	SessionAttachmentsDirName = "session-attachments"
	// RestartsDirName is the directory used for persisted daemon restart operations.
	RestartsDirName = "restarts"
	// LogsDirName is the directory used for structured logs.
	LogsDirName = "logs"
	// GatewayDirName is the directory used for client-local gateway state.
	GatewayDirName = "gateway"
	// GatewayCredentialsDirName is the private gateway credential directory.
	GatewayCredentialsDirName = "credentials"
	// BinDirName is the directory for runtime binaries managed by CompozyOS.
	BinDirName = "bin"
	// DatabaseName is the global database filename.
	DatabaseName = "compozy.db"
	// DaemonSocketName is the daemon UDS filename.
	DaemonSocketName = "daemon.sock"
	// DaemonLockName is the daemon file-lock name.
	DaemonLockName = "daemon.lock"
	// UpdateLockName is the cross-process runtime mutation lock name.
	UpdateLockName = "update.lock"
	// DaemonInfoName is the daemon metadata filename.
	DaemonInfoName = "daemon.json"
	// LogFileName is the structured daemon log filename.
	LogFileName = "compozy.log"
	// NetworkAuditFileName is the append-only network audit filename.
	NetworkAuditFileName = "network.audit"
	// AppStateFileName is the desktop shell state filename.
	AppStateFileName = "app.json"
	// UpdateOperationFileName is the live host update journal filename.
	UpdateOperationFileName = "update-operation.json"
	// UpdateOperationLockName is the stable lock guarding the update journal.
	UpdateOperationLockName = "update-operation.lock"
	// UpdateHistoryFileName is the terminal update audit filename.
	UpdateHistoryFileName = "update-history.jsonl"
	// DesktopProvenanceFileName records ownership of the bundled runtime.
	DesktopProvenanceFileName = ".desktop-provenance.json"
	// AgentDefinitionFileName is the canonical file name for persisted agent definitions.
	AgentDefinitionFileName = "AGENT.md"
	agentDefName            = AgentDefinitionFileName
)

// HomePaths captures the filesystem layout for the Compozy home directory.
type HomePaths struct {
	HomeDir               string
	ConfigFile            string
	AgentsDir             string
	SkillsDir             string
	LoopsDir              string
	ProfilesDir           string
	DefaultProfileDir     string
	MemoryDir             string
	SessionsDir           string
	ToolArtifactsDir      string
	SessionAttachmentsDir string
	RestartsDir           string
	LogsDir               string
	GatewayDir            string
	GatewayCredentialsDir string
	ExtensionDataRoot     string
	BinDir                string
	LogFile               string
	NetworkAuditFile      string
	AppStateFile          string
	UpdateOperationFile   string
	UpdateOperationLock   string
	UpdateHistoryFile     string
	DesktopProvenanceFile string
	DatabaseFile          string
	DaemonSocket          string
	DaemonLock            string
	DaemonInfo            string
}

// ResolveHomeDir resolves the global Compozy home directory, honoring COMPOZY_HOME when present.
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
		if override, ok := lookup("COMPOZY_HOME"); ok && strings.TrimSpace(override) != "" {
			return resolveAbsoluteDir(override)
		}
	}

	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}

	return filepath.Join(userHome, DirName), nil
}

// ResolveHomePaths resolves the canonical Compozy home layout.
func ResolveHomePaths() (HomePaths, error) {
	return resolveHomePaths(processEnvLookup)
}

// ResolveHomePathsForWorkspace resolves the canonical Compozy home layout while
// honoring COMPOZY_HOME from the supplied workspace .env when the process env omits it.
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

// ResolveHomePathsFrom resolves the canonical Compozy home layout from an explicit directory.
func ResolveHomePathsFrom(homeDir string) (HomePaths, error) {
	root, err := resolveAbsoluteDir(homeDir)
	if err != nil {
		return HomePaths{}, err
	}

	return HomePaths{
		HomeDir:               root,
		ConfigFile:            filepath.Join(root, ConfigName),
		AgentsDir:             filepath.Join(root, AgentsDirName),
		SkillsDir:             filepath.Join(root, SkillsDirName),
		LoopsDir:              filepath.Join(root, LoopsDirName),
		ProfilesDir:           filepath.Join(root, ProfilesDirName),
		DefaultProfileDir:     filepath.Join(root, ProfilesDirName, DefaultProfileDirName),
		MemoryDir:             filepath.Join(root, ProfilesDirName, DefaultProfileDirName, MemoryDirName),
		SessionsDir:           filepath.Join(root, SessionsDirName),
		ToolArtifactsDir:      filepath.Join(root, ToolArtifactsDirName),
		SessionAttachmentsDir: filepath.Join(root, SessionAttachmentsDirName),
		RestartsDir:           filepath.Join(root, RestartsDirName),
		LogsDir:               filepath.Join(root, LogsDirName),
		GatewayDir:            filepath.Join(root, GatewayDirName),
		GatewayCredentialsDir: filepath.Join(root, GatewayDirName, GatewayCredentialsDirName),
		ExtensionDataRoot:     filepath.Join(root, ExtensionDataDirName),
		BinDir:                filepath.Join(root, BinDirName),
		LogFile:               filepath.Join(root, LogsDirName, LogFileName),
		NetworkAuditFile:      filepath.Join(root, LogsDirName, NetworkAuditFileName),
		AppStateFile:          filepath.Join(root, AppStateFileName),
		UpdateOperationFile:   filepath.Join(root, UpdateOperationFileName),
		UpdateOperationLock:   filepath.Join(root, UpdateOperationLockName),
		UpdateHistoryFile:     filepath.Join(root, LogsDirName, UpdateHistoryFileName),
		DesktopProvenanceFile: filepath.Join(root, BinDirName, DesktopProvenanceFileName),
		DatabaseFile:          filepath.Join(root, DatabaseName),
		DaemonSocket:          filepath.Join(root, DaemonSocketName),
		DaemonLock:            filepath.Join(root, DaemonLockName),
		DaemonInfo:            filepath.Join(root, DaemonInfoName),
	}, nil
}

// EnsureHomeLayout creates the directories required by the Compozy home layout.
func EnsureHomeLayout(paths HomePaths) error {
	for _, dir := range []string{
		paths.HomeDir,
		paths.AgentsDir,
		paths.SkillsDir,
		paths.LoopsDir,
		paths.ProfilesDir,
		paths.DefaultProfileDir,
		paths.MemoryDir,
		paths.SessionsDir,
		paths.ToolArtifactsDir,
		paths.SessionAttachmentsDir,
		paths.RestartsDir,
		paths.LogsDir,
		paths.GatewayDir,
		paths.GatewayCredentialsDir,
		paths.ExtensionDataRoot,
		paths.BinDir,
	} {
		if strings.TrimSpace(dir) == "" {
			return errors.New("config: home path is required")
		}
		if err := os.MkdirAll(dir, privateDirMode); err != nil {
			return fmt.Errorf("create compozy directory %q: %w", dir, err)
		}
		if err := os.Chmod(dir, privateDirMode); err != nil {
			return fmt.Errorf("secure compozy directory %q: %w", dir, err)
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
