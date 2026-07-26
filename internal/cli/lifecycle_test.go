package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	aghupdate "github.com/compozy/agh/internal/update"
)

type stubUpdateManager struct {
	checkFn    func(context.Context, aghupdate.CheckOptions) (aghupdate.State, *aghupdate.Release, error)
	applyFn    func(context.Context, *aghupdate.Release) (aghupdate.AppliedBinary, error)
	restoreFn  func(aghupdate.AppliedBinary) error
	finalizeFn func(aghupdate.AppliedBinary) error
}

func (s stubUpdateManager) Check(
	ctx context.Context,
	opts aghupdate.CheckOptions,
) (aghupdate.State, *aghupdate.Release, error) {
	if s.checkFn != nil {
		return s.checkFn(ctx, opts)
	}
	return aghupdate.State{}, nil, nil
}

func (s stubUpdateManager) ApplyRelease(
	ctx context.Context,
	release *aghupdate.Release,
) (aghupdate.AppliedBinary, error) {
	if s.applyFn != nil {
		return s.applyFn(ctx, release)
	}
	return aghupdate.AppliedBinary{}, nil
}

func (s stubUpdateManager) Restore(applied aghupdate.AppliedBinary) error {
	if s.restoreFn != nil {
		return s.restoreFn(applied)
	}
	return nil
}

func (s stubUpdateManager) Finalize(applied aghupdate.AppliedBinary) error {
	if s.finalizeFn != nil {
		return s.finalizeFn(applied)
	}
	return nil
}

func TestInstallUpdateAndUninstallReportManagedState(t *testing.T) {
	t.Parallel()

	deps := newTestDeps(t, &stubClient{})
	deps.getenv = func(key string) string {
		if key == aghupdate.ManagedEnvName {
			return "homebrew"
		}
		return ""
	}
	deps.runInstallWizard = func(context.Context, installWizardInput) (installWizardSelection, error) {
		return installWizardSelection{Provider: "claude", Model: "claude-sonnet-4-6"}, nil
	}
	deps.newUpdateManager = func(aghconfig.HomePaths) (updateManager, error) {
		return stubUpdateManager{
			checkFn: func(context.Context, aghupdate.CheckOptions) (aghupdate.State, *aghupdate.Release, error) {
				return aghupdate.State{
					Managed:        true,
					InstallMethod:  "homebrew",
					CurrentVersion: "v1.0.0",
					LatestVersion:  "v1.1.0",
					Available:      true,
					Status:         aghupdate.StatusDeferred,
					Recommendation: "Use `brew upgrade compozy/compozy/agh`.",
					Message:        "AGH is managed by an external package manager; no local update was performed.",
				}, &aghupdate.Release{Version: "v1.1.0"}, nil
			},
		}, nil
	}

	installOut, _, err := executeRootCommand(t, deps, "install", "-o", "json")
	if err != nil {
		t.Fatalf("managed install error = %v", err)
	}
	var install installRecord
	if err := json.Unmarshal([]byte(installOut), &install); err != nil {
		t.Fatalf("json.Unmarshal(install) error = %v", err)
	}
	if !install.Managed || install.Manager != "homebrew" {
		t.Fatalf("install managed state = %#v, want homebrew", install)
	}

	updateOut, _, err := executeRootCommand(t, deps, "update", "-o", "json")
	if err != nil {
		t.Fatalf("managed update error = %v", err)
	}
	var update updateRecord
	if err := json.Unmarshal([]byte(updateOut), &update); err != nil {
		t.Fatalf("json.Unmarshal(update) error = %v", err)
	}
	if update.Status != string(aghupdate.StatusDeferred) || !update.Managed ||
		!strings.Contains(update.Recommendation, "brew") {
		t.Fatalf("managed update record = %#v, want deferred brew recommendation", update)
	}

	uninstallOut, _, err := executeRootCommand(t, deps, "uninstall", "--purge", "-o", "json")
	if err != nil {
		t.Fatalf("managed uninstall error = %v", err)
	}
	var uninstall lifecycleRecord
	if err := json.Unmarshal([]byte(uninstallOut), &uninstall); err != nil {
		t.Fatalf("json.Unmarshal(uninstall) error = %v", err)
	}
	if uninstall.Status != lifecycleStatusDeferred || !uninstall.Managed ||
		!strings.Contains(uninstall.Recommendation, "brew") {
		t.Fatalf("managed uninstall record = %#v, want deferred brew recommendation", uninstall)
	}
}

