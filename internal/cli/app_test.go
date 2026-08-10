package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestAppStatusReportsCanonicalState(t *testing.T) {
	t.Parallel()

	t.Run("Should read the shared Rust-written state fixtures", func(t *testing.T) {
		t.Parallel()
		for _, fixture := range []struct {
			name      string
			wantState string
		}{
			{name: "product.json", wantState: "product"},
			{name: "error.json", wantState: "error"},
		} {
			t.Run("Should read "+fixture.name, func(t *testing.T) {
				t.Parallel()
				raw, err := os.ReadFile(filepath.Join("..", "..", "desktop", "schema", "fixtures", fixture.name))
				if err != nil {
					t.Fatalf("ReadFile(shared state fixture) error = %v", err)
				}
				homePaths := appTestHome(t)
				if err := os.WriteFile(filepath.Join(homePaths.HomeDir, "app.json"), raw, 0o600); err != nil {
					t.Fatalf("WriteFile(app record) error = %v", err)
				}
				deps := appTestDeps(homePaths)
				deps.processAlive = func(int) bool { return true }
				deps.processMatchesStartTime = func(int, time.Time) bool { return true }
				report, err := resolveAppStatus(t.Context(), deps, homePaths)
				if err != nil {
					t.Fatalf("resolveAppStatus(shared state fixture) error = %v", err)
				}
				if report.State != fixture.wantState || !report.Running {
					t.Fatalf("shared state report = %#v, want state %q and running", report, fixture.wantState)
				}
			})
		}
	})

	t.Run("Should report a live product state with platform registration truth", func(t *testing.T) {
		t.Parallel()
		homePaths := appTestHome(t)
		writeAppTestRecord(t, homePaths, map[string]any{
			"schema_version": 1,
			"pid":            4242,
			"started_at":     "2026-08-10T03:00:00Z",
			"app_version":    "9.9.9-stale",
			"state":          "product",
			"origin":         "http://localhost:2123/",
			"owned":          true,
		})
		deps := appTestDeps(homePaths)
		deps.resolveAppInstallation = func(context.Context, compozyconfig.HomePaths) (appInstallation, error) {
			return appInstallation{Installed: true, Version: "0.3.0"}, nil
		}
		deps.processAlive = func(pid int) bool { return pid == 4242 }
		deps.processMatchesStartTime = func(pid int, startedAt time.Time) bool {
			return pid == 4242 && startedAt.Equal(time.Date(2026, 8, 10, 3, 0, 0, 0, time.UTC))
		}

		stdout, err := executeAppTestCommand(t, deps, "app", "status", "-o", "json")
		if err != nil {
			t.Fatalf("app status error = %v", err)
		}
		var report AppStatusReport
		if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
			t.Fatalf("decode app status: %v", err)
		}
		if !report.Installed || report.AppVersion != "0.3.0" || !report.Running ||
			report.State != "product" || !report.Runtime.Attached || !report.Runtime.Owned {
			t.Fatalf("app status = %#v, want live installed product", report)
		}
		if err := validateAppStateJSON(stdout.Bytes()); err != nil {
			t.Fatalf("status must validate against canonical schema: %v", err)
		}
	})

	t.Run("Should report an uninstalled and stopped app without error", func(t *testing.T) {
		t.Parallel()
		homePaths := appTestHome(t)
		deps := appTestDeps(homePaths)
		stdout, err := executeAppTestCommand(t, deps, "app", "status", "-o", "json")
		if err != nil {
			t.Fatalf("app status error = %v", err)
		}
		var report AppStatusReport
		if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
			t.Fatalf("decode app status: %v", err)
		}
		if report.Installed || report.Running {
			t.Fatalf("app status = %#v, want installed=false running=false", report)
		}
	})

	t.Run("Should preserve every nonterminal state shape written by Rust", func(t *testing.T) {
		t.Parallel()
		states := []struct {
			name  string
			extra map[string]any
		}{
			{name: "resolving"},
			{name: "provisioning", extra: map[string]any{"stage": "verify"}},
			{name: "starting", extra: map[string]any{"attempt": 2}},
			{name: "attaching"},
			{name: "updating", extra: map[string]any{"target": "runtime"}},
			{name: "disconnected", extra: map[string]any{"cause": "runtime_down"}},
			{name: "skew", extra: map[string]any{
				"runtime": "0.2.0",
				"needed":  ">=0.3.0",
				"newer":   false,
			}},
		}
		for _, state := range states {
			t.Run("Should report "+state.name, func(t *testing.T) {
				t.Parallel()
				homePaths := appTestHome(t)
				record := map[string]any{
					"schema_version": 1,
					"pid":            4242,
					"started_at":     "2026-08-10T03:00:00Z",
					"state":          state.name,
				}
				maps.Copy(record, state.extra)
				writeAppTestRecord(t, homePaths, record)
				deps := appTestDeps(homePaths)
				deps.processAlive = func(int) bool { return true }
				deps.processMatchesStartTime = func(int, time.Time) bool { return true }
				stdout, err := executeAppTestCommand(t, deps, "app", "status", "-o", "json")
				if err != nil {
					t.Fatalf("app status error = %v", err)
				}
				var report AppStatusReport
				if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
					t.Fatalf("decode app status: %v", err)
				}
				if report.State != state.name {
					t.Fatalf("app status state = %q, want %q", report.State, state.name)
				}
			})
		}
	})

	t.Run("Should mark a stale record as not running", func(t *testing.T) {
		t.Parallel()
		homePaths := appTestHome(t)
		writeAppTestRecord(t, homePaths, map[string]any{
			"schema_version": 1,
			"pid":            4242,
			"started_at":     "2026-08-10T03:00:00Z",
			"state":          "attaching",
		})
		deps := appTestDeps(homePaths)
		deps.processAlive = func(int) bool { return false }
		stdout, err := executeAppTestCommand(t, deps, "app", "status", "-o", "json")
		if err != nil {
			t.Fatalf("app status error = %v", err)
		}
		var report AppStatusReport
		if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
			t.Fatalf("decode app status: %v", err)
		}
		if report.Running || report.PID != 0 {
			t.Fatalf("stale app status = %#v, want running=false without pid", report)
		}
	})

	t.Run("Should reject an unknown state schema version deterministically", func(t *testing.T) {
		t.Parallel()
		homePaths := appTestHome(t)
		writeAppTestRecord(t, homePaths, map[string]any{
			"schema_version": 2,
			"pid":            4242,
			"started_at":     "2026-08-10T03:00:00Z",
			"state":          "starting",
		})
		_, err := executeAppTestCommand(t, appTestDeps(homePaths), "app", "status", "-o", "json")
		assertAppCommandError(t, err, appStateSchemaUnknownCode)
		assertStructuredAppError(
			t,
			[]string{"app", "status", "-o", "json"},
			err,
			appStateSchemaUnknownCode,
		)
	})
}

