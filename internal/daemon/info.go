package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ReadyState is the persisted daemon readiness state.
type ReadyState string

const (
	ReadyStateStarting ReadyState = "starting"
	ReadyStateReady    ReadyState = "ready"
	ReadyStateStopped  ReadyState = "stopped"

	// DefaultHTTPPort is the daemon's default localhost HTTP transport port.
	DefaultHTTPPort = 2323
	// EphemeralHTTPPort requests an OS-assigned localhost HTTP port during startup.
	// The effective port is persisted after the listener binds.
	EphemeralHTTPPort = -1
)

// Info is the persisted daemon discovery record written to daemon.json.
type Info struct {
	PID        int        `json:"pid"`
	Version    string     `json:"version,omitempty"`
	SocketPath string     `json:"socket_path,omitempty"`
	HTTPPort   int        `json:"http_port,omitempty"`
	StartedAt  time.Time  `json:"started_at"`
	State      ReadyState `json:"state"`
}

// Validate ensures the persisted daemon info is usable for discovery and readiness checks.
func (i Info) Validate() error {
	switch {
	case i.PID <= 0:
		return fmt.Errorf("daemon: daemon pid must be positive: %d", i.PID)
	case i.HTTPPort < 0 || i.HTTPPort > 65535:
		return fmt.Errorf("daemon: daemon http port must be between 0 and 65535: %d", i.HTTPPort)
	case i.StartedAt.IsZero():
		return errors.New("daemon: daemon start time is required")
	case i.State == "":
		return errors.New("daemon: daemon state is required")
	}
	return nil
}

// ReadInfo loads daemon.json from disk.
func ReadInfo(path string) (Info, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return Info{}, errors.New("daemon: daemon info path is required")
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return Info{}, fmt.Errorf("daemon: read daemon info %q: %w", cleanPath, err)
	}

	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		return Info{}, fmt.Errorf("daemon: decode daemon info %q: %w", cleanPath, err)
	}
	if err := info.Validate(); err != nil {
		return Info{}, err
	}

	return info, nil
}

// WriteInfo writes daemon.json atomically via temp file and rename.
func WriteInfo(path string, info Info) error {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return errors.New("daemon: daemon info path is required")
	}
	if err := info.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o700); err != nil {
		return fmt.Errorf("daemon: create daemon info directory for %q: %w", cleanPath, err)
	}

	payload, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("daemon: encode daemon info %q: %w", cleanPath, err)
	}
	payload = append(payload, '\n')

	file, err := os.CreateTemp(filepath.Dir(cleanPath), filepath.Base(cleanPath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("daemon: create temp daemon info for %q: %w", cleanPath, err)
	}
	tempPath := file.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if _, err := file.Write(payload); err != nil {
		_ = file.Close()
		return fmt.Errorf("daemon: write temp daemon info %q: %w", tempPath, err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("daemon: sync temp daemon info %q: %w", tempPath, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("daemon: close temp daemon info %q: %w", tempPath, err)
	}
	if err := os.Chmod(tempPath, 0o600); err != nil {
		return fmt.Errorf("daemon: chmod temp daemon info %q: %w", tempPath, err)
	}
	if err := os.Rename(tempPath, cleanPath); err != nil {
		return fmt.Errorf("daemon: replace daemon info %q: %w", cleanPath, err)
	}

	return syncDir(filepath.Dir(cleanPath))
}

// RemoveInfo removes daemon.json if it exists.
func RemoveInfo(path string) error {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return nil
	}

	if err := os.Remove(cleanPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("daemon: remove daemon info %q: %w", cleanPath, err)
	}
	return nil
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("daemon: open directory %q for sync: %w", path, err)
	}
	defer func() {
		_ = dir.Close()
	}()

	if err := dir.Sync(); err != nil {
		return fmt.Errorf("daemon: sync directory %q: %w", path, err)
	}
	return nil
}