func TestManagedRecommendationReportsNPMCommands(t *testing.T) {
	t.Run("Should report npm lifecycle commands for npm-managed installs", func(t *testing.T) {
		t.Parallel()

		update := managedRecommendation("npm", "update AGH")
		if !strings.Contains(update, "npm update -g @compozy/agh") {
			t.Fatalf("managedRecommendation(update) = %q, want npm update command", update)
		}

		uninstall := managedRecommendation("nodejs", "uninstall AGH")
		if !strings.Contains(uninstall, "npm uninstall -g @compozy/agh") {
			t.Fatalf("managedRecommendation(uninstall) = %q, want npm uninstall command", uninstall)
		}
	})
}

func TestUninstallRemovesRuntimeArtifactsIdempotentlyAndRequiresForceForPurge(t *testing.T) {
	t.Parallel()

	deps := newTestDeps(t, &stubClient{})
	homePaths, err := deps.resolveHome()
	if err != nil {
		t.Fatalf("resolveHome() error = %v", err)
	}
	if err := aghconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	deps.processAlive = func(int) bool { return false }
	for _, path := range []string{homePaths.DaemonSocket, homePaths.DaemonLock} {
		writeFile(t, path, "runtime artifact")
	}
	writeFile(t, homePaths.DaemonInfo, `{"pid":999999,"port":0,"started_at":"2026-04-03T12:00:00Z"}`)

	out, _, err := executeRootCommand(t, deps, "uninstall", "-o", "json")
	if err != nil {
		t.Fatalf("uninstall error = %v", err)
	}
	var record lifecycleRecord
	if err := json.Unmarshal([]byte(out), &record); err != nil {
		t.Fatalf("json.Unmarshal(uninstall) error = %v", err)
	}
	if record.Status != lifecycleStatusUninstalled || record.Purged {
		t.Fatalf("uninstall record = %#v, want uninstalled without purge", record)
	}
	if len(record.Removed) != 3 {
		t.Fatalf("uninstall removed = %#v, want three runtime artifacts", record.Removed)
	}
	for _, path := range []string{homePaths.DaemonSocket, homePaths.DaemonLock, homePaths.DaemonInfo} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("runtime artifact %q still exists or stat error = %v", path, err)
		}
	}
	if _, err := os.Stat(homePaths.HomeDir); err != nil {
		t.Fatalf("home dir stat after uninstall error = %v", err)
	}

	secondOut, _, err := executeRootCommand(t, deps, "uninstall", "-o", "json")
	if err != nil {
		t.Fatalf("second uninstall error = %v", err)
	}
	var second lifecycleRecord
	if err := json.Unmarshal([]byte(secondOut), &second); err != nil {
		t.Fatalf("json.Unmarshal(second uninstall) error = %v", err)
	}
	if second.Status != lifecycleStatusUninstalled || len(second.Removed) != 0 {
		t.Fatalf("second uninstall record = %#v, want idempotent no-op removal", second)
	}

	if _, _, err := executeRootCommand(t, deps, "uninstall", "--purge"); err == nil {
		t.Fatal("uninstall --purge error = nil, want --force requirement")
	}
	purgeOut, _, err := executeRootCommand(t, deps, "uninstall", "--purge", "--force", "-o", "json")
	if err != nil {
		t.Fatalf("uninstall --purge --force error = %v", err)
	}
	var purge lifecycleRecord
	if err := json.Unmarshal([]byte(purgeOut), &purge); err != nil {
		t.Fatalf("json.Unmarshal(purge) error = %v", err)
	}
	if !purge.Purged {
		t.Fatalf("purge record = %#v, want purged", purge)
	}
	if _, err := os.Stat(homePaths.HomeDir); !os.IsNotExist(err) {
		t.Fatalf("home dir still exists after purge or stat error = %v", err)
	}
}

