package update

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"testing/synctest"

	compozyconfig "github.com/compozy/compozy/internal/config"
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

		info, err := manager.detectInstall(t.Context())
		if err != nil {
			t.Fatalf("detectInstall() error = %v", err)
		}
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

		info, err := manager.detectInstall(t.Context())
		if err != nil {
			t.Fatalf("detectInstall() error = %v", err)
		}
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
				return filepath.Join(goPath, "bin", "compozy"), nil
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

		info, err := manager.detectInstall(t.Context())
		if err != nil {
			t.Fatalf("detectInstall() error = %v", err)
		}
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
				return "/usr/local/bin/compozy", nil
			},
		})

		info, err := manager.detectInstall(t.Context())
		if err != nil {
			t.Fatalf("detectInstall() error = %v", err)
		}
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
				return "compozy: /usr/bin/compozy", nil
			},
		})

		if err := manager.PrimeInstallDetection(t.Context()); err != nil {
			t.Fatalf("PrimeInstallDetection() error = %v", err)
		}
		first, err := manager.detectInstall(t.Context())
		if err != nil {
			t.Fatalf("detectInstall(first) error = %v", err)
		}
		second, err := manager.detectInstall(t.Context())
		if err != nil {
			t.Fatalf("detectInstall(second) error = %v", err)
		}
		if runCalls != 1 {
			t.Fatalf("runCommand() calls = %d, want 1", runCalls)
		}
		if first != second {
			t.Fatalf("detectInstall() results differ: first=%#v second=%#v", first, second)
		}
	})

	t.Run("Should not memoize caller cancellation as direct-binary detection", func(t *testing.T) {
		t.Parallel()

		var runCalls int
		var cancelFirst context.CancelFunc
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
				if runCalls == 1 {
					cancelFirst()
				}
				return "compozy: /usr/bin/compozy", nil
			},
		})

		canceledCtx, cancel := context.WithCancel(t.Context())
		cancelFirst = cancel
		defer cancel()
		if _, err := manager.detectInstall(canceledCtx); !errors.Is(err, context.Canceled) {
			t.Fatalf("detectInstall(canceled) error = %v, want context.Canceled", err)
		}
		info, err := manager.detectInstall(t.Context())
		if err != nil {
			t.Fatalf("detectInstall(retry) error = %v", err)
		}
		if info.Method != string(InstallMethodAPT) || !info.Managed {
			t.Fatalf("detectInstall(retry) = %#v, want managed apt", info)
		}
		if runCalls != 2 {
			t.Fatalf("runCommand() calls = %d, want canceled attempt plus successful retry", runCalls)
		}
	})

	t.Run("Should share one concurrent detection per manager", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			var runCalls atomic.Int64
			probeStarted := make(chan struct{})
			releaseProbe := make(chan struct{})
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
					if runCalls.Add(1) == 1 {
						close(probeStarted)
					}
					<-releaseProbe
					return "compozy: /usr/bin/compozy", nil
				},
			})

			results := make(chan installDetectionTestResult, 2)
			for range 2 {
				go func() {
					info, err := manager.detectInstall(t.Context())
					results <- installDetectionTestResult{info: info, err: err}
				}()
			}

			<-probeStarted
			synctest.Wait()
			if got := runCalls.Load(); got != 1 {
				close(releaseProbe)
				synctest.Wait()
				t.Fatalf("concurrent runCommand() calls = %d, want 1", got)
			}

			close(releaseProbe)
			for range 2 {
				result := <-results
				if result.err != nil {
					t.Fatalf("detectInstall() error = %v", result.err)
				}
				if result.info.Method != string(InstallMethodAPT) || !result.info.Managed {
					t.Fatalf("detectInstall() = %#v, want managed apt", result.info)
				}
			}

			if _, err := manager.detectInstall(t.Context()); err != nil {
				t.Fatalf("detectInstall(cached) error = %v", err)
			}
			if got := runCalls.Load(); got != 1 {
				t.Fatalf("runCommand() calls after cached lookup = %d, want 1", got)
			}
		})
	})

	t.Run("Should let a waiter cancel while the shared owner continues", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			var runCalls atomic.Int64
			probeStarted := make(chan struct{})
			releaseProbe := make(chan struct{})
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
					if runCalls.Add(1) == 1 {
						close(probeStarted)
					}
					<-releaseProbe
					return "compozy: /usr/bin/compozy", nil
				},
			})

			ownerResult := make(chan installDetectionTestResult, 1)
			go func() {
				info, err := manager.detectInstall(t.Context())
				ownerResult <- installDetectionTestResult{info: info, err: err}
			}()
			<-probeStarted

			waiterCtx, cancelWaiter := context.WithCancel(t.Context())
			waiterResult := make(chan installDetectionTestResult, 1)
			go func() {
				info, err := manager.detectInstall(waiterCtx)
				waiterResult <- installDetectionTestResult{info: info, err: err}
			}()
			synctest.Wait()

			cancelWaiter()
			synctest.Wait()
			if result := <-waiterResult; !errors.Is(result.err, context.Canceled) {
				t.Fatalf("waiter detectInstall() error = %v, want context.Canceled", result.err)
			}
			select {
			case result := <-ownerResult:
				t.Fatalf("owner returned before its probe completed: %#v", result)
			default:
			}

			close(releaseProbe)
			result := <-ownerResult
			if result.err != nil {
				t.Fatalf("owner detectInstall() error = %v", result.err)
			}
			if result.info.Method != string(InstallMethodAPT) || !result.info.Managed {
				t.Fatalf("owner detectInstall() = %#v, want managed apt", result.info)
			}
			if got := runCalls.Load(); got != 1 {
				t.Fatalf("runCommand() calls = %d, want 1", got)
			}
		})
	})

	t.Run("Should retry for an active waiter after a canceled owner returns success", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			var runCalls atomic.Int64
			firstProbeStarted := make(chan struct{})
			releaseFirstProbe := make(chan struct{})
			retryProbeStarted := make(chan struct{})
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
					switch runCalls.Add(1) {
					case 1:
						close(firstProbeStarted)
						<-releaseFirstProbe
					case 2:
						close(retryProbeStarted)
					default:
						return "", errors.New("unexpected extra install detection")
					}
					return "compozy: /usr/bin/compozy", nil
				},
			})

			ownerCtx, cancelOwner := context.WithCancel(t.Context())
			ownerResult := make(chan installDetectionTestResult, 1)
			go func() {
				info, err := manager.detectInstall(ownerCtx)
				ownerResult <- installDetectionTestResult{info: info, err: err}
			}()
			<-firstProbeStarted

			waiterResult := make(chan installDetectionTestResult, 1)
			go func() {
				info, err := manager.detectInstall(t.Context())
				waiterResult <- installDetectionTestResult{info: info, err: err}
			}()
			synctest.Wait()

			cancelOwner()
			close(releaseFirstProbe)
			owner := <-ownerResult
			if !errors.Is(owner.err, context.Canceled) {
				t.Fatalf("owner detectInstall() error = %v, want context.Canceled", owner.err)
			}

			waiter := <-waiterResult
			if waiter.err != nil {
				t.Fatalf("waiter detectInstall() error = %v", waiter.err)
			}
			if waiter.info.Method != string(InstallMethodAPT) || !waiter.info.Managed {
				t.Fatalf("waiter detectInstall() = %#v, want managed apt", waiter.info)
			}
			select {
			case <-retryProbeStarted:
			default:
				t.Fatal("active waiter did not retry install detection")
			}

			if _, err := manager.detectInstall(t.Context()); err != nil {
				t.Fatalf("detectInstall(cached) error = %v", err)
			}
			if got := runCalls.Load(); got != 2 {
				t.Fatalf("runCommand() calls = %d, want canceled owner plus waiter retry", got)
			}
		})
	})
}

type installDetectionTestResult struct {
	info installInfo
	err  error
}

func testManager(t *testing.T, cfg Config) *Manager {
	t.Helper()

	homePaths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	cfg.HomePaths = homePaths
	cfg.CurrentVersion = "v1.0.0"
	if cfg.ExecutablePath == nil {
		cfg.ExecutablePath = func() (string, error) {
			return filepath.Join(os.TempDir(), "compozy"), nil
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
