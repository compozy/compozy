package update

import (
	"context"
	"go/build"
	"path/filepath"
	"strings"
)

// PrimeInstallDetection resolves and memoizes the install method for this manager.
func (m *Manager) PrimeInstallDetection(ctx context.Context) {
	_ = m.detectInstall(ctx)
}

func (m *Manager) detectInstall(ctx context.Context) installInfo {
	if ctx == nil {
		ctx = context.Background()
	}

	m.installOnce.Do(func() {
		m.install = m.resolveInstall(ctx)
	})
	return m.install
}

func (m *Manager) resolveInstall(ctx context.Context) installInfo {
	override := normalizeInstallMethod(strings.TrimSpace(m.getenv(ManagedEnvName)))
	if override != "" {
		return installInfo{Method: override, Managed: true}
	}

	normalizedPath := normalizePath(m.executablePath)
	switch {
	case isHomebrewPath(normalizedPath):
		return installInfo{Method: string(InstallMethodHomebrew), Managed: true}
	case isNPMPath(normalizedPath):
		return installInfo{Method: string(InstallMethodNPM), Managed: true}
	case isScoopPath(normalizedPath):
		return installInfo{Method: string(InstallMethodScoop), Managed: true}
	case isGoInstallPath(normalizedPath, installEnvironment{
		gobin:  m.getenv("GOBIN"),
		gopath: m.getenv("GOPATH"),
	}):
		return installInfo{Method: string(InstallMethodGoInstall), Managed: true}
	}

	if method := detectLinuxPackageInstall(ctx, normalizedPath, m.runtimeOS, m.lookPath, m.runCommand); method != "" {
		return installInfo{Method: method, Managed: true}
	}

	return installInfo{Method: string(InstallMethodDirectBinary)}
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
) string {
	if runtimeOS != runtimeOSLinux {
		return ""
	}
	if runCommand == nil || lookPath == nil {
		return ""
	}
	if normalizedPath != managedPathUsrBin && normalizedPath != managedPathBin &&
		normalizedPath != managedPathUsrLocalBin {
		return ""
	}

	if _, err := lookPath("dpkg"); err == nil {
		output, cmdErr := runCommand(ctx, "dpkg", "-S", normalizedPath)
		if cmdErr == nil && strings.TrimSpace(output) != "" {
			return string(InstallMethodAPT)
		}
	}

	if _, err := lookPath("rpm"); err == nil {
		output, cmdErr := runCommand(ctx, "rpm", "-qf", normalizedPath, "--queryformat", "%{NAME}")
		if cmdErr == nil && strings.TrimSpace(output) != "" {
			if _, dnfErr := lookPath("dnf"); dnfErr == nil {
				return string(InstallMethodDNF)
			}
			return string(InstallMethodRPM)
		}
	}

	return ""
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
	return strings.Contains(path, "/node_modules/@compozy/agh/")
}

func isScoopPath(path string) bool {
	return strings.Contains(path, "/scoop/apps/agh/")
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
