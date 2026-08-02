package update

import (
	"context"
	"errors"
	"fmt"
	"go/build"
	"path/filepath"
	"strings"
)

// PrimeInstallDetection resolves and memoizes the install method for this manager.
func (m *Manager) PrimeInstallDetection(ctx context.Context) error {
	_, err := m.detectInstall(ctx)
	return err
}

func (m *Manager) detectInstall(ctx context.Context) (installInfo, error) {
	if ctx == nil {
		return installInfo{}, errors.New("update: install detection context is required")
	}

	for {
		if err := ctx.Err(); err != nil {
			return installInfo{}, fmt.Errorf("update: detect install method: %w", err)
		}

		m.installMu.Lock()
		if m.install != nil {
			install := *m.install
			m.installMu.Unlock()
			return install, nil
		}
		if m.installFlight != nil {
			flight := m.installFlight
			m.installMu.Unlock()
			select {
			case <-ctx.Done():
				return installInfo{}, fmt.Errorf("update: wait for install detection: %w", ctx.Err())
			case <-flight:
				continue
			}
		}
		flight := make(chan struct{})
		m.installFlight = flight
		m.installMu.Unlock()

		install, err := m.resolveInstall(ctx)
		if err == nil {
			if contextErr := ctx.Err(); contextErr != nil {
				install = installInfo{}
				err = fmt.Errorf("update: detect install method: %w", contextErr)
			}
		}

		m.installMu.Lock()
		if err == nil {
			m.install = &install
		}
		m.installFlight = nil
		close(flight)
		m.installMu.Unlock()
		return install, err
	}
}

func (m *Manager) resolveInstall(ctx context.Context) (installInfo, error) {
	override := normalizeInstallMethod(strings.TrimSpace(m.getenv(ManagedEnvName)))
	if override != "" {
		return installInfo{Method: override, Managed: true}, nil
	}

	normalizedPath := normalizePath(m.executablePath)
	switch {
	case isHomebrewPath(normalizedPath):
		return installInfo{Method: string(InstallMethodHomebrew), Managed: true}, nil
	case isNPMPath(normalizedPath):
		return installInfo{Method: string(InstallMethodNPM), Managed: true}, nil
	case isScoopPath(normalizedPath):
		return installInfo{Method: string(InstallMethodScoop), Managed: true}, nil
	case isGoInstallPath(normalizedPath, installEnvironment{
		gobin:  m.getenv("GOBIN"),
		gopath: m.getenv("GOPATH"),
	}):
		return installInfo{Method: string(InstallMethodGoInstall), Managed: true}, nil
	}

	method, err := detectLinuxPackageInstall(ctx, normalizedPath, m.runtimeOS, m.lookPath, m.runCommand)
	if err != nil {
		return installInfo{}, err
	}
	if method != "" {
		return installInfo{Method: method, Managed: true}, nil
	}

	return installInfo{Method: string(InstallMethodDirectBinary)}, nil
}

type installEnvironment struct {
	gobin  string
	gopath string
}

func detectLinuxPackageInstall(
	ctx context.Context,
	normalizedPath string,
	runtimeOS string,
	lookPath func(string) (string, error),
	runCommand func(context.Context, string, ...string) (string, error),
) (string, error) {
	if runtimeOS != runtimeOSLinux {
		return "", nil
	}
	if runCommand == nil || lookPath == nil {
		return "", nil
	}
	if normalizedPath != managedPathUsrBin && normalizedPath != managedPathBin &&
		normalizedPath != managedPathUsrLocalBin {
		return "", nil
	}
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("update: detect Linux package install: %w", err)
	}

	if _, err := lookPath("dpkg"); err == nil {
		output, cmdErr := runCommand(ctx, "dpkg", "-S", normalizedPath)
		if cmdErr == nil && strings.TrimSpace(output) != "" {
			return string(InstallMethodAPT), nil
		}
		if err := installDetectionContextError(ctx, cmdErr); err != nil {
			return "", err
		}
	}

	if _, err := lookPath("rpm"); err == nil {
		output, cmdErr := runCommand(ctx, "rpm", "-qf", normalizedPath, "--queryformat", "%{NAME}")
		if cmdErr == nil && strings.TrimSpace(output) != "" {
			if _, dnfErr := lookPath("dnf"); dnfErr == nil {
				return string(InstallMethodDNF), nil
			}
			return string(InstallMethodRPM), nil
		}
		if err := installDetectionContextError(ctx, cmdErr); err != nil {
			return "", err
		}
	}

	return "", nil
}

func installDetectionContextError(ctx context.Context, commandErr error) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("update: detect Linux package install: %w", err)
	}
	if errors.Is(commandErr, context.Canceled) || errors.Is(commandErr, context.DeadlineExceeded) {
		return fmt.Errorf("update: detect Linux package install: %w", commandErr)
	}
	return nil
}

func normalizeInstallMethod(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch normalized {
	case "", "0", "false", "no":
		return ""
	case "brew", "homebrew":
		return string(InstallMethodHomebrew)
	case "npm", "node", "nodejs":
		return string(InstallMethodNPM)
	case "apt", "deb", "debian":
		return string(InstallMethodAPT)
	case "dnf":
		return string(InstallMethodDNF)
	case string(InstallMethodRPM):
		return string(InstallMethodRPM)
	case "scoop":
		return string(InstallMethodScoop)
	case "go", "go-install", "goinstall":
		return string(InstallMethodGoInstall)
	default:
		return normalized
	}
}

func normalizePath(path string) string {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	cleaned = strings.ReplaceAll(cleaned, "\\", "/")
	return strings.ToLower(cleaned)
}

func isHomebrewPath(path string) bool {
	return strings.Contains(path, "/cellar/") || strings.Contains(path, "/caskroom/")
}

func isNPMPath(path string) bool {
	return strings.Contains(path, "/node_modules/@compozy/cli/")
}

func isScoopPath(path string) bool {
	return strings.Contains(path, "/scoop/apps/compozy/")
}

func isGoInstallPath(path string, env installEnvironment) bool {
	for _, dir := range goBinDirs(env) {
		if dir == "" {
			continue
		}
		if withinDir(path, normalizePath(dir)) {
			return true
		}
	}
	return false
}

func goBinDirs(env installEnvironment) []string {
	dirs := make([]string, 0, 4)
	if gobin := strings.TrimSpace(env.gobin); gobin != "" {
		dirs = append(dirs, gobin)
	}

	gopath := strings.TrimSpace(env.gopath)
	if gopath == "" {
		gopath = build.Default.GOPATH
	}
	for _, root := range filepath.SplitList(gopath) {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		dirs = append(dirs, filepath.Join(root, "bin"))
	}

	return dirs
}

func withinDir(path string, dir string) bool {
	if dir == "" {
		return false
	}
	if path == dir {
		return true
	}
	return strings.HasPrefix(path, dir+"/")
}