func TestUpdateCheckReportsAvailableReleaseForDirectBinaryInstall(t *testing.T) {
	t.Parallel()

	deps := newTestDeps(t, &stubClient{})
	deps.newUpdateManager = func(aghconfig.HomePaths) (updateManager, error) {
		return stubUpdateManager{
			checkFn: func(context.Context, aghupdate.CheckOptions) (aghupdate.State, *aghupdate.Release, error) {
				return aghupdate.State{
					Supported:      true,
					Managed:        false,
					InstallMethod:  "direct-binary",
					CurrentVersion: "v1.0.0",
					LatestVersion:  "v1.1.0",
					Available:      true,
					Status:         aghupdate.StatusAvailable,
					Message:        "A newer stable AGH release is available.",
				}, &aghupdate.Release{Version: "v1.1.0"}, nil
			},
		}, nil
	}

	out, _, err := executeRootCommand(t, deps, "update", "--check", "-o", "json")
	if err != nil {
		t.Fatalf("update --check error = %v", err)
	}
	var record updateRecord
	if err := json.Unmarshal([]byte(out), &record); err != nil {
		t.Fatalf("json.Unmarshal(update) error = %v", err)
	}
	if record.Status != string(aghupdate.StatusAvailable) || record.Managed || record.InstallMethod != "direct-binary" {
		t.Fatalf("update record = %#v, want available direct-binary update", record)
	}
}

func TestUpdateAppliesReleaseAndRestartsDaemonWhenRunning(t *testing.T) {
	t.Parallel()

	deps := newTestDeps(t, &stubClient{
		triggerSettingsRestartFn: func(context.Context) (SettingsRestartActionRecord, error) {
			return SettingsRestartActionRecord{OperationID: "op-123", Status: "pending"}, nil
		},
		getSettingsRestartStatusFn: func(context.Context, string) (SettingsRestartStatusRecord, error) {
			return SettingsRestartStatusRecord{OperationID: "op-123", Status: "ready"}, nil
		},
	})
	homePaths, err := deps.resolveHome()
	if err != nil {
		t.Fatalf("resolveHome() error = %v", err)
	}
	writeFile(t, homePaths.DaemonInfo, `{"pid":42,"port":2123,"started_at":"2026-04-03T12:00:00Z"}`)
	deps.processAlive = func(int) bool { return true }
	deps.newUpdateManager = func(aghconfig.HomePaths) (updateManager, error) {
		return stubUpdateManager{
			checkFn: func(context.Context, aghupdate.CheckOptions) (aghupdate.State, *aghupdate.Release, error) {
				return aghupdate.State{
					Supported:      true,
					Managed:        false,
					InstallMethod:  "direct-binary",
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
					BackupPath: filepath.Join(t.TempDir(), "agh.backup"),
					Version:    "v1.1.0",
				}, nil
			},
		}, nil
	}

	out, _, err := executeRootCommand(t, deps, "update", "-o", "json")
	if err != nil {
		t.Fatalf("update error = %v", err)
	}
	var record updateRecord
	if err := json.Unmarshal([]byte(out), &record); err != nil {
		t.Fatalf("json.Unmarshal(update) error = %v", err)
	}
	if record.Status != string(aghupdate.StatusUpdated) || !record.DaemonRestarted {
		t.Fatalf("update record = %#v, want updated with daemon restart", record)
	}
}

