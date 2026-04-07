package update

import (
	"context"
	"fmt"
	"go/build"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var osExecutable = os.Executable

// InstallMethod identifies how the compozy binary was installed.
type InstallMethod int

const (
	InstallBinary InstallMethod = iota
	InstallHomebrew
	InstallNPM
	InstallGo
)

// DetectInstallMethod determines how the current executable was installed.
func DetectInstallMethod() InstallMethod {
	executablePath, err := osExecutable()
	if err != nil {
		return InstallBinary
	}

	return detectInstallMethod(executablePath, installEnvironment{
		gobin:  os.Getenv("GOBIN"),
		gopath: os.Getenv("GOPATH"),
	})
}

// Upgrade performs the appropriate upgrade flow for the detected install method.
//
// Managed installs print the correct package manager command. Direct binary installs
// perform an in-place self-update.
func Upgrade(ctx context.Context, currentVersion string, stdout io.Writer) error {
	if stdout == nil {
		stdout = io.Discard
	}

	switch DetectInstallMethod() {
	case InstallHomebrew:
		return printManagedUpgradeCommand(stdout, "brew upgrade --cask compozy")
	case InstallNPM:
		return printManagedUpgradeCommand(stdout, "npm install -g @compozy/cli@latest")
	case InstallGo:
		return printManagedUpgradeCommand(stdout, "go install github.com/compozy/compozy/cmd/compozy@latest")
	default:
		client, err := newUpdaterClient()
		if err != nil {
			return err
		}

		latest, err := client.UpdateSelf(ctx, currentVersion)
		if err != nil {
			return err
		}

		newer, err := newerRelease(currentVersion, latest)
		if err != nil {
			return err
		}
		if newer == nil {
			_, writeErr := fmt.Fprintln(stdout, "compozy is already up to date")
			return writeErr
		}

		_, writeErr := fmt.Fprintf(stdout, "Updated compozy to %s\n", newer.Version)
		return writeErr
	}
}

type installEnvironment struct {
	gobin  string
	gopath string
}

func detectInstallMethod(executablePath string, env installEnvironment) InstallMethod {
	normalizedPath := normalizePath(executablePath)

	switch {
	case isHomebrewPath(normalizedPath):
		return InstallHomebrew
	case isNPMPath(normalizedPath):
		return InstallNPM
	case isGoInstallPath(normalizedPath, env):
		return InstallGo
	default:
		return InstallBinary
	}
}

func isHomebrewPath(path string) bool {
	return strings.Contains(path, "/cellar/") || strings.Contains(path, "/caskroom/")
}

func isNPMPath(path string) bool {
	return strings.Contains(path, "/node_modules/")
}

func isGoInstallPath(path string, env installEnvironment) bool {
	for _, candidate := range goBinDirs(env) {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if withinDir(path, normalizePath(candidate)) {
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

func withinDir(path, dir string) bool {
	if dir == "" {
		return false
	}
	if path == dir {
		return true
	}
	return strings.HasPrefix(path, dir+"/")
}

func normalizePath(path string) string {
	cleaned := filepath.Clean(path)
	cleaned = strings.ReplaceAll(cleaned, "\\", "/")
	return strings.ToLower(cleaned)
}

func printManagedUpgradeCommand(stdout io.Writer, command string) error {
	_, err := fmt.Fprintln(stdout, command)
	return err
}
