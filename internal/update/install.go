package update

import (
	"context"
	"fmt"
	"go/build"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	osExecutable             = os.Executable
	runManagedUpgradeCommand = defaultManagedUpgradeCommand
)

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
// Managed installs run the correct package manager command. Direct binary installs
// perform an in-place self-update.
func Upgrade(ctx context.Context, currentVersion string, stdout io.Writer) error {
	if stdout == nil {
		stdout = io.Discard
	}

	switch DetectInstallMethod() {
	case InstallHomebrew:
		return runManagedUpgradeCommand(ctx, stdout, InstallHomebrew)
	case InstallNPM:
		return runManagedUpgradeCommand(ctx, stdout, InstallNPM)
	case InstallGo:
		return runManagedUpgradeCommand(ctx, stdout, InstallGo)
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

type managedUpgradeCommand struct {
	name string
	args []string
}

func (c managedUpgradeCommand) String() string {
	parts := make([]string, 0, 1+len(c.args))
	parts = append(parts, c.name)
	parts = append(parts, c.args...)
	return strings.Join(parts, " ")
}

func managedUpgradeCommandForMethod(method InstallMethod) (managedUpgradeCommand, bool) {
	switch method {
	case InstallHomebrew:
		return managedUpgradeCommand{name: "brew", args: []string{"upgrade", "--cask", "compozy"}}, true
	case InstallNPM:
		return managedUpgradeCommand{name: "npm", args: []string{"install", "-g", "@compozy/cli@latest"}}, true
	case InstallGo:
		return managedUpgradeCommand{
			name: "go",
			args: []string{"install", "github.com/compozy/compozy/cmd/compozy@latest"},
		}, true
	default:
		return managedUpgradeCommand{}, false
	}
}

func defaultManagedUpgradeCommand(ctx context.Context, output io.Writer, method InstallMethod) error {
	command, ok := managedUpgradeCommandForMethod(method)
	if !ok {
		return fmt.Errorf("unsupported managed install method: %d", method)
	}

	var cmd *exec.Cmd
	switch method {
	case InstallHomebrew:
		cmd = exec.CommandContext(ctx, "brew", "upgrade", "--cask", "compozy")
	case InstallNPM:
		cmd = exec.CommandContext(ctx, "npm", "install", "-g", "@compozy/cli@latest")
	case InstallGo:
		cmd = exec.CommandContext(ctx, "go", "install", "github.com/compozy/compozy/cmd/compozy@latest")
	}

	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s: %w", command.String(), err)
	}
	return nil
}