func TestConfigEditUsesEditorAndValidatesResult(t *testing.T) {
	t.Parallel()

	deps := newTestDeps(t, &stubClient{})
	homePaths, err := deps.resolveHome()
	if err != nil {
		t.Fatalf("resolveHome() error = %v", err)
	}
	editorPath := filepath.Join(t.TempDir(), "editor with spaces.sh")
	writeFile(
		t,
		editorPath,
		"#!/bin/sh\nfor target do :; done\nprintf '\\n[defaults]\\nprovider = \"claude\"\\n' >> \"$target\"\n",
	)
	if err := os.Chmod(editorPath, 0o700); err != nil {
		t.Fatalf("chmod editor error = %v", err)
	}
	deps.getenv = func(key string) string {
		if key == "EDITOR" {
			return "'" + editorPath + "' --append"
		}
		return ""
	}

	if _, _, err := executeRootCommand(t, deps, "config", "edit", "-o", "json"); err != nil {
		t.Fatalf("config edit error = %v", err)
	}
	cfg, err := aghconfig.LoadGlobalConfig(homePaths)
	if err != nil {
		t.Fatalf("LoadGlobalConfig(after edit) error = %v", err)
	}
	if cfg.Defaults.Provider != "claude" {
		t.Fatalf("Defaults.Provider after edit = %q, want claude", cfg.Defaults.Provider)
	}
}

func TestUninstallContinuesWhenRunningDaemonAlreadyExited(t *testing.T) {
	t.Parallel()

	deps := newTestDeps(t, &stubClient{})
	homePaths, err := deps.resolveHome()
	if err != nil {
		t.Fatalf("resolveHome() error = %v", err)
	}
	if err := aghconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	for _, path := range []string{homePaths.DaemonSocket, homePaths.DaemonLock} {
		writeFile(t, path, "runtime artifact")
	}
	writeFile(t, homePaths.DaemonInfo, `{"pid":999999,"port":0,"started_at":"2026-04-03T12:00:00Z"}`)
	deps.processAlive = func(int) bool { return true }
	deps.signalProcess = func(int, syscall.Signal) error { return os.ErrProcessDone }

	out, _, err := executeRootCommand(t, deps, "uninstall", "-o", "json")
	if err != nil {
		t.Fatalf("uninstall after already-exited daemon error = %v", err)
	}
	var record lifecycleRecord
	if err := json.Unmarshal([]byte(out), &record); err != nil {
		t.Fatalf("json.Unmarshal(uninstall) error = %v", err)
	}
	if record.Status != lifecycleStatusUninstalled || !record.DaemonStopped || len(record.Removed) != 3 {
		t.Fatalf("uninstall record = %#v, want stopped uninstall with artifacts removed", record)
	}
}

func TestUninstallIgnoresReusedPIDFromDaemonInfo(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should remove runtime artifacts without signaling when daemon info points to a reused PID",
		func(t *testing.T) {
			t.Parallel()

			deps := newTestDeps(t, &stubClient{})
			homePaths, err := deps.resolveHome()
			if err != nil {
				t.Fatalf("resolveHome() error = %v", err)
			}
			if err := aghconfig.EnsureHomeLayout(homePaths); err != nil {
				t.Fatalf("EnsureHomeLayout() error = %v", err)
			}
			for _, path := range []string{homePaths.DaemonSocket, homePaths.DaemonLock} {
				writeFile(t, path, "runtime artifact")
			}
			writeFile(t, homePaths.DaemonInfo, `{"pid":999999,"port":0,"started_at":"2026-04-03T12:00:00Z"}`)

			deps.processAlive = func(int) bool { return true }
			deps.processMatchesStartTime = func(int, time.Time) bool { return false }

			signalCalled := false
			deps.signalProcess = func(int, syscall.Signal) error {
				signalCalled = true
				return nil
			}

			out, _, err := executeRootCommand(t, deps, "uninstall", "-o", "json")
			if err != nil {
				t.Fatalf("uninstall error = %v", err)
			}
			if signalCalled {
				t.Fatal("signalProcess() called for reused PID, want no signal")
			}

			var record lifecycleRecord
			if err := json.Unmarshal([]byte(out), &record); err != nil {
				t.Fatalf("json.Unmarshal(uninstall) error = %v", err)
			}
			if record.Status != lifecycleStatusUninstalled || record.DaemonStopped || len(record.Removed) != 3 {
				t.Fatalf(
					"uninstall record = %#v, want uninstalled without daemon stop and with artifacts removed",
					record,
				)
			}
		},
	)
}
