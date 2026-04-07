package update

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const stateFilename = "state.yml"

var (
	osUserHomeDir = os.UserHomeDir
	readFile      = os.ReadFile
	writeFile     = os.WriteFile
	mkdirAll      = os.MkdirAll
)

// StateEntry stores the cached release-check metadata on disk.
type StateEntry struct {
	CheckedForUpdateAt time.Time   `yaml:"checked_for_update_at"`
	LatestRelease      ReleaseInfo `yaml:"latest_release"`
}

// StateFilePath returns the XDG-compliant path for the update state file.
func StateFilePath() (string, error) {
	xdgConfigHome := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if xdgConfigHome == "" {
		homeDir, err := osUserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		xdgConfigHome = filepath.Join(homeDir, ".config")
	}

	return filepath.Join(filepath.Clean(xdgConfigHome), "compozy", stateFilename), nil
}

// ReadState reads the persisted update cache from disk.
//
// Missing or malformed files are treated as empty cache entries.
func ReadState(path string) (*StateEntry, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("state file path is empty")
	}

	data, err := readFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read state file %q: %w", path, err)
	}

	var entry StateEntry
	if err := yaml.Unmarshal(data, &entry); err != nil {
		return nil, nil
	}

	return &entry, nil
}

// WriteState writes the update cache to disk, creating parent directories when needed.
func WriteState(path string, entry *StateEntry) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("state file path is empty")
	}
	if entry == nil {
		return fmt.Errorf("state entry is nil")
	}

	data, err := yaml.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal state entry: %w", err)
	}

	if err := mkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create state directory for %q: %w", path, err)
	}
	if err := writeFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write state file %q: %w", path, err)
	}

	return nil
}
