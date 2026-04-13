package extensions

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// InstallOriginFileName records where a user-scoped extension was installed from.
	InstallOriginFileName = ".compozy-origin.json"
)

// InstallOrigin captures provenance metadata for an installed extension.
type InstallOrigin struct {
	Remote         string    `json:"remote"`
	Repository     string    `json:"repository,omitempty"`
	Ref            string    `json:"ref,omitempty"`
	Subdir         string    `json:"subdir,omitempty"`
	ResolvedSource string    `json:"resolved_source"`
	InstalledAt    time.Time `json:"installed_at"`
}

// LoadInstallOrigin loads persisted install provenance from one extension directory.
func LoadInstallOrigin(dir string) (*InstallOrigin, error) {
	resolvedDir := strings.TrimSpace(dir)
	if resolvedDir == "" {
		return nil, fmt.Errorf("load extension install origin: directory is empty")
	}

	path := filepath.Join(resolvedDir, InstallOriginFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read extension install origin %q: %w", path, err)
	}

	var origin InstallOrigin
	if err := json.Unmarshal(data, &origin); err != nil {
		return nil, fmt.Errorf("decode extension install origin %q: %w", path, err)
	}
	return &origin, nil
}

// WriteInstallOrigin persists install provenance into one extension directory.
func WriteInstallOrigin(dir string, origin InstallOrigin) error {
	resolvedDir := strings.TrimSpace(dir)
	if resolvedDir == "" {
		return fmt.Errorf("write extension install origin: directory is empty")
	}

	path := filepath.Join(resolvedDir, InstallOriginFileName)
	payload, err := json.MarshalIndent(origin, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal extension install origin: %w", err)
	}
	if err := os.WriteFile(path, append(payload, '\n'), 0o600); err != nil {
		return fmt.Errorf("write extension install origin %q: %w", path, err)
	}
	return nil
}
