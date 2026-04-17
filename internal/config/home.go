package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DirName           = ".compozy"
	ConfigFileName    = "config.toml"
	AgentsDirName     = "agents"
	ExtensionsDirName = "extensions"
	StateDirName      = "state"
	DaemonDirName     = "daemon"
	DBDirName         = "db"
	RunsDirName       = "runs"
	LogsDirName       = "logs"
	CacheDirName      = "cache"

	GlobalDBFileName = "global.db"
	DaemonSocketName = "daemon.sock"
	DaemonLockName   = "daemon.lock"
	DaemonInfoName   = "daemon.json"
	DaemonLogName    = "daemon.log"
)

var osUserHomeDir = os.UserHomeDir

// HomePaths captures the stable home-scoped Compozy layout.
type HomePaths struct {
	HomeDir       string
	ConfigFile    string
	AgentsDir     string
	ExtensionsDir string
	StateDir      string
	DaemonDir     string
	SocketPath    string
	LockPath      string
	InfoPath      string
	DBDir         string
	GlobalDBPath  string
	RunsDir       string
	LogsDir       string
	LogFile       string
	CacheDir      string
}

// ResolveHomeDir returns the canonical Compozy home root under the current user's home directory.
func ResolveHomeDir() (string, error) {
	homeDir, err := osUserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return ResolvePath(filepath.Join(homeDir, DirName))
}

// ResolveHomePaths resolves the canonical Compozy home layout from the current user's home directory.
func ResolveHomePaths() (HomePaths, error) {
	homeDir, err := ResolveHomeDir()
	if err != nil {
		return HomePaths{}, err
	}
	return ResolveHomePathsFrom(homeDir)
}

// ResolveHomePathsFrom resolves the canonical Compozy home layout from an explicit base directory.
func ResolveHomePathsFrom(homeDir string) (HomePaths, error) {
	root, err := resolveAbsoluteDir(homeDir)
	if err != nil {
		return HomePaths{}, err
	}

	return HomePaths{
		HomeDir:       root,
		ConfigFile:    filepath.Join(root, ConfigFileName),
		AgentsDir:     filepath.Join(root, AgentsDirName),
		ExtensionsDir: filepath.Join(root, ExtensionsDirName),
		StateDir:      filepath.Join(root, StateDirName),
		DaemonDir:     filepath.Join(root, DaemonDirName),
		SocketPath:    filepath.Join(root, DaemonDirName, DaemonSocketName),
		LockPath:      filepath.Join(root, DaemonDirName, DaemonLockName),
		InfoPath:      filepath.Join(root, DaemonDirName, DaemonInfoName),
		DBDir:         filepath.Join(root, DBDirName),
		GlobalDBPath:  filepath.Join(root, DBDirName, GlobalDBFileName),
		RunsDir:       filepath.Join(root, RunsDirName),
		LogsDir:       filepath.Join(root, LogsDirName),
		LogFile:       filepath.Join(root, LogsDirName, DaemonLogName),
		CacheDir:      filepath.Join(root, CacheDirName),
	}, nil
}

// EnsureHomeLayout creates and validates the stable Compozy home layout.
func EnsureHomeLayout(paths HomePaths) error {
	dirs := []struct {
		path string
		perm os.FileMode
	}{
		{paths.HomeDir, 0o755},
		{paths.AgentsDir, 0o755},
		{paths.ExtensionsDir, 0o755},
		{paths.StateDir, 0o755},
		{paths.DaemonDir, 0o700},
		{paths.DBDir, 0o755},
		{paths.RunsDir, 0o755},
		{paths.LogsDir, 0o755},
		{paths.CacheDir, 0o755},
	}

	for _, dir := range dirs {
		if err := ensureDir(dir.path, dir.perm); err != nil {
			return err
		}
	}

	return nil
}

func ensureDir(path string, perm os.FileMode) error {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return errors.New("config: home path is required")
	}

	if err := os.MkdirAll(cleanPath, perm); err != nil {
		return fmt.Errorf("create compozy directory %q: %w", cleanPath, err)
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		return fmt.Errorf("stat compozy directory %q: %w", cleanPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("config: path %q is not a directory", cleanPath)
	}
	if err := os.Chmod(cleanPath, perm); err != nil {
		return fmt.Errorf("chmod compozy directory %q: %w", cleanPath, err)
	}

	return nil
}

func resolveAbsoluteDir(path string) (string, error) {
	absPath, err := ResolvePath(path)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(absPath) == "" {
		return "", errors.New("config: base directory is empty")
	}
	return absPath, nil
}

// ResolvePath expands a possible `~`-prefixed path and returns an absolute path.
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
		return osUserHomeDir()
	}
	if !strings.HasPrefix(clean, "~/") {
		return clean, nil
	}

	homeDir, err := osUserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(homeDir, clean[2:]), nil
}