func TestAppOpenUsesValidatedProductTargets(t *testing.T) {
	t.Parallel()

	t.Run("Should return app_not_installed through structured output", func(t *testing.T) {
		t.Parallel()
		homePaths := appTestHome(t)
		deps := appTestDeps(homePaths)
		_, err := executeAppTestCommand(t, deps, "app", "open")
		if err == nil {
			t.Fatal("app open error = nil, want app_not_installed")
		}
		assertAppCommandError(t, err, appNotInstalledCode)
		assertStructuredAppError(t, []string{"app", "open", "-o", "json"}, err, appNotInstalledCode)
	})

	t.Run("Should reject traversal and foreign URLs", func(t *testing.T) {
		t.Parallel()
		for _, target := range []string{"../evil", "http://evil.example", "/sessions/%2e%2e/admin"} {
			t.Run("Should reject "+target, func(t *testing.T) {
				t.Parallel()
				homePaths := appTestHome(t)
				deps := appTestDeps(homePaths)
				_, err := executeAppTestCommand(t, deps, "app", "open", target)
				if err == nil {
					t.Fatal("app open error = nil, want invalid_target_path")
				}
				assertAppCommandError(t, err, invalidTargetPathCode)
			})
		}
	})

	t.Run("Should hand a valid product path to the registered scheme", func(t *testing.T) {
		t.Parallel()
		homePaths := appTestHome(t)
		deps := appTestDeps(homePaths)
		deps.resolveAppInstallation = func(context.Context, compozyconfig.HomePaths) (appInstallation, error) {
			return appInstallation{Installed: true, Version: "0.3.0"}, nil
		}
		opened := ""
		deps.openBrowser = func(_ context.Context, target string) error {
			opened = target
			return nil
		}
		if _, err := executeAppTestCommand(t, deps, "app", "open", "/sessions/abc"); err != nil {
			t.Fatalf("app open error = %v", err)
		}
		if opened != "compozyos://open/sessions/abc" {
			t.Fatalf("opened target = %q, want compozyos deep link", opened)
		}
	})

	t.Run("Should report app_launch_failed when the registered scheme cannot open", func(t *testing.T) {
		t.Parallel()
		homePaths := appTestHome(t)
		deps := appTestDeps(homePaths)
		deps.resolveAppInstallation = func(context.Context, compozyconfig.HomePaths) (appInstallation, error) {
			return appInstallation{Installed: true, Version: "0.3.0"}, nil
		}
		deps.openBrowser = func(context.Context, string) error { return errors.New("scheme unavailable") }
		_, err := executeAppTestCommand(t, deps, "app", "open")
		assertAppCommandError(t, err, appLaunchFailedCode)
	})
}

func TestAppControlReportsDeterministicTransportErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should report app_not_running when the socket is absent", func(t *testing.T) {
		t.Parallel()
		_, err := callAppControl(t.Context(), filepath.Join(t.TempDir(), "app.sock"), "update.check", nil)
		assertAppCommandError(t, err, appNotRunningCode)
	})

	t.Run("Should report app_control_unavailable when the socket is present but unresponsive", func(t *testing.T) {
		t.Parallel()
		socketPath := filepath.Join(t.TempDir(), "app.sock")
		if err := os.WriteFile(socketPath, []byte("stale"), 0o600); err != nil {
			t.Fatalf("WriteFile(socket) error = %v", err)
		}
		_, err := callAppControl(t.Context(), socketPath, "update.apply", map[string]string{"target": "runtime"})
		assertAppCommandError(t, err, appControlUnavailableCode)
	})

	t.Run("Should surface an absent socket through app update apply", func(t *testing.T) {
		t.Parallel()
		homePaths := appTestHome(t)
		deps := appTestDeps(homePaths)
		deps.resolveAppInstallation = func(context.Context, compozyconfig.HomePaths) (appInstallation, error) {
			return appInstallation{Installed: true, Version: "0.3.0"}, nil
		}
		deps.callAppControl = callAppControl
		_, err := executeAppTestCommand(t, deps, "app", "update", "--apply", "runtime", "-o", "json")
		assertAppCommandError(t, err, appNotRunningCode)
		assertStructuredAppError(
			t,
			[]string{"app", "update", "--apply", "runtime", "-o", "json"},
			err,
			appNotRunningCode,
		)
	})
}

