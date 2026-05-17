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
	"testing"
)

func TestDetectInstallMethodHomebrew(t *testing.T) {
	restore := stubExecutablePath(t, "/opt/homebrew/Cellar/compozy/1.0.0/bin/compozy")
	defer restore()

	if got := DetectInstallMethod(); got != InstallHomebrew {
		t.Fatalf("expected InstallHomebrew, got %v", got)
	}
}

func TestDetectInstallMethodNPM(t *testing.T) {
	restore := stubExecutablePath(t, "/usr/local/lib/node_modules/@compozy/cli/bin/compozy")
	defer restore()

	if got := DetectInstallMethod(); got != InstallNPM {
		t.Fatalf("expected InstallNPM, got %v", got)
	}
}

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

func TestDetectInstallMethodBinaryFallback(t *testing.T) {
	restore := stubExecutablePath(t, "/usr/local/bin/compozy")
	defer restore()

	if got := DetectInstallMethod(); got != InstallBinary {
		t.Fatalf("expected InstallBinary, got %v", got)
	}
}

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

func TestUpgradeRunsHomebrewCommand(t *testing.T) {
	restore := stubExecutablePath(t, "/opt/homebrew/Cellar/compozy/1.0.0/bin/compozy")
	defer restore()

	var gotCtx context.Context
	var gotCommand managedUpgradeCommand
	restoreRunner := stubManagedUpgradeCommand(
		t,
		func(ctx context.Context, output io.Writer, method InstallMethod) error {
			gotCtx = ctx
			gotCommand = mustManagedUpgradeCommand(t, method)
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
	assertManagedUpgradeCommand(t, gotCommand, "brew", []string{"upgrade", "--cask", "compozy"})
	if got := out.String(); got != "brew upgraded\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestUpgradeRunsNPMCommand(t *testing.T) {
	restore := stubExecutablePath(t, "/usr/local/lib/node_modules/@compozy/cli/bin/compozy")
	defer restore()

	var gotCommand managedUpgradeCommand
	restoreRunner := stubManagedUpgradeCommand(
		t,
		func(_ context.Context, output io.Writer, method InstallMethod) error {
			gotCommand = mustManagedUpgradeCommand(t, method)
			_, err := fmt.Fprintln(output, "npm upgraded")
			return err
		},
	)
	defer restoreRunner()

	var out bytes.Buffer
	if err := Upgrade(context.Background(), "1.0.0", &out); err != nil {
		t.Fatalf("Upgrade returned error: %v", err)
	}
	assertManagedUpgradeCommand(t, gotCommand, "npm", []string{"install", "-g", "@compozy/cli@latest"})
	if got := out.String(); got != "npm upgraded\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestUpgradeRunsGoInstallCommand(t *testing.T) {
	t.Setenv("GOBIN", "")
	goPath := filepath.Join(os.TempDir(), "gopath")
	t.Setenv("GOPATH", goPath)

	restore := stubExecutablePath(t, filepath.Join(goPath, "bin", "compozy"))
	defer restore()

	var gotCommand managedUpgradeCommand
	restoreRunner := stubManagedUpgradeCommand(
		t,
		func(_ context.Context, output io.Writer, method InstallMethod) error {
			gotCommand = mustManagedUpgradeCommand(t, method)
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
	if got := out.String(); got != "go upgraded\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestUpgradeReturnsManagedCommandError(t *testing.T) {
	restore := stubExecutablePath(t, "/usr/local/lib/node_modules/@compozy/cli/bin/compozy")
	defer restore()

	wantErr := errors.New("npm failed")
	restoreRunner := stubManagedUpgradeCommand(
		t,
		func(context.Context, io.Writer, InstallMethod) error {
			return wantErr
		},
	)
	defer restoreRunner()

	if err := Upgrade(context.Background(), "1.0.0", io.Discard); !errors.Is(err, wantErr) {
		t.Fatalf("Upgrade error = %v, want %v", err, wantErr)
	}
}

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

type testContextKey struct{}

func stubManagedUpgradeCommand(
	t *testing.T,
	fn func(context.Context, io.Writer, InstallMethod) error,
) func() {
	t.Helper()

	previous := runManagedUpgradeCommand
	runManagedUpgradeCommand = fn
	return func() {
		runManagedUpgradeCommand = previous
	}
}

func mustManagedUpgradeCommand(t *testing.T, method InstallMethod) managedUpgradeCommand {
	t.Helper()

	command, ok := managedUpgradeCommandForMethod(method)
	if !ok {
		t.Fatalf("missing managed upgrade command for method %v", method)
	}
	return command
}

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
