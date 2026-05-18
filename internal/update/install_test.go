package update

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestDetectInstallMethodHomebrew verifies Homebrew cellar paths are detected as Homebrew installs.
func TestDetectInstallMethodHomebrew(t *testing.T) {
	restore := stubExecutablePath(t, "/opt/homebrew/Cellar/compozy/1.0.0/bin/compozy")
	defer restore()

	if got := DetectInstallMethod(); got != InstallHomebrew {
		t.Fatalf("expected InstallHomebrew, got %v", got)
	}
}

// TestDetectInstallMethodNPM verifies standard npm global package paths are detected as npm installs.
func TestDetectInstallMethodNPM(t *testing.T) {
	restore := stubExecutablePath(t, "/usr/local/lib/node_modules/@compozy/cli/bin/compozy")
	defer restore()

	if got := DetectInstallMethod(); got != InstallNPM {
		t.Fatalf("expected InstallNPM, got %v", got)
	}
}

// TestDetectInstallMethodNPMWindowsGlobalLayout verifies Windows npm global paths are npm installs.
func TestDetectInstallMethodNPMWindowsGlobalLayout(t *testing.T) {
	t.Run("Should detect NPM on Windows global layout", func(t *testing.T) {
		restore := stubExecutablePath(
			t,
			`C:\Users\matheus\AppData\Roaming\npm\node_modules\@compozy\cli\bin\compozy.exe`,
		)
		defer restore()

		if got := DetectInstallMethod(); got != InstallNPM {
			t.Fatalf("expected InstallNPM, got %v", got)
		}
	})
}

// TestDetectInstallMethodGo verifies binaries under GOBIN or GOPATH bin are detected as Go installs.
func TestDetectInstallMethodGo(t *testing.T) {
	t.Setenv("GOBIN", "")
	goPath := filepath.Join(os.TempDir(), "gopath")
	t.Setenv("GOPATH", goPath)

	restore := stubExecutablePath(t, filepath.Join(goPath, "bin", "compozy"))
	defer restore()

	if got := DetectInstallMethod(); got != InstallGo {
		t.Fatalf("expected InstallGo, got %v", got)
	}
}

// TestDetectInstallMethodBinaryFallback verifies unknown executable paths use the self-updater path.
func TestDetectInstallMethodBinaryFallback(t *testing.T) {
	restore := stubExecutablePath(t, "/usr/local/bin/compozy")
	defer restore()

	if got := DetectInstallMethod(); got != InstallBinary {
		t.Fatalf("expected InstallBinary, got %v", got)
	}
}

// TestDetectInstallMethodFallsBackToBinaryWhenExecutableLookupFails covers executable lookup errors.
func TestDetectInstallMethodFallsBackToBinaryWhenExecutableLookupFails(t *testing.T) {
	previous := osExecutable
	osExecutable = func() (string, error) {
		return "", context.Canceled
	}
	defer func() {
		osExecutable = previous
	}()

	if got := DetectInstallMethod(); got != InstallBinary {
		t.Fatalf("expected InstallBinary, got %v", got)
	}
}

