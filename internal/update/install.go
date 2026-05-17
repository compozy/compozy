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
	return detectCurrentInstall().method
}

func detectCurrentInstall() installDetails {
	executablePath, err := osExecutable()
	if err != nil {
		return installDetails{method: InstallBinary}
	}

	env := installEnvironment{
		gobin:  os.Getenv("GOBIN"),
		gopath: os.Getenv("GOPATH"),
	}
	return installDetails{
		method:         detectInstallMethod(executablePath, env),
		executablePath: executablePath,
		env:            env,
	}
}

// Upgrade performs the appropriate upgrade flow for the detected install method.
//
// Managed installs run the correct package manager command. Direct binary installs
// perform an in-place self-update.
func Upgrade(ctx context.Context, currentVersion string, stdout io.Writer) error {
	if stdout == nil {
		stdout = io.Discard
	}

	install := detectCurrentInstall()
	switch install.method {
	case InstallHomebrew:
		return runManagedUpgradeCommand(ctx, stdout, install)
	case InstallNPM:
		return runManagedUpgradeCommand(ctx, stdout, install)
	case InstallGo:
		return runManagedUpgradeCommand(ctx, stdout, install)
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

type installDetails struct {
	method         InstallMethod
	executablePath string
	env            installEnvironment
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
	_, ok := npmGlobalPrefix(path)
	return ok
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
	name       string
	args       []string
	pathPrefix string
	env        []string
}

func (c managedUpgradeCommand) String() string {
	parts := make([]string, 0, 1+len(c.args))
	parts = append(parts, c.name)
	parts = append(parts, c.args...)
	return strings.Join(parts, " ")
}

func managedUpgradeCommandForInstall(install installDetails) (managedUpgradeCommand, error) {
	switch install.method {
	case InstallHomebrew:
		prefix, ok := homebrewPrefix(install.executablePath)
		if !ok {
			return managedUpgradeCommand{}, fmt.Errorf("detect homebrew prefix from %q", install.executablePath)
		}
		return managedUpgradeCommand{
			name:       "brew",
			args:       []string{"upgrade", "--cask", "compozy"},
			pathPrefix: filepath.Join(prefix, "bin"),
		}, nil
	case InstallNPM:
		prefix, ok := npmGlobalPrefix(install.executablePath)
		if !ok {
			return managedUpgradeCommand{}, fmt.Errorf("detect npm prefix from %q", install.executablePath)
		}
		return managedUpgradeCommand{
			name:       "npm",
			args:       []string{"install", "-g", "@compozy/cli@latest"},
			pathPrefix: filepath.Join(prefix, "bin"),
			env:        []string{"NPM_CONFIG_PREFIX=" + prefix},
		}, nil
	case InstallGo:
		binDir, ok := goInstallBinDir(install.executablePath, install.env)
		if !ok {
			return managedUpgradeCommand{}, fmt.Errorf("detect go install bin from %q", install.executablePath)
		}
		return managedUpgradeCommand{
			name: "go",
			args: []string{"install", "github.com/compozy/compozy/cmd/compozy@latest"},
			env:  []string{"GOBIN=" + binDir},
		}, nil
	default:
		return managedUpgradeCommand{}, fmt.Errorf("unsupported managed install method: %d", install.method)
	}
}

func defaultManagedUpgradeCommand(ctx context.Context, output io.Writer, install installDetails) error {
	command, err := managedUpgradeCommandForInstall(install)
	if err != nil {
		return err
	}
	if err := requireManagedExecutable(command); err != nil {
		return err
	}

	var cmd *exec.Cmd
	switch install.method {
	case InstallHomebrew:
		cmd = exec.CommandContext(ctx, "brew", "upgrade", "--cask", "compozy")
	case InstallNPM:
		cmd = exec.CommandContext(ctx, "npm", "install", "-g", "@compozy/cli@latest")
	case InstallGo:
		cmd = exec.CommandContext(ctx, "go", "install", "github.com/compozy/compozy/cmd/compozy@latest")
	}

	cmd.Env = managedCommandEnv(os.Environ(), command)
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s: %w", command.String(), err)
	}
	return nil
}

func requireManagedExecutable(command managedUpgradeCommand) error {
	if command.pathPrefix == "" {
		return nil
	}
	path := filepath.Join(command.pathPrefix, command.name)
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf(
			"detected install expects %s at %s; refusing to run ambient %s from PATH: %w",
			command.name,
			path,
			command.name,
			err,
		)
	}
	if info.IsDir() || info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("detected install expects executable %s at %s", command.name, path)
	}
	return nil
}

func managedCommandEnv(base []string, command managedUpgradeCommand) []string {
	env := append([]string(nil), base...)
	if command.pathPrefix != "" {
		env = prependEnvPath(env, command.pathPrefix)
	}
	for _, item := range command.env {
		env = upsertEnv(env, item)
	}
	return env
}

func prependEnvPath(env []string, dir string) []string {
	if strings.TrimSpace(dir) == "" {
		return env
	}
	pathValue := dir
	if existing, ok := lookupEnvEntry(env, "PATH"); ok && strings.TrimSpace(existing) != "" {
		pathValue = dir + string(os.PathListSeparator) + existing
	}
	return upsertEnv(env, "PATH="+pathValue)
}

func upsertEnv(env []string, item string) []string {
	key, _, ok := strings.Cut(item, "=")
	if !ok {
		return env
	}
	for i, existing := range env {
		if existingKey, _, existingOK := strings.Cut(existing, "="); existingOK && existingKey == key {
			env[i] = item
			return env
		}
	}
	return append(env, item)
}

func lookupEnvEntry(env []string, key string) (string, bool) {
	for _, item := range env {
		existingKey, value, ok := strings.Cut(item, "=")
		if ok && existingKey == key {
			return value, true
		}
	}
	return "", false
}

func homebrewPrefix(path string) (string, bool) {
	return prefixBeforeAnyPathSegment(path, "/Cellar/", "/Caskroom/")
}

func npmGlobalPrefix(path string) (string, bool) {
	return prefixBeforeAnyPathSegment(path, "/lib/node_modules/")
}

func prefixBeforeAnyPathSegment(path string, markers ...string) (string, bool) {
	cleaned := filepath.Clean(path)
	normalized := strings.ReplaceAll(cleaned, "\\", "/")
	normalizedLower := strings.ToLower(normalized)
	for _, marker := range markers {
		index := strings.Index(normalizedLower, strings.ToLower(marker))
		if index > 0 {
			return normalized[:index], true
		}
	}
	return "", false
}

func goInstallBinDir(path string, env installEnvironment) (string, bool) {
	normalizedPath := normalizePath(path)
	for _, candidate := range goBinDirs(env) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		normalizedCandidate := normalizePath(candidate)
		if withinDir(normalizedPath, normalizedCandidate) {
			return candidate, true
		}
	}
	return "", false
}
