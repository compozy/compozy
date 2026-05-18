package update

import (
	"context"
	"errors"
	"fmt"
	"go/build"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	osExecutable             = os.Executable
	runManagedUpgradeCommand = defaultManagedUpgradeCommand
)

const (
	goosWindows         = "windows"
	homebrewFormulaName = "compozy/compozy/compozy"
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

// detectCurrentInstall captures the executable path and environment used to classify the current install.
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

// installDetails carries the detected install method plus inputs needed to rebuild its upgrade command.
type installDetails struct {
	method         InstallMethod
	executablePath string
	env            installEnvironment
}

// installEnvironment keeps environment-derived install roots separate from direct os lookups.
type installEnvironment struct {
	gobin  string
	gopath string
}

// detectInstallMethod classifies an executable path against known managed install layouts.
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

// isHomebrewPath reports whether the normalized path belongs to a Homebrew cellar or caskroom.
func isHomebrewPath(path string) bool {
	return strings.Contains(path, "/cellar/") || strings.Contains(path, "/caskroom/")
}

// isNPMPath reports whether the normalized path belongs to an npm global package layout.
func isNPMPath(path string) bool {
	_, ok := npmInstallLocation(path)
	return ok
}

// isGoInstallPath reports whether the normalized path is under a configured Go bin directory.
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

// goBinDirs returns all candidate Go bin directories from GOBIN and GOPATH.
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

// withinDir reports whether path is the directory itself or a descendant of it.
func withinDir(path, dir string) bool {
	if dir == "" {
		return false
	}
	if path == dir {
		return true
	}
	return strings.HasPrefix(path, dir+"/")
}

// normalizePath makes install layout checks stable across separators and case.
func normalizePath(path string) string {
	cleaned := filepath.Clean(path)
	cleaned = strings.ReplaceAll(cleaned, "\\", "/")
	return strings.ToLower(cleaned)
}

// managedUpgradeCommand describes the package-manager command for a managed install.
type managedUpgradeCommand struct {
	name       string
	args       []string
	pathPrefix string
	env        []string
}

// npmLocation records the global npm prefix and the bin directory that owns npm.
type npmLocation struct {
	prefix string
	binDir string
}

// String renders the command in the same shape users expect to see in errors.
func (c managedUpgradeCommand) String() string {
	parts := make([]string, 0, 1+len(c.args))
	parts = append(parts, c.name)
	parts = append(parts, c.args...)
	return strings.Join(parts, " ")
}

// managedUpgradeCommandForInstall builds the upgrade command for a managed install method.
func managedUpgradeCommandForInstall(install installDetails) (managedUpgradeCommand, error) {
	switch install.method {
	case InstallHomebrew:
		prefix, ok := homebrewPrefix(install.executablePath)
		if !ok {
			return managedUpgradeCommand{}, fmt.Errorf("detect homebrew prefix from %q", install.executablePath)
		}
		return managedUpgradeCommand{
			name:       "brew",
			args:       []string{"upgrade", homebrewFormulaName},
			pathPrefix: filepath.Join(prefix, "bin"),
		}, nil
	case InstallNPM:
		location, ok := npmInstallLocation(install.executablePath)
		if !ok {
			return managedUpgradeCommand{}, fmt.Errorf("detect npm prefix from %q", install.executablePath)
		}
		return managedUpgradeCommand{
			name:       "npm",
			args:       []string{"install", "-g", "@compozy/cli@latest"},
			pathPrefix: location.binDir,
			env:        []string{"NPM_CONFIG_PREFIX=" + location.prefix},
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

// defaultManagedUpgradeCommand runs the detected package manager with isolated path and env settings.
func defaultManagedUpgradeCommand(ctx context.Context, output io.Writer, install installDetails) error {
	command, err := managedUpgradeCommandForInstall(install)
	if err != nil {
		return err
	}
	executablePath, err := managedExecutablePath(command)
	if err != nil {
		return err
	}

	var cmd *exec.Cmd
	switch install.method {
	case InstallHomebrew:
		cmd = exec.CommandContext(ctx, "brew", "upgrade", homebrewFormulaName)
	case InstallNPM:
		cmd = exec.CommandContext(ctx, "npm", "install", "-g", "@compozy/cli@latest")
	case InstallGo:
		cmd = exec.CommandContext(ctx, "go", "install", "github.com/compozy/compozy/cmd/compozy@latest")
	}

	if executablePath != "" {
		cmd.Path = executablePath
		cmd.Err = nil
	}
	cmd.Env = managedCommandEnv(os.Environ(), command)
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s: %w", command.String(), err)
	}
	return nil
}

// managedExecutablePath resolves the package manager executable from the detected install prefix.
func managedExecutablePath(command managedUpgradeCommand) (string, error) {
	if command.pathPrefix == "" {
		return "", nil
	}
	for _, name := range managedExecutableNames(command.name) {
		path := filepath.Join(command.pathPrefix, name)
		info, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return "", fmt.Errorf("stat detected %s executable at %s: %w", command.name, path, err)
		}
		if info.IsDir() {
			continue
		}
		if runtime.GOOS != goosWindows && info.Mode().Perm()&0o111 == 0 {
			continue
		}
		return path, nil
	}
	return "", fmt.Errorf(
		"detected install expects executable %s under %s; refusing to run ambient %s from PATH",
		command.name,
		command.pathPrefix,
		command.name,
	)
}

// managedExecutableNames returns platform-specific executable names for managed tools.
func managedExecutableNames(name string) []string {
	if runtime.GOOS != goosWindows {
		return []string{name}
	}
	switch name {
	case "npm":
		return []string{"npm.cmd", "npm.exe", "npm"}
	default:
		return []string{name + ".exe", name}
	}
}

// managedCommandEnv composes the child process environment for a managed upgrade command.
func managedCommandEnv(base []string, command managedUpgradeCommand) []string {
	return managedCommandEnvForOS(base, command, runtime.GOOS)
}

// managedCommandEnvForOS composes command env while honoring platform-specific env key semantics.
func managedCommandEnvForOS(base []string, command managedUpgradeCommand, goos string) []string {
	env := append([]string(nil), base...)
	if command.pathPrefix != "" {
		env = prependEnvPathForOS(env, command.pathPrefix, goos)
	}
	for _, item := range command.env {
		env = upsertEnvForOS(env, item, goos)
	}
	return env
}

// prependEnvPathForOS prepends a directory to PATH using the requested platform's env rules.
func prependEnvPathForOS(env []string, dir string, goos string) []string {
	if strings.TrimSpace(dir) == "" {
		return env
	}
	pathValue := dir
	if existing, ok := lookupEnvEntryForOS(env, "PATH", goos); ok && strings.TrimSpace(existing) != "" {
		pathValue = dir + pathListSeparator(goos) + existing
	}
	return upsertEnvForOS(env, "PATH="+pathValue, goos)
}

// upsertEnvForOS replaces an environment entry using platform-specific key matching.
func upsertEnvForOS(env []string, item string, goos string) []string {
	key, _, ok := strings.Cut(item, "=")
	if !ok {
		return env
	}
	for i, existing := range env {
		existingKey, _, existingOK := strings.Cut(existing, "=")
		if existingOK && envKeyMatches(existingKey, key, goos) {
			env[i] = item
			return env
		}
	}
	return append(env, item)
}

// lookupEnvEntryForOS returns an environment value using platform-specific key matching.
func lookupEnvEntryForOS(env []string, key string, goos string) (string, bool) {
	for _, item := range env {
		existingKey, value, ok := strings.Cut(item, "=")
		if ok && envKeyMatches(existingKey, key, goos) {
			return value, true
		}
	}
	return "", false
}

// envKeyMatches compares environment keys using Windows' case-insensitive semantics when needed.
func envKeyMatches(existingKey string, key string, goos string) bool {
	if goos == goosWindows {
		return strings.EqualFold(existingKey, key)
	}
	return existingKey == key
}

// pathListSeparator returns the platform-specific separator used in PATH-like variables.
func pathListSeparator(goos string) string {
	if goos == goosWindows {
		return ";"
	}
	return string(os.PathListSeparator)
}

// homebrewPrefix extracts the Homebrew root from a Homebrew executable path.
func homebrewPrefix(path string) (string, bool) {
	return prefixBeforeAnyPathSegment(path, "/Cellar/", "/Caskroom/")
}

// npmInstallLocation extracts npm global prefix details from a package executable path.
func npmInstallLocation(path string) (npmLocation, bool) {
	if prefix, ok := prefixBeforeAnyPathSegment(path, "/lib/node_modules/"); ok {
		return npmLocation{prefix: prefix, binDir: filepath.Join(prefix, "bin")}, true
	}
	if prefix, ok := prefixBeforeAnyPathSegment(path, "/node_modules/"); ok {
		return npmLocation{prefix: prefix, binDir: prefix}, true
	}
	return npmLocation{}, false
}

// prefixBeforeAnyPathSegment returns the path prefix before the first matching marker.
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

// goInstallBinDir returns the Go bin directory that owns the current executable.
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
