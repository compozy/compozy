// Package service manages the per-user daemon service.
package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/fileutil"
)

const (
	linuxManager = "systemd --user"
	macManager   = "launchd"
	LingerOff    = "linger_off"
	NoManager    = "no_service_manager"
)

type Result struct {
	Manager   string `json:"manager"`
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	Warning   string `json:"warning,omitempty"`
}

type Manager struct {
	home, operatorHome, binary, platform, cloudURL string
}

func New(paths config.HomePaths, executable string) (*Manager, error) {
	home, err := config.ResolveOperatorHomeDir(paths)
	if err != nil {
		return nil, err
	}
	binary, err := executablePath(executable)
	if err != nil {
		return nil, err
	}
	return &Manager{home: paths.HomeDir, operatorHome: home, binary: binary,
		platform: runtime.GOOS, cloudURL: os.Getenv("COMPOZY_CLOUD_URL")}, nil
}

func executablePath(executable string) (string, error) {
	if filepath.IsAbs(executable) {
		return filepath.Clean(executable), nil
	}
	if strings.ContainsRune(executable, os.PathSeparator) {
		return filepath.Abs(executable)
	}
	return exec.LookPath(executable)
}

func (m *Manager) Install(ctx context.Context) (Result, error) {
	switch m.platform {
	case "linux":
		return m.installLinux(ctx)
	case "darwin":
		return m.installMac(ctx)
	default:
		return Result{Warning: NoManager}, nil
	}
}

func (m *Manager) Uninstall(ctx context.Context) (Result, error) {
	switch m.platform {
	case "linux":
		return m.uninstallLinux(ctx)
	case "darwin":
		return m.uninstallMac(ctx)
	default:
		return Result{Warning: NoManager}, nil
	}
}

func (m *Manager) Status(ctx context.Context) (Result, error) {
	switch m.platform {
	case "linux":
		return m.statusLinux(ctx)
	case "darwin":
		return m.statusMac(ctx)
	default:
		return Result{Warning: NoManager}, nil
	}
}

func writeService(path string, contents []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return fileutil.AtomicWritePrivateFile(path, contents)
}

func installed(path string) (bool, error) {
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func run(ctx context.Context, program string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	// #nosec G204 -- callers use fixed service-manager programs and structured arguments.
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Env = serviceEnvironment()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s %s: %w", program, strings.TrimSpace(string(out)), err)
	}
	return string(out), nil
}
