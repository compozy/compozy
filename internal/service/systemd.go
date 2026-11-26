package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

const unitName = "compozy.service"

func (m *Manager) unitPath() string {
	return filepath.Join(m.operatorHome, ".config", "systemd", "user", unitName)
}

func (m *Manager) installLinux(ctx context.Context) (Result, error) {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return Result{Manager: linuxManager, Warning: NoManager}, nil
	}
	if _, err := systemctl(ctx, "show-environment"); err != nil {
		return Result{Manager: linuxManager, Warning: NoManager}, nil
	}
	if err := writeService(m.unitPath(), []byte(m.unit())); err != nil {
		return Result{}, err
	}
	if _, err := systemctl(ctx, "daemon-reload"); err != nil {
		return Result{}, err
	}
	if _, err := systemctl(ctx, "enable", unitName); err != nil {
		return Result{}, err
	}
	if _, err := systemctl(ctx, "restart", unitName); err != nil {
		return Result{}, err
	}
	return m.statusLinux(ctx)
}

func (m *Manager) unit() string {
	cfg := "[Unit]\nDescription=CompozyOS daemon\n\n[Service]\nType=simple\n" +
		"ExecStart=" + systemdQuote(m.binary) + " daemon start --foreground\n" +
		"Environment=" + systemdQuote("COMPOZY_HOME="+m.home) + "\n"
	if m.cloudURL != "" {
		cfg += "Environment=" + systemdQuote("COMPOZY_CLOUD_URL="+m.cloudURL) + "\n"
	}
	return cfg + "Restart=on-failure\nRestartSec=2\n\n[Install]\nWantedBy=default.target\n"
}

func systemdQuote(s string) string {
	return strconv.Quote(strings.ReplaceAll(strings.ReplaceAll(s, "%", "%%"), "$", "$$"))
}

func (m *Manager) uninstallLinux(ctx context.Context) (Result, error) {
	result, err := m.statusLinux(ctx)
	if err != nil || !result.Installed {
		return result, err
	}
	if _, err := systemctl(ctx, "disable", "--now", unitName); err != nil {
		return result, err
	}
	if err := os.Remove(m.unitPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return result, err
	}
	_, err = systemctl(ctx, "daemon-reload")
	return Result{Manager: linuxManager}, err
}

func (m *Manager) statusLinux(ctx context.Context) (Result, error) {
	present, err := installed(m.unitPath())
	result := Result{Manager: linuxManager, Installed: present}
	if err != nil || !present {
		return result, err
	}
	_, activeErr := systemctl(ctx, "is-active", "--quiet", unitName)
	result.Running = activeErr == nil
	if current, err := user.Current(); err == nil {
		output, err := run(ctx, "loginctl", "show-user", current.Username, "-p", "Linger")
		if err == nil && strings.TrimSpace(output) == "Linger=no" {
			result.Warning = LingerOff
		}
	}
	return result, nil
}

func systemctl(ctx context.Context, args ...string) (string, error) {
	return run(ctx, "systemctl", append([]string{"--user"}, args...)...)
}

func serviceEnvironment() []string {
	env := os.Environ()
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir == "" {
		if current, err := user.Current(); err == nil {
			env = append(env, fmt.Sprintf("XDG_RUNTIME_DIR=/run/user/%s", current.Uid))
		}
	}
	return env
}
