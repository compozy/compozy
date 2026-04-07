package update

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
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

func TestUpgradePrintsHomebrewCommand(t *testing.T) {
	restore := stubExecutablePath(t, "/opt/homebrew/Cellar/compozy/1.0.0/bin/compozy")
	defer restore()

	var out bytes.Buffer
	if err := Upgrade(context.Background(), "1.0.0", &out); err != nil {
		t.Fatalf("Upgrade returned error: %v", err)
	}
	if got := out.String(); got != "brew upgrade --cask compozy\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestUpgradePrintsNPMCommand(t *testing.T) {
	restore := stubExecutablePath(t, "/usr/local/lib/node_modules/@compozy/cli/bin/compozy")
	defer restore()

	var out bytes.Buffer
	if err := Upgrade(context.Background(), "1.0.0", &out); err != nil {
		t.Fatalf("Upgrade returned error: %v", err)
	}
	if got := out.String(); got != "npm install -g @compozy/cli@latest\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestUpgradePrintsGoInstallCommand(t *testing.T) {
	t.Setenv("GOBIN", "")
	goPath := filepath.Join(os.TempDir(), "gopath")
	t.Setenv("GOPATH", goPath)

	restore := stubExecutablePath(t, filepath.Join(goPath, "bin", "compozy"))
	defer restore()

	var out bytes.Buffer
	if err := Upgrade(context.Background(), "1.0.0", &out); err != nil {
		t.Fatalf("Upgrade returned error: %v", err)
	}
	if got := out.String(); got != "go install github.com/compozy/compozy/cmd/compozy@latest\n" {
		t.Fatalf("unexpected output: %q", got)
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
