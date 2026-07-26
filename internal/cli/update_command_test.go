package cli

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	aghupdate "github.com/compozy/agh/internal/update"
)

func TestUpdateCommandFlows(t *testing.T) {
	t.Run("Should finalize the updated binary without restarting when the daemon is not running", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{}
		deps := newTestDeps(t, client)
		var (
			applyCalls    int
			finalizeCalls int
		)
		deps.newUpdateManager = func(aghconfig.HomePaths) (updateManager, error) {
			return stubUpdateManager{
				checkFn: func(_ context.Context, opts aghupdate.CheckOptions) (aghupdate.State, *aghupdate.Release, error) {
					if !opts.ForceRefresh {
						t.Fatal("CheckOptions.ForceRefresh = false, want true")
					}
					return aghupdate.State{
						Supported:      true,
						InstallMethod:  string(aghupdate.InstallMethodDirectBinary),
						CurrentVersion: "v1.0.0",
						LatestVersion:  "v1.1.0",
						Available:      true,
						Status:         aghupdate.StatusAvailable,
						Message:        "A newer stable AGH release is available.",
					}, &aghupdate.Release{Version: "v1.1.0"}, nil
				},
				applyFn: func(_ context.Context, release *aghupdate.Release) (aghupdate.AppliedBinary, error) {
					applyCalls++
					if release == nil || release.Version != "v1.1.0" {
						t.Fatalf("apply release = %#v, want v1.1.0", release)
					}
					return aghupdate.AppliedBinary{
						TargetPath: filepath.Join(t.TempDir(), "agh"),
						BackupPath: filepath.Join(t.TempDir(), ".agh.backup"),
						Version:    "v1.1.0",
					}, nil
				},
				finalizeFn: func(applied aghupdate.AppliedBinary) error {
					finalizeCalls++
					if applied.Version != "v1.1.0" {
						t.Fatalf("finalize applied = %#v, want version v1.1.0", applied)
					}
					return nil
				},
			}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "update", "-o", "json")
		if err != nil {
			t.Fatalf("executeRootCommand() error = %v", err)
		}
		if applyCalls != 1 || finalizeCalls != 1 {
			t.Fatalf("apply/finalize calls = %d/%d, want 1/1", applyCalls, finalizeCalls)
		}

		var record updateRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(update) error = %v", err)
		}
		if record.Status != string(aghupdate.StatusUpdated) || record.DaemonRestarted {
			t.Fatalf("record = %#v, want updated without daemon restart", record)
		}
	})

	t.Run("Should restore the previous binary and relaunch the daemon when restart fails", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			triggerSettingsRestartFn: func(context.Context) (SettingsRestartActionRecord, error) {
				return SettingsRestartActionRecord{OperationID: "op-123", Status: "pending"}, nil
			},
			getSettingsRestartStatusFn: func(context.Context, string) (SettingsRestartStatusRecord, error) {
				return SettingsRestartStatusRecord{
					OperationID:   "op-123",
					Status:        "failed",
					FailureReason: "replacement daemon failed readiness checks",
				}, nil
			},
			daemonStatusFn: func(context.Context) (DaemonStatus, error) {
				return DaemonStatus{Status: "ready", PID: 42}, nil
			},
		}
		deps := newTestDeps(t, client)
		homePaths, err := deps.resolveHome()
		if err != nil {
			t.Fatalf("resolveHome() error = %v", err)
		}
		writeFile(t, homePaths.DaemonInfo, `{"pid":42,"port":2123,"started_at":"2026-04-03T12:00:00Z"}`)

		var (
			restoreCalls  int
			finalizeCalls int
			spawnCalls    int
			aliveChecks   int
		)
		recoveryProcess := &stubDaemonProcess{done: make(chan struct{})}
		t.Cleanup(func() {
			recoveryProcess.complete(nil)
		})

		deps.processAlive = func(int) bool {
			aliveChecks++
			return aliveChecks == 1
		}
		deps.spawnDetached = func(_ context.Context, gotHome aghconfig.HomePaths) (daemonProcess, error) {
			spawnCalls++
			if gotHome.HomeDir != homePaths.HomeDir {
				t.Fatalf("spawnDetached() home = %q, want %q", gotHome.HomeDir, homePaths.HomeDir)
			}
			return recoveryProcess, nil
		}
		deps.pollInterval = defaultPollInterval / 10
		deps.startTimeout = defaultStartTimeout / 10

		deps.newUpdateManager = func(aghconfig.HomePaths) (updateManager, error) {
			return stubUpdateManager{
				checkFn: func(context.Context, aghupdate.CheckOptions) (aghupdate.State, *aghupdate.Release, error) {
					return aghupdate.State{
						Supported:      true,
						InstallMethod:  string(aghupdate.InstallMethodDirectBinary),
						CurrentVersion: "v1.0.0",
						LatestVersion:  "v1.1.0",
						Available:      true,
						Status:         aghupdate.StatusAvailable,
						Message:        "A newer stable AGH release is available.",
					}, &aghupdate.Release{Version: "v1.1.0"}, nil
				},
				applyFn: func(context.Context, *aghupdate.Release) (aghupdate.AppliedBinary, error) {
					return aghupdate.AppliedBinary{
						TargetPath: filepath.Join(t.TempDir(), "agh"),
						BackupPath: filepath.Join(t.TempDir(), ".agh.backup"),
						Version:    "v1.1.0",
					}, nil
				},
				restoreFn: func(applied aghupdate.AppliedBinary) error {
					restoreCalls++
					if applied.Version != "v1.1.0" {
						t.Fatalf("restore applied = %#v, want version v1.1.0", applied)
					}
					return nil
				},
				finalizeFn: func(aghupdate.AppliedBinary) error {
					finalizeCalls++
					return nil
				},
			}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "update", "-o", "json")
		if err == nil {
			t.Fatal("executeRootCommand() error = nil, want restart failure")
		}
		if restoreCalls != 1 {
			t.Fatalf("restore calls = %d, want 1", restoreCalls)
		}
		if finalizeCalls != 0 {
			t.Fatalf("finalize calls = %d, want 0", finalizeCalls)
		}
		if spawnCalls != 1 {
			t.Fatalf("spawnDetached() calls = %d, want 1", spawnCalls)
		}

		var record updateRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(update failure) error = %v", err)
		}
		if record.Status != string(aghupdate.StatusFailed) {
			t.Fatalf("record.Status = %q, want %q", record.Status, aghupdate.StatusFailed)
		}
		if !strings.Contains(record.Message, "replacement daemon failed readiness checks") {
			t.Fatalf("record.Message = %q, want restart failure detail", record.Message)
		}
	})

	t.Run("Should skip apply when the current install already matches the latest stable release", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
		deps.newUpdateManager = func(aghconfig.HomePaths) (updateManager, error) {
			return stubUpdateManager{
				checkFn: func(context.Context, aghupdate.CheckOptions) (aghupdate.State, *aghupdate.Release, error) {
					return aghupdate.State{
						Supported:      true,
						InstallMethod:  string(aghupdate.InstallMethodDirectBinary),
						CurrentVersion: "v1.1.0",
						LatestVersion:  "v1.1.0",
						Available:      false,
						Status:         aghupdate.StatusCurrent,
						Message:        "AGH is already on the latest stable release.",
					}, &aghupdate.Release{Version: "v1.1.0"}, nil
				},
				applyFn: func(context.Context, *aghupdate.Release) (aghupdate.AppliedBinary, error) {
					return aghupdate.AppliedBinary{}, errors.New("apply should not run")
				},
			}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "update", "-o", "json")
		if err != nil {
			t.Fatalf("executeRootCommand() error = %v", err)
		}

		var record updateRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(update current) error = %v", err)
		}
		if record.Status != string(aghupdate.StatusCurrent) || record.CurrentVersion != "v1.1.0" {
			t.Fatalf("record = %#v, want current latest snapshot", record)
		}
	})
}

func TestSettingsRestartTimeout(t *testing.T) {
	t.Run("Should enforce the restart timeout floor", func(t *testing.T) {
		t.Parallel()

		timeout := settingsRestartTimeout(commandDeps{
			startTimeout: 15 * time.Second,
			stopTimeout:  15 * time.Second,
		})
		if timeout != defaultSettingsRestartTimeout {
			t.Fatalf("settingsRestartTimeout() = %s, want %s", timeout, defaultSettingsRestartTimeout)
		}
	})

	t.Run("Should extend the timeout when the configured stop and start windows exceed the floor", func(t *testing.T) {
		t.Parallel()

		timeout := settingsRestartTimeout(commandDeps{
			startTimeout: 25 * time.Second,
			stopTimeout:  25 * time.Second,
		})
		if timeout != 55*time.Second {
			t.Fatalf("settingsRestartTimeout() = %s, want 55s", timeout)
		}
	})
}