func TestAppPlatformRegistrationOwnsInstallationTruth(t *testing.T) {
	t.Parallel()

	t.Run("Should derive macOS installation and version from bundle registration", func(t *testing.T) {
		t.Parallel()
		installation, err := resolveDarwinAppInstallation(t.Context(), func(
			_ context.Context,
			name string,
			args ...string,
		) (string, error) {
			switch name {
			case "mdfind":
				if len(args) != 1 {
					return "", fmt.Errorf("mdfind args = %#v", args)
				}
				return "/Applications/CompozyOS.app\n", nil
			case "defaults":
				return "0.3.0\n", nil
			default:
				return "", fmt.Errorf("unexpected command %q", name)
			}
		})
		if err != nil || !installation.Installed || installation.Version != "0.3.0" {
			t.Fatalf("macOS installation = %#v error=%v", installation, err)
		}
	})

	t.Run("Should derive Windows installation and version from the uninstall registry", func(t *testing.T) {
		t.Parallel()
		installation, err := resolveWindowsAppInstallation(t.Context(), func(
			context.Context,
			string,
			...string,
		) (string, error) {
			return "installed\r\n0.3.0\r\n", nil
		})
		if err != nil || !installation.Installed || installation.Version != "0.3.0" {
			t.Fatalf("Windows installation = %#v error=%v", installation, err)
		}
	})

	t.Run("Should treat a registered Windows app without a version as installed", func(t *testing.T) {
		t.Parallel()
		installation, err := resolveWindowsAppInstallation(t.Context(), func(
			context.Context,
			string,
			...string,
		) (string, error) {
			return "installed\r\n", nil
		})
		if err != nil || !installation.Installed || installation.Version != "" {
			t.Fatalf("Windows installation = %#v error=%v", installation, err)
		}
	})

	t.Run("Should derive Linux installation from the desktop entry", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "compozyos.desktop")
		contents := []byte("[Desktop Entry]\nName=CompozyOS\nX-Compozy-Version=0.3.0\n")
		if err := os.WriteFile(path, contents, 0o600); err != nil {
			t.Fatalf("WriteFile(desktop entry) error = %v", err)
		}
		installation, found, err := parseDesktopRegistration(path)
		if err != nil || !found || !installation.Installed || installation.Version != "0.3.0" {
			t.Fatalf("Linux installation = %#v found=%t error=%v", installation, found, err)
		}
	})
}

func appTestHome(t *testing.T) compozyconfig.HomePaths {
	t.Helper()
	homePaths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := os.MkdirAll(homePaths.HomeDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
	}
	return homePaths
}

func appTestDeps(homePaths compozyconfig.HomePaths) commandDeps {
	return commandDeps{
		resolveHome: func() (compozyconfig.HomePaths, error) { return homePaths, nil },
		resolveAppInstallation: func(context.Context, compozyconfig.HomePaths) (appInstallation, error) {
			return appInstallation{}, nil
		},
		processAlive:            func(int) bool { return false },
		processMatchesStartTime: func(int, time.Time) bool { return false },
	}
}

func executeAppTestCommand(t *testing.T, deps commandDeps, args ...string) (bytes.Buffer, error) {
	t.Helper()
	command := newRootCommand(deps)
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&bytes.Buffer{})
	command.SetArgs(args)
	err := command.ExecuteContext(t.Context())
	return stdout, err
}

func writeAppTestRecord(t *testing.T, homePaths compozyconfig.HomePaths, value map[string]any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal(app record) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(homePaths.HomeDir, "app.json"), raw, 0o600); err != nil {
		t.Fatalf("WriteFile(app record) error = %v", err)
	}
}

func assertAppCommandError(t *testing.T, err error, wantCode string) {
	t.Helper()
	var appErr *appCommandError
	if !errors.As(err, &appErr) || appErr.code != wantCode {
		t.Fatalf("error = %#v, want app command code %q", err, wantCode)
	}
}

func assertStructuredAppError(t *testing.T, args []string, err error, wantCode string) {
	t.Helper()
	var stderr bytes.Buffer
	if exitCode := writeExecutionError(&stderr, args, err); exitCode == 0 {
		t.Fatal("writeExecutionError() exit code = 0, want non-zero")
	}
	var payload appCommandErrorEnvelope
	if decodeErr := json.Unmarshal(stderr.Bytes(), &payload); decodeErr != nil {
		t.Fatalf("decode structured app error: %v", decodeErr)
	}
	if payload.Error.Code != wantCode {
		t.Fatalf("structured app error = %#v, want code %q", payload, wantCode)
	}
}