// TestUpgradeRunsHomebrewCommand verifies Homebrew installs dispatch to the managed brew command.
func TestUpgradeRunsHomebrewCommand(t *testing.T) {
	restore := stubExecutablePath(t, "/opt/homebrew/Cellar/compozy/1.0.0/bin/compozy")
	defer restore()

	var gotCtx context.Context
	var gotCommand managedUpgradeCommand
	restoreRunner := stubManagedUpgradeCommand(
		t,
		func(ctx context.Context, output io.Writer, install installDetails) error {
			gotCtx = ctx
			gotCommand = mustManagedUpgradeCommand(t, install)
			_, err := fmt.Fprintln(output, "brew upgraded")
			return err
		},
	)
	defer restoreRunner()

	ctx := context.WithValue(context.Background(), testContextKey{}, "upgrade")
	var out bytes.Buffer
	if err := Upgrade(ctx, "1.0.0", &out); err != nil {
		t.Fatalf("Upgrade returned error: %v", err)
	}
	if gotCtx != ctx {
		t.Fatalf("managed command context = %#v, want caller context", gotCtx)
	}
	assertManagedUpgradeCommand(t, gotCommand, "brew", []string{"upgrade", homebrewFormulaName})
	if got := out.String(); got != "brew upgraded\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}

// TestUpgradeRunsNPMCommand verifies npm installs dispatch to npm with the detected prefix.
func TestUpgradeRunsNPMCommand(t *testing.T) {
	restore := stubExecutablePath(t, "/usr/local/lib/node_modules/@compozy/cli/bin/compozy")
	defer restore()

	var gotCommand managedUpgradeCommand
	restoreRunner := stubManagedUpgradeCommand(
		t,
		func(_ context.Context, output io.Writer, install installDetails) error {
			gotCommand = mustManagedUpgradeCommand(t, install)
			_, err := fmt.Fprintln(output, "npm upgraded")
			return err
		},
	)
	defer restoreRunner()

	var out bytes.Buffer
	if err := Upgrade(context.Background(), "1.0.0", &out); err != nil {
		t.Fatalf("Upgrade returned error: %v", err)
	}
	assertManagedUpgradeCommand(t, gotCommand, "npm", []string{
		"install",
		"-g",
		"@compozy/cli@latest",
	})
	if gotCommand.pathPrefix != "/usr/local/bin" {
		t.Fatalf("path prefix = %q, want /usr/local/bin", gotCommand.pathPrefix)
	}
	if !slices.Contains(gotCommand.env, "NPM_CONFIG_PREFIX=/usr/local") {
		t.Fatalf("command env = %#v, want NPM_CONFIG_PREFIX=/usr/local", gotCommand.env)
	}
	if got := out.String(); got != "npm upgraded\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}

// TestNPMCommandUsesWindowsGlobalPrefixLayout verifies Windows npm prefixes are derived correctly.
func TestNPMCommandUsesWindowsGlobalPrefixLayout(t *testing.T) {
	command, err := managedUpgradeCommandForInstall(installDetails{
		method:         InstallNPM,
		executablePath: `C:\Users\matheus\AppData\Roaming\npm\node_modules\@compozy\cli\bin\compozy.exe`,
	})
	if err != nil {
		t.Fatalf("managedUpgradeCommandForInstall returned error: %v", err)
	}

	wantPrefix := "C:/Users/matheus/AppData/Roaming/npm"
	assertManagedUpgradeCommand(t, command, "npm", []string{
		"install",
		"-g",
		"@compozy/cli@latest",
	})
	if command.pathPrefix != wantPrefix {
		t.Fatalf("path prefix = %q, want %q", command.pathPrefix, wantPrefix)
	}
	if !slices.Contains(command.env, "NPM_CONFIG_PREFIX="+wantPrefix) {
		t.Fatalf("command env = %#v, want NPM_CONFIG_PREFIX=%s", command.env, wantPrefix)
	}
}

// TestManagedCommandEnvMatchesWindowsPathCaseInsensitively covers Windows env key matching.
func TestManagedCommandEnvMatchesWindowsPathCaseInsensitively(t *testing.T) {
	base := []string{
		`Path=C:\Windows\System32`,
		`npm_config_prefix=C:\old-prefix`,
	}
	command := managedUpgradeCommand{
		pathPrefix: `C:\Users\matheus\AppData\Roaming\npm`,
		env:        []string{`NPM_CONFIG_PREFIX=C:\Users\matheus\AppData\Roaming\npm`},
	}

	got := managedCommandEnvForOS(base, command, goosWindows)
	wantPath := `PATH=C:\Users\matheus\AppData\Roaming\npm;C:\Windows\System32`
	wantPrefix := `NPM_CONFIG_PREFIX=C:\Users\matheus\AppData\Roaming\npm`

	if !slices.Contains(got, wantPath) {
		t.Fatalf("command env = %#v, want %q", got, wantPath)
	}
	if !slices.Contains(got, wantPrefix) {
		t.Fatalf("command env = %#v, want %q", got, wantPrefix)
	}
	if slices.Contains(got, `Path=C:\Windows\System32`) {
		t.Fatalf("command env kept duplicate Windows Path entry: %#v", got)
	}
	if slices.Contains(got, `npm_config_prefix=C:\old-prefix`) {
		t.Fatalf("command env kept duplicate Windows npm prefix entry: %#v", got)
	}
}

// TestManagedCommandEnvKeepsNonWindowsPathCaseSensitive covers POSIX env key matching.
func TestManagedCommandEnvKeepsNonWindowsPathCaseSensitive(t *testing.T) {
	base := []string{"Path=/ambient/bin"}
	command := managedUpgradeCommand{pathPrefix: "/managed/bin"}

	got := managedCommandEnvForOS(base, command, "linux")
	wantPath := "PATH=/managed/bin"

	if !slices.Contains(got, wantPath) {
		t.Fatalf("command env = %#v, want %q", got, wantPath)
	}
	if !slices.Contains(got, "Path=/ambient/bin") {
		t.Fatalf("command env dropped case-distinct Path entry: %#v", got)
	}
}

// TestUpgradeRunsGoInstallCommand verifies Go installs dispatch to go install with GOBIN pinned.
func TestUpgradeRunsGoInstallCommand(t *testing.T) {
	t.Setenv("GOBIN", "")
	goPath := filepath.Join(os.TempDir(), "gopath")
	t.Setenv("GOPATH", goPath)

	restore := stubExecutablePath(t, filepath.Join(goPath, "bin", "compozy"))
	defer restore()

	var gotCommand managedUpgradeCommand
	restoreRunner := stubManagedUpgradeCommand(
		t,
		func(_ context.Context, output io.Writer, install installDetails) error {
			gotCommand = mustManagedUpgradeCommand(t, install)
			_, err := fmt.Fprintln(output, "go upgraded")
			return err
		},
	)
	defer restoreRunner()

	var out bytes.Buffer
	if err := Upgrade(context.Background(), "1.0.0", &out); err != nil {
		t.Fatalf("Upgrade returned error: %v", err)
	}
	assertManagedUpgradeCommand(t, gotCommand, "go", []string{
		"install",
		"github.com/compozy/compozy/cmd/compozy@latest",
	})
	wantGoBin := "GOBIN=" + filepath.Join(goPath, "bin")
	if !slices.Contains(gotCommand.env, wantGoBin) {
		t.Fatalf("command env = %#v, want %q", gotCommand.env, wantGoBin)
	}
	if got := out.String(); got != "go upgraded\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}

// TestUpgradeReturnsManagedCommandError verifies managed command failures are returned to callers.
func TestUpgradeReturnsManagedCommandError(t *testing.T) {
	restore := stubExecutablePath(t, "/usr/local/lib/node_modules/@compozy/cli/bin/compozy")
	defer restore()

	wantErr := errors.New("npm failed")
	restoreRunner := stubManagedUpgradeCommand(
		t,
		func(context.Context, io.Writer, installDetails) error {
			return wantErr
		},
	)
	defer restoreRunner()

	if err := Upgrade(context.Background(), "1.0.0", io.Discard); !errors.Is(err, wantErr) {
		t.Fatalf("Upgrade error = %v, want %v", err, wantErr)
	}
}

// TestUpgradeRejectsNPMWhenDetectedPrefixDoesNotOwnNPM prevents ambient npm fallback.
func TestUpgradeRejectsNPMWhenDetectedPrefixDoesNotOwnNPM(t *testing.T) {
	prefix := filepath.Join(t.TempDir(), "detected-prefix")
	restore := stubExecutablePath(t, filepath.Join(prefix, "lib", "node_modules", "@compozy", "cli", "bin", "compozy"))
	defer restore()

	ambientBin := filepath.Join(t.TempDir(), "ambient-bin")
	if err := os.MkdirAll(ambientBin, 0o755); err != nil {
		t.Fatalf("create ambient bin: %v", err)
	}
	marker := filepath.Join(t.TempDir(), "ambient-npm-ran")
	ambientNPM := filepath.Join(ambientBin, "npm")
	if err := writeFile(ambientNPM, []byte("#!/bin/sh\n: > '"+marker+"'\n"), 0o755); err != nil {
		t.Fatalf("write ambient npm: %v", err)
	}
	t.Setenv("PATH", ambientBin)

	err := Upgrade(context.Background(), "1.0.0", io.Discard)
	if err == nil {
		t.Fatal("expected missing detected-prefix npm error")
	}
	if !strings.Contains(err.Error(), filepath.Join(prefix, "bin")) {
		t.Fatalf("Upgrade error = %v, want detected prefix npm path", err)
	}
	if _, statErr := os.Stat(marker); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("ambient npm ran unexpectedly, stat error = %v", statErr)
	}
}

// TestUpgradeRunsDetectedNPMWhenAmbientNPMIsFirst verifies detected npm wins over PATH order.
func TestUpgradeRunsDetectedNPMWhenAmbientNPMIsFirst(t *testing.T) {
	prefix := filepath.Join(t.TempDir(), "detected-prefix")
	restore := stubExecutablePath(t, filepath.Join(prefix, "lib", "node_modules", "@compozy", "cli", "bin", "compozy"))
	defer restore()

	detectedBin := filepath.Join(prefix, "bin")
	if err := os.MkdirAll(detectedBin, 0o755); err != nil {
		t.Fatalf("create detected bin: %v", err)
	}
	detectedMarker := filepath.Join(t.TempDir(), "detected-npm-ran")
	detectedNPM := filepath.Join(detectedBin, "npm")
	if err := writeFile(detectedNPM, []byte("#!/bin/sh\n: > '"+detectedMarker+"'\n"), 0o755); err != nil {
		t.Fatalf("write detected npm: %v", err)
	}

	ambientBin := filepath.Join(t.TempDir(), "ambient-bin")
	if err := os.MkdirAll(ambientBin, 0o755); err != nil {
		t.Fatalf("create ambient bin: %v", err)
	}
	ambientMarker := filepath.Join(t.TempDir(), "ambient-npm-ran")
	ambientNPM := filepath.Join(ambientBin, "npm")
	if err := writeFile(ambientNPM, []byte("#!/bin/sh\n: > '"+ambientMarker+"'\n"), 0o755); err != nil {
		t.Fatalf("write ambient npm: %v", err)
	}
	t.Setenv("PATH", ambientBin)

	if err := Upgrade(context.Background(), "1.0.0", io.Discard); err != nil {
		t.Fatalf("Upgrade returned error: %v", err)
	}
	if _, err := os.Stat(detectedMarker); err != nil {
		t.Fatalf("detected npm did not run: %v", err)
	}
	if _, err := os.Stat(ambientMarker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ambient npm ran unexpectedly, stat error = %v", err)
	}
}

// TestUpgradeBinaryInstallUsesSelfUpdater verifies direct binary installs use the self-updater.
func TestUpgradeBinaryInstallUsesSelfUpdater(t *testing.T) {
	restoreExe := stubExecutablePath(t, "/usr/local/bin/compozy")
	defer restoreExe()

	restoreUpdater := stubUpdaterClient(t, fakeUpdaterClient{
		updateSelfFn: func(_ context.Context, currentVersion string) (*ReleaseInfo, error) {
			if currentVersion != "1.0.0" {
				t.Fatalf("unexpected current version: %q", currentVersion)
			}
			return &ReleaseInfo{Version: "1.1.0"}, nil
		},
	})
	defer restoreUpdater()

	var out bytes.Buffer
	if err := Upgrade(context.Background(), "1.0.0", &out); err != nil {
		t.Fatalf("Upgrade returned error: %v", err)
	}
	if got := out.String(); got != "Updated compozy to 1.1.0\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}

// TestUpgradeBinaryInstallReportsAlreadyUpToDate verifies already-current binary installs report cleanly.
func TestUpgradeBinaryInstallReportsAlreadyUpToDate(t *testing.T) {
	restoreExe := stubExecutablePath(t, "/usr/local/bin/compozy")
	defer restoreExe()

	restoreUpdater := stubUpdaterClient(t, fakeUpdaterClient{
		updateSelfFn: func(context.Context, string) (*ReleaseInfo, error) {
			return &ReleaseInfo{Version: "1.0.0"}, nil
		},
	})
	defer restoreUpdater()

	var out bytes.Buffer
	if err := Upgrade(context.Background(), "1.0.0", &out); err != nil {
		t.Fatalf("Upgrade returned error: %v", err)
	}
	if got := out.String(); got != "compozy is already up to date\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}

// stubExecutablePath replaces os.Executable with a deterministic path for install detection tests.
func stubExecutablePath(t *testing.T, executablePath string) func() {
	t.Helper()

	previous := osExecutable
	osExecutable = func() (string, error) {
		return executablePath, nil
	}
	return func() {
		osExecutable = previous
	}
}

// testContextKey provides an identity-only context key for context propagation assertions.
type testContextKey struct{}

// stubManagedUpgradeCommand replaces managed command execution with a test callback.
func stubManagedUpgradeCommand(
	t *testing.T,
	fn func(context.Context, io.Writer, installDetails) error,
) func() {
	t.Helper()

	previous := runManagedUpgradeCommand
	runManagedUpgradeCommand = fn
	return func() {
		runManagedUpgradeCommand = previous
	}
}

// mustManagedUpgradeCommand builds a managed command or fails the current test.
func mustManagedUpgradeCommand(t *testing.T, install installDetails) managedUpgradeCommand {
	t.Helper()

	command, err := managedUpgradeCommandForInstall(install)
	if err != nil {
		t.Fatalf("managed upgrade command: %v", err)
	}
	return command
}

// assertManagedUpgradeCommand checks the command name and arguments.
func assertManagedUpgradeCommand(
	t *testing.T,
	got managedUpgradeCommand,
	wantName string,
	wantArgs []string,
) {
	t.Helper()

	if got.name != wantName {
		t.Fatalf("command name = %q, want %q", got.name, wantName)
	}
	if !slices.Equal(got.args, wantArgs) {
		t.Fatalf("command args = %#v, want %#v", got.args, wantArgs)
	}
}
