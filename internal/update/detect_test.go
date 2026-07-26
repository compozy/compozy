package update

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
)

func TestDetectInstallUsesManagedEnvironmentOverride(t *testing.T) {
	t.Run("Should prefer the managed environment override", func(t *testing.T) {
		t.Parallel()

		manager := testManager(t, Config{
			Getenv: func(key string) string {
				if key == ManagedEnvName {
					return "homebrew"
				}
				return ""
			},
		})

		info := manager.detectInstall(context.Background())
		if !info.Managed {
			t.Fatal("detectInstall() managed = false, want true")
		}
		if info.Method != string(InstallMethodHomebrew) {
			t.Fatalf("detectInstall() method = %q, want %q", info.Method, InstallMethodHomebrew)
		}
	})

	t.Run("Should normalize npm managed environment aliases", func(t *testing.T) {
		t.Parallel()

		manager := testManager(t, Config{
			Getenv: func(key string) string {
				if key == ManagedEnvName {
					return "nodejs"
				}
				return ""
			},
		})

		info := manager.detectInstall(context.Background())
		if !info.Managed {
			t.Fatal("detectInstall() managed = false, want true")
		}
		if info.Method != string(InstallMethodNPM) {
			t.Fatalf("detectInstall() method = %q, want %q", info.Method, InstallMethodNPM)
		}
	})
}

func TestDetectInstallRecognizesGoInstallPaths(t *testing.T) {
	t.Run("Should recognize Go-installed binaries under GOPATH/bin", func(t *testing.T) {
		t.Parallel()

		goPath := filepath.Join(t.TempDir(), "gopath")
		manager := testManager(t, Config{
			ExecutablePath: func() (string, error) {
				return filepath.Join(goPath, "bin", "agh"), nil
			},
			Getenv: func(key string) string {
				switch key {
				case "GOPATH":
					return goPath
				case "GOBIN":
					return ""
				default:
					return ""
				}
			},
		})

		info := manager.detectInstall(context.Background())
		if !info.Managed {
			t.Fatal("detectInstall() managed = false, want true")
		}
		if info.Method != string(InstallMethodGoInstall) {
			t.Fatalf("detectInstall() method = %q, want %q", info.Method, InstallMethodGoInstall)
		}
	})
}

func TestDetectInstallFallsBackToDirectBinary(t *testing.T) {
	t.Run("Should fall back to direct-binary when no managed install matches", func(t *testing.T) {
		t.Parallel()

		manager := testManager(t, Config{
			ExecutablePath: func() (string, error) {
				return "/usr/local/bin/agh", nil
			},
		})

		info := manager.detectInstall(context.Background())
		if info.Managed {
			t.Fatal("detectInstall() managed = true, want false")
		}
		if info.Method != string(InstallMethodDirectBinary) {
			t.Fatalf("detectInstall() method = %q, want %q", info.Method, InstallMethodDirectBinary)
		}
	})
}

func TestDetectInstallMemoizesLinuxPackageDetection(t *testing.T) {
	t.Run("Should run Linux package detection only once per manager", func(t *testing.T) {
		t.Parallel()

		var runCalls int
		manager := testManager(t, Config{
			RuntimeOS: runtimeOSLinux,
			ExecutablePath: func() (string, error) {
				return managedPathUsrBin, nil
			},
			LookPath: func(name string) (string, error) {
				if name == "dpkg" {
					return "/usr/bin/dpkg", nil
				}
				return "", os.ErrNotExist
			},
			RunCommand: func(context.Context, string, ...string) (string, error) {
				runCalls++
				return "agh: /usr/bin/agh", nil
			},
		})

		manager.PrimeInstallDetection(context.Background())
		first := manager.detectInstall(context.Background())
		second := manager.detectInstall(context.Background())
		if runCalls != 1 {
			t.Fatalf("runCommand() calls = %d, want 1", runCalls)
		}
		if first != second {
			t.Fatalf("detectInstall() results differ: first=%#v second=%#v", first, second)
		}
	})
}

func testManager(t *testing.T, cfg Config) *Manager {
	t.Helper()

	homePaths, err := aghconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	cfg.HomePaths = homePaths
	cfg.CurrentVersion = "v1.0.0"
	if cfg.ExecutablePath == nil {
		cfg.ExecutablePath = func() (string, error) {
			return filepath.Join(os.TempDir(), "agh"), nil
		}
	}
	if cfg.ResolveSymlinks == nil {
		cfg.ResolveSymlinks = func(path string) (string, error) {
			return path, nil
		}
	}

	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return manager
}
