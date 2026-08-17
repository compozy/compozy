package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	compozyupdate "github.com/compozy/compozy/internal/update"
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
			"schema_version":    2,
			"pid":               4242,
			"started_at":        "2026-08-10T03:00:00Z",
			"app_version":       "9.9.9-stale",
			"state":             "product",
			"origin":            "http://localhost:2123/",
			"owned":             true,
			"diagnostic_report": appDiagnosticReportFixture(),
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

	t.Run("Should overlay the durable operation onto the schema v2 update block", func(t *testing.T) {
		t.Parallel()

		homePaths := appTestHome(t)
		store, err := compozyupdate.NewOperationStore(homePaths, nil)
		if err != nil {
			t.Fatalf("NewOperationStore() error = %v", err)
		}
		operation := acquireCLIUpdateOperation(t, store, []compozyupdate.Target{compozyupdate.TargetApp})
		staged, err := store.Transition(
			t.Context(),
			operation.ID,
			operation.Holder.ExecutorGeneration,
			operation.Revision,
			compozyupdate.Transition{
				Kind: compozyupdate.TransitionPhase, Actor: compozyupdate.ActorCLI,
				Target: compozyupdate.TargetApp, Phase: compozyupdate.PhaseStaged, Percent: 100,
			},
		)
		if err != nil {
			t.Fatalf("Transition(staged) error = %v", err)
		}
		_, err = store.Transition(t.Context(), staged.ID, staged.Holder.ExecutorGeneration, staged.Revision,
			compozyupdate.Transition{
				Kind: compozyupdate.TransitionWaitForApp, Actor: compozyupdate.ActorCLI,
				Target: compozyupdate.TargetApp, Percent: -1,
			})
		if err != nil {
			t.Fatalf("Transition(waiting) error = %v", err)
		}
		deps := appTestDeps(homePaths)
		deps.resolveAppInstallation = func(context.Context, compozyconfig.HomePaths) (appInstallation, error) {
			return appInstallation{Installed: true, Version: "v1.0.0"}, nil
		}
		report, err := resolveAppStatus(t.Context(), deps, homePaths)
		if err != nil {
			t.Fatalf("resolveAppStatus() error = %v", err)
		}
		if report.Update.OperationID != operation.ID || report.Update.AppState != "staged" ||
			report.Update.AppAvailable != "v1.1.0" || report.Update.Percent == nil || *report.Update.Percent != -1 {
			t.Fatalf("app update projection = %#v, want dormant staged operation", report.Update)
		}
	})

	t.Run("Should project representative live update phases through the shared mapper", func(t *testing.T) {
		t.Parallel()

		for _, test := range []struct {
			name        string
			target      compozyupdate.Target
			phase       compozyupdate.OperationPhase
			percent     int
			wantState   string
			wantUIPhase string
		}{
			{
				name: "Should project accepted runtime work", target: compozyupdate.TargetRuntime,
				phase: compozyupdate.PhasePending, percent: 0, wantState: string(compozyupdate.StatusAccepted),
			},
			{
				name: "Should project downloading runtime work", target: compozyupdate.TargetRuntime,
				phase: compozyupdate.PhaseDownloading, percent: 25,
				wantState: "applying", wantUIPhase: string(compozyupdate.UIPhaseDownload),
			},
			{
				name: "Should project accepted app work", target: compozyupdate.TargetApp,
				phase: compozyupdate.PhasePending, percent: 0, wantState: string(compozyupdate.StatusAccepted),
			},
			{
				name: "Should project applying app work", target: compozyupdate.TargetApp,
				phase: compozyupdate.PhaseApplying, percent: 25,
				wantState: "applying", wantUIPhase: string(compozyupdate.UIPhaseDownload),
			},
		} {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				homePaths := appTestHome(t)
				store, err := compozyupdate.NewOperationStore(homePaths, nil)
				if err != nil {
					t.Fatalf("NewOperationStore() error = %v", err)
				}
				operation := acquireCLIUpdateOperation(t, store, []compozyupdate.Target{test.target})
				if test.phase != compozyupdate.PhasePending {
					phases := []compozyupdate.OperationPhase{test.phase}
					if test.target == compozyupdate.TargetApp && test.phase == compozyupdate.PhaseApplying {
						phases = []compozyupdate.OperationPhase{compozyupdate.PhaseStaged, test.phase}
					}
					for _, phase := range phases {
						operation, err = store.Transition(
							t.Context(), operation.ID, operation.Holder.ExecutorGeneration, operation.Revision,
							compozyupdate.Transition{
								Kind: compozyupdate.TransitionPhase, Actor: compozyupdate.ActorCLI,
								Target: test.target, Phase: phase, Percent: test.percent,
							},
						)
						if err != nil {
							t.Fatalf("Transition(%s) error = %v", phase, err)
						}
					}
				}
				report := AppStatusReport{Update: AppUpdateState{
					AppState: appIdleState, RuntimeState: appIdleState,
				}}
				if err := overlayAppUpdateOperation(t.Context(), homePaths, &report); err != nil {
					t.Fatalf("overlayAppUpdateOperation() error = %v", err)
				}
				gotState := report.Update.RuntimeState
				if test.target == compozyupdate.TargetApp {
					gotState = report.Update.AppState
				}
				if gotState != test.wantState || report.Update.Phase != test.wantUIPhase ||
					report.Update.Percent == nil || *report.Update.Percent != test.percent {
					t.Fatalf("update projection = %#v, want state=%q phase=%q percent=%d", report.Update, test.wantState, test.wantUIPhase, test.percent)
				}
			})
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
					"schema_version":    2,
					"pid":               4242,
					"started_at":        "2026-08-10T03:00:00Z",
					"state":             state.name,
					"diagnostic_report": appDiagnosticReportFixture(),
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
			"schema_version":    2,
			"pid":               4242,
			"started_at":        "2026-08-10T03:00:00Z",
			"state":             "attaching",
			"diagnostic_report": appDiagnosticReportFixture(),
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
			"schema_version": 3,
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

func TestAppDiagnoseReportsSafeStateAndBundles(t *testing.T) {
	t.Parallel()

	t.Run("Should return the live shared report without wrapping it", func(t *testing.T) {
		t.Parallel()
		homePaths := appTestHome(t)
		deps := appTestDeps(homePaths)
		deps.callAppControl = func(
			_ context.Context,
			_ string,
			method string,
			params any,
		) (any, error) {
			if method != appDiagnoseMethod {
				t.Fatalf("app control method = %q, want %q", method, appDiagnoseMethod)
			}
			values, ok := params.(map[string]any)
			if !ok || len(values) != 0 {
				t.Fatalf("app diagnose params = %#v, want empty object", params)
			}
			return appDiagnosticReportFixture(), nil
		}

		stdout, err := executeAppTestCommand(t, deps, "app", "diagnose", "-o", "json")
		if err != nil {
			t.Fatalf("app diagnose error = %v", err)
		}
		var report appDiagnosticReport
		if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
			t.Fatalf("decode app diagnose report: %v", err)
		}
		if report.BootID != "boot_123" || report.BootPhase != appProductState || report.CurrentError == nil {
			t.Fatalf("app diagnose report = %#v, want the shared safe report", report)
		}
		if _, err := validateAppDiagnosticReport(stdout.Bytes()); err != nil {
			t.Fatalf("app diagnose report must validate against the shared schema: %v", err)
		}
		if bytes.Contains(stdout.Bytes(), []byte(homePaths.HomeDir)) {
			t.Fatalf("app diagnose report contains raw home path: %s", stdout.Bytes())
		}
		human, humanErr := executeAppTestCommand(t, deps, "app", "diagnose")
		if humanErr != nil {
			t.Fatalf("human app diagnose error = %v", humanErr)
		}
		if !strings.Contains(human.String(), "Error code") || !strings.Contains(human.String(), "runtime_unhealthy") {
			t.Fatalf("human app diagnose output = %q, want error code", human.String())
		}
	})

	t.Run("Should delegate a live bundle export to the desktop app", func(t *testing.T) {
		t.Parallel()
		homePaths := appTestHome(t)
		if err := os.MkdirAll(homePaths.LogsDir, 0o700); err != nil {
			t.Fatalf("MkdirAll(live diagnostic logs) error = %v", err)
		}
		for _, source := range []struct {
			name string
			line string
		}{
			{
				name: "desktop.log",
				line: `{"timestamp":"2026-04-03T12:00:00Z","level":"info","boot_id":"boot_exported","message":"Desktop event"}`,
			},
			{
				name: "desktop-bootstrap.jsonl",
				line: `{"timestamp":"2026-04-03T12:00:00Z","level":"info","boot_id":"boot_exported","message":"Bootstrap event"}`,
			},
		} {
			if err := os.WriteFile(
				filepath.Join(homePaths.LogsDir, source.name),
				[]byte(source.line),
				0o600,
			); err != nil {
				t.Fatalf("WriteFile(live diagnostic log %q) error = %v", source.name, err)
			}
		}
		deps := appTestDeps(homePaths)
		deps.now = func() time.Time { return fixedTestNow }
		exported := false
		deps.callAppControl = func(
			_ context.Context,
			_ string,
			method string,
			params any,
		) (any, error) {
			switch method {
			case appDiagnoseMethod:
				report := appDiagnosticReportFixture()
				report["boot_id"] = "boot_before_export"
				return report, nil
			case appExportDiagnosticsMethod:
				exported = true
				values, ok := params.(map[string]any)
				if !ok || len(values) != 1 || values["consent"] != true {
					t.Fatalf("app export params = %#v, want only consent=true", params)
				}
				exportedReport := appDiagnosticReportFixture()
				exportedReport["boot_id"] = "boot_exported"
				report, decodeErr := decodeAppDiagnosticReport(exportedReport)
				if decodeErr != nil {
					t.Fatalf("decode fixture report: %v", decodeErr)
				}
				outputPath := filepath.Join(
					homePaths.HomeDir,
					supportBundlesDirName,
					appDiagnosticBundleFileName(fixedTestNow),
				)
				bundle, writeErr := writeLocalAppDiagnosticBundle(homePaths, report, outputPath, fixedTestNow, true)
				if writeErr != nil {
					t.Fatalf("write delegated bundle: %v", writeErr)
				}
				return map[string]any{"bundle_path": bundle.Path, "bytes": bundle.Bytes}, nil
			default:
				t.Fatalf("app control method = %q, want diagnose or export", method)
				return nil, nil
			}
		}

		stdout, err := executeAppTestCommand(t, deps, "app", "diagnose", "--bundle", "--yes", "-o", "json")
		if err != nil {
			t.Fatalf("live app diagnose bundle error = %v", err)
		}
		if !exported {
			t.Fatal("app control did not receive export_diagnostics")
		}
		var result appDiagnosticBundleResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("decode live diagnostic bundle result: %v", err)
		}
		if result.Report.BootID != "boot_exported" || result.Bundle.Manifest.Report.BootID != "boot_exported" {
			t.Fatalf("live diagnostic bundle result = %#v, want shared manifest", result)
		}
		if entries := readAppDiagnosticBundleArchive(t, result.Bundle.Path); len(entries) != 3 {
			t.Fatalf("live diagnostic bundle entries = %#v, want manifest and both safe log tails", entries)
		}
	})

	t.Run("Should keep explicit live bundle paths in the CLI", func(t *testing.T) {
		t.Parallel()
		homePaths := appTestHome(t)
		outputPath := filepath.Join(t.TempDir(), "diagnostics.tar.gz")
		deps := appTestDeps(homePaths)
		deps.now = func() time.Time { return fixedTestNow }
		deps.callAppControl = func(_ context.Context, _ string, method string, _ any) (any, error) {
			if method != appDiagnoseMethod {
				t.Fatalf("app control method = %q, want only diagnose", method)
			}
			return appDiagnosticReportFixture(), nil
		}

		stdout, err := executeAppTestCommand(
			t,
			deps,
			"app",
			"diagnose",
			"--bundle",
			"--yes",
			"--bundle-output",
			outputPath,
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("explicit live bundle error = %v", err)
		}
		var result appDiagnosticBundleResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("decode explicit live bundle result: %v", err)
		}
		if result.Bundle.Path != outputPath || result.Bundle.Manifest.Report.BootID != "boot_123" {
			t.Fatalf("explicit live bundle = %#v, want local archive at selected path", result.Bundle)
		}
	})

	t.Run("Should reject invalid live bundle archive structures", func(t *testing.T) {
		t.Parallel()
		report, err := decodeAppDiagnosticReport(appDiagnosticReportFixture())
		if err != nil {
			t.Fatalf("decode fixture report error = %v", err)
		}
		_, manifestBytes, err := newAppDiagnosticBundleManifest(report, fixedTestNow)
		if err != nil {
			t.Fatalf("create fixture manifest error = %v", err)
		}
		for _, test := range []struct {
			name    string
			entries []appDiagnosticBundleEntry
			want    string
		}{
			{
				name: "unexpected entry",
				entries: []appDiagnosticBundleEntry{
					{Name: appDiagnosticBundleManifestFile, Data: manifestBytes},
					{Name: "compozy.log", Data: []byte("must not be accepted")},
				},
				want: "unexpected entry",
			},
			{
				name: "duplicate entry",
				entries: []appDiagnosticBundleEntry{
					{Name: appDiagnosticBundleManifestFile, Data: manifestBytes},
					{Name: appDiagnosticBundleManifestFile, Data: manifestBytes},
				},
				want: "duplicate entry",
			},
			{
				name: "oversized manifest",
				entries: []appDiagnosticBundleEntry{
					{Name: appDiagnosticBundleManifestFile, Data: bytes.Repeat([]byte("m"), 49*1024)},
				},
				want: "manifest exceeds",
			},
			{
				name: "oversized log",
				entries: []appDiagnosticBundleEntry{
					{Name: appDiagnosticBundleManifestFile, Data: manifestBytes},
					{Name: appDiagnosticBundleDesktopLogFile, Data: bytes.Repeat([]byte("l"), 16*1024+1)},
				},
				want: "log entry exceeds",
			},
			{
				name: "oversized aggregate",
				entries: []appDiagnosticBundleEntry{
					{Name: appDiagnosticBundleManifestFile, Data: bytes.Repeat([]byte("m"), 20*1024)},
					{Name: appDiagnosticBundleDesktopLogFile, Data: bytes.Repeat([]byte("d"), 15*1024)},
					{Name: appDiagnosticBundleBootstrapLogFile, Data: bytes.Repeat([]byte("b"), 15*1024)},
				},
				want: "bundle exceeds",
			},
		} {
			t.Run("Should reject "+test.name, func(t *testing.T) {
				t.Parallel()
				bundlePath := filepath.Join(t.TempDir(), "invalid.tar.gz")
				file, openErr := os.OpenFile(bundlePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
				if openErr != nil {
					t.Fatalf("OpenFile(invalid bundle) error = %v", openErr)
				}
				if writeErr := writeAppDiagnosticBundleArchive(file, test.entries); writeErr != nil {
					if closeErr := file.Close(); closeErr != nil {
						t.Fatalf("write invalid bundle error = %v; Close() error = %v", writeErr, closeErr)
					}
					t.Fatalf("write invalid bundle error = %v", writeErr)
				}
				if closeErr := file.Close(); closeErr != nil {
					t.Fatalf("Close(invalid bundle) error = %v", closeErr)
				}
				_, readErr := readAppDiagnosticBundleManifest(bundlePath)
				if readErr == nil || !strings.Contains(readErr.Error(), test.want) {
					t.Fatalf("invalid bundle error = %v, want %q", readErr, test.want)
				}
			})
		}
	})

	t.Run("Should reject an outside live bundle path before reading it", func(t *testing.T) {
		t.Parallel()
		homePaths := appTestHome(t)
		deps := appTestDeps(homePaths)
		deps.callAppControl = func(_ context.Context, _ string, method string, _ any) (any, error) {
			if method == appDiagnoseMethod {
				return appDiagnosticReportFixture(), nil
			}
			return map[string]any{
				"bundle_path": filepath.Join(t.TempDir(), "outside.tar.gz"),
				"bytes":       1,
			}, nil
		}

		_, err := executeAppTestCommand(t, deps, "app", "diagnose", "--bundle", "--yes")
		if err == nil || !strings.Contains(err.Error(), "unexpected bundle path") {
			t.Fatalf("outside live bundle error = %v, want path rejection", err)
		}
	})

	t.Run("Should fall back to a validated local report when the socket is absent", func(t *testing.T) {
		t.Parallel()
		homePaths := appTestHome(t)
		writeAppTestRecord(t, homePaths, map[string]any{
			"schema_version": 2,
			"pid":            4242,
			"started_at":     "2026-08-10T03:00:00Z",
			"state":          "error",
			"error": map[string]any{
				"code":         "runtime_unhealthy",
				"safe_message": "The runtime is not responding.",
				"log_path":     filepath.Join(homePaths.HomeDir, "desktop.log"),
			},
			"diagnostic_report": appDiagnosticReportFixture(),
		})
		deps := appTestDeps(homePaths)
		deps.callAppControl = func(context.Context, string, string, any) (any, error) {
			return nil, newAppCommandError(appNotRunningCode, "the CompozyOS desktop app is not running", nil)
		}

		stdout, err := executeAppTestCommand(t, deps, "app", "diagnose", "-o", "json")
		if err != nil {
			t.Fatalf("app diagnose fallback error = %v", err)
		}
		var report appDiagnosticReport
		if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
			t.Fatalf("decode fallback report: %v", err)
		}
		if report.BootPhase != "product" || report.CurrentError == nil {
			t.Fatalf("fallback report = %#v, want persisted report", report)
		}
		if bytes.Contains(stdout.Bytes(), []byte(homePaths.HomeDir)) {
			t.Fatalf("fallback report contains raw home path: %s", stdout.Bytes())
		}
	})

	t.Run("Should require consent before creating a diagnostic bundle", func(t *testing.T) {
		t.Parallel()
		homePaths := appTestHome(t)
		deps := appTestDeps(homePaths)
		_, err := executeAppTestCommand(t, deps, "app", "diagnose", "--bundle", "-o", "json")
		if err == nil {
			t.Fatal("app diagnose bundle error = nil, want consent failure")
		}
		assertAppCommandError(t, err, appDiagnosticConsentRequiredCode)
		assertStructuredAppError(
			t,
			[]string{"app", "diagnose", "--bundle", "-o", "json"},
			err,
			appDiagnosticConsentRequiredCode,
		)
	})

	t.Run("Should create a safe local archive when the app is offline", func(t *testing.T) {
		t.Parallel()
		homePaths := appTestHome(t)
		diagnosticReport := appDiagnosticReportFixture()
		diagnosticReport["current_error"] = map[string]any{
			"code":         "runtime_unhealthy",
			"safe_message": "token=raw-secret",
		}
		writeAppTestRecord(t, homePaths, map[string]any{
			"schema_version": 2,
			"pid":            4242,
			"started_at":     "2026-08-10T03:00:00Z",
			"state":          "error",
			"error": map[string]any{
				"code":         "runtime_unhealthy",
				"safe_message": "The runtime is not responding.",
				"log_path":     filepath.Join(homePaths.HomeDir, "desktop.log"),
			},
			"diagnostic_report": diagnosticReport,
		})
		if err := os.MkdirAll(homePaths.LogsDir, 0o700); err != nil {
			t.Fatalf("MkdirAll(diagnostic logs) error = %v", err)
		}
		desktopLog := strings.Join([]string{
			`{"timestamp":"2026-04-03T12:00:00Z","level":"info","boot_id":"boot_123","message":"Desktop started from ` + homePaths.HomeDir + `"}`,
			`{"timestamp":"2026-04-03T12:00:01Z","level":"info","boot_id":"boot_123","message":"token=raw-secret"}`,
			`{"timestamp":"2026-04-03T12:00:02Z","level":"info","boot_id":"old_boot","message":"Old boot entry"}`,
			`{"timestamp":"2026-04-03T12:00:03Z","level":"error","boot_id":"boot_123","message":"Failed to open compozy.db"}`,
		}, "\n")
		if err := os.WriteFile(filepath.Join(homePaths.LogsDir, "desktop.log"), []byte(desktopLog), 0o600); err != nil {
			t.Fatalf("WriteFile(desktop diagnostic log) error = %v", err)
		}
		bootstrapLog := `{"timestamp":"2026-04-03T12:00:03Z","level":"warn","boot_id":"boot_123","message":"Bootstrap warning"}`
		if err := os.WriteFile(
			filepath.Join(homePaths.LogsDir, "desktop-bootstrap.jsonl"),
			[]byte(bootstrapLog),
			0o600,
		); err != nil {
			t.Fatalf("WriteFile(bootstrap diagnostic log) error = %v", err)
		}
		if err := os.WriteFile(homePaths.LogFile, []byte(desktopLog), 0o600); err != nil {
			t.Fatalf("WriteFile(runtime log) error = %v", err)
		}
		deps := appTestDeps(homePaths)
		deps.now = func() time.Time { return fixedTestNow }
		deps.callAppControl = func(context.Context, string, string, any) (any, error) {
			return nil, newAppCommandError(appNotRunningCode, "the CompozyOS desktop app is not running", nil)
		}

		stdout, err := executeAppTestCommand(t, deps, "app", "diagnose", "--bundle", "--yes", "-o", "json")
		if err != nil {
			t.Fatalf("offline app diagnose bundle error = %v", err)
		}
		var result appDiagnosticBundleResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("decode diagnostic bundle result: %v", err)
		}
		wantPath := filepath.Join(
			homePaths.HomeDir,
			supportBundlesDirName,
			"compozyos-desktop-diagnostics-20260403T120000000Z.tar.gz",
		)
		if result.Bundle.Path != wantPath {
			t.Fatalf("diagnostic bundle path = %q, want %q", result.Bundle.Path, wantPath)
		}
		if result.Bundle.Manifest.SchemaVersion != appDiagnosticBundleManifestSchemaVersion {
			t.Fatalf(
				"diagnostic bundle manifest = %#v, want schema version %d",
				result.Bundle.Manifest,
				appDiagnosticBundleManifestSchemaVersion,
			)
		}
		if result.Bundle.Manifest.Report.BootID != "boot_123" {
			t.Fatalf("diagnostic bundle manifest report = %#v, want shared report", result.Bundle.Manifest.Report)
		}
		if runtime.GOOS != "windows" {
			directory, statErr := os.Stat(filepath.Dir(result.Bundle.Path))
			if statErr != nil {
				t.Fatalf("Stat(bundle directory) error = %v", statErr)
			}
			if directory.Mode().Perm() != 0o700 {
				t.Fatalf("bundle directory permissions = %#o, want 0700", directory.Mode().Perm())
			}
			archive, statErr := os.Stat(result.Bundle.Path)
			if statErr != nil {
				t.Fatalf("Stat(bundle archive) error = %v", statErr)
			}
			if archive.Mode().Perm() != 0o600 {
				t.Fatalf("bundle archive permissions = %#o, want 0600", archive.Mode().Perm())
			}
		}
		contents := readAppDiagnosticBundleArchive(t, result.Bundle.Path)
		if len(contents) != 3 || contents["manifest.json"] == nil || contents["desktop.log"] == nil ||
			contents["desktop-bootstrap.jsonl"] == nil || contents["compozy.log"] != nil {
			t.Fatalf("diagnostic archive contents = %#v, want only the approved entries", contents)
		}
		if !bytes.Contains(contents["desktop.log"], []byte("Desktop started from [redacted]")) ||
			!bytes.Contains(contents["desktop-bootstrap.jsonl"], []byte("Bootstrap warning")) {
			t.Fatalf("diagnostic archive log tails = %#v, want redacted current-boot entries", contents)
		}
		for _, content := range contents {
			for _, forbidden := range []string{
				homePaths.HomeDir,
				"raw-secret",
				"transcript",
				"compozy.db",
				"Old boot entry",
			} {
				if bytes.Contains(content, []byte(forbidden)) {
					t.Fatalf("diagnostic archive contains forbidden value %q", forbidden)
				}
			}
		}
	})

	t.Run("Should repair the default bundle directory permissions", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("file mode assertions are not portable on Windows")
		}
		t.Parallel()
		homePaths := appTestHome(t)
		defaultDirectory := filepath.Join(homePaths.HomeDir, supportBundlesDirName)
		if err := os.Mkdir(defaultDirectory, 0o700); err != nil {
			t.Fatalf("Mkdir(default bundle directory) error = %v", err)
		}
		if err := os.Chmod(defaultDirectory, 0o755); err != nil {
			t.Fatalf("Chmod(default bundle directory) error = %v", err)
		}
		writeAppTestRecord(t, homePaths, map[string]any{
			"schema_version":    2,
			"pid":               4242,
			"started_at":        "2026-08-10T03:00:00Z",
			"state":             "resolving",
			"diagnostic_report": appDiagnosticReportFixture(),
		})
		deps := appTestDeps(homePaths)
		deps.now = func() time.Time { return fixedTestNow }
		deps.callAppControl = func(context.Context, string, string, any) (any, error) {
			return nil, newAppCommandError(appNotRunningCode, "the CompozyOS desktop app is not running", nil)
		}
		_, err := executeAppTestCommand(t, deps, "app", "diagnose", "--bundle", "--yes")
		if err != nil {
			t.Fatalf("app diagnose default bundle error = %v", err)
		}
		info, err := os.Stat(defaultDirectory)
		if err != nil {
			t.Fatalf("Stat(default bundle directory) error = %v", err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Fatalf("default bundle directory permissions = %#o, want 0700", info.Mode().Perm())
		}
	})

	t.Run("Should reject a symlink bundle destination", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("symlink creation requires platform-specific privileges")
		}
		homePaths := appTestHome(t)
		writeAppTestRecord(t, homePaths, map[string]any{
			"schema_version":    2,
			"pid":               4242,
			"started_at":        "2026-08-10T03:00:00Z",
			"state":             "resolving",
			"diagnostic_report": appDiagnosticReportFixture(),
		})
		destination := filepath.Join(t.TempDir(), "diagnostic.tar.gz")
		if err := os.Symlink(filepath.Join(t.TempDir(), "target"), destination); err != nil {
			t.Fatalf("Symlink(bundle destination) error = %v", err)
		}
		deps := appTestDeps(homePaths)
		deps.callAppControl = func(context.Context, string, string, any) (any, error) {
			return nil, newAppCommandError(appNotRunningCode, "the CompozyOS desktop app is not running", nil)
		}
		_, err := executeAppTestCommand(
			t,
			deps,
			"app",
			"diagnose",
			"--bundle",
			"--yes",
			"--bundle-output",
			destination,
		)
		if err == nil || !strings.Contains(err.Error(), "must not be a symlink") {
			t.Fatalf("symlink bundle destination error = %v, want rejection", err)
		}
	})

	t.Run("Should reject a symlink in the bundle destination ancestry", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlink creation requires platform-specific privileges")
		}
		t.Parallel()
		homePaths := appTestHome(t)
		writeAppTestRecord(t, homePaths, map[string]any{
			"schema_version":    2,
			"pid":               4242,
			"started_at":        "2026-08-10T03:00:00Z",
			"state":             "resolving",
			"diagnostic_report": appDiagnosticReportFixture(),
		})
		outputRoot := t.TempDir()
		if err := os.Symlink(t.TempDir(), filepath.Join(outputRoot, "redirect")); err != nil {
			t.Fatalf("Symlink(bundle ancestor) error = %v", err)
		}
		deps := appTestDeps(homePaths)
		deps.callAppControl = func(context.Context, string, string, any) (any, error) {
			return nil, newAppCommandError(appNotRunningCode, "the CompozyOS desktop app is not running", nil)
		}
		_, err := executeAppTestCommand(
			t,
			deps,
			"app",
			"diagnose",
			"--bundle",
			"--yes",
			"--bundle-output",
			filepath.Join(outputRoot, "redirect", "nested", "diagnostic.tar.gz"),
		)
		if err == nil || !strings.Contains(err.Error(), "ancestry must not contain symlinks") {
			t.Fatalf("symlink bundle ancestry error = %v, want rejection", err)
		}
	})

	t.Run("Should limit macOS system symlink aliases", func(t *testing.T) {
		t.Parallel()
		if !isAppDiagnosticBundleSystemAlias("/var") {
			if isAppDiagnosticBundleSystemAlias("/tmp") || isAppDiagnosticBundleSystemAlias("/etc") {
				t.Fatal("non-macOS bundle output must not allow a system symlink alias")
			}
			return
		}
		for _, path := range []string{"/var", "/tmp", "/etc"} {
			if !isAppDiagnosticBundleSystemAlias(path) {
				t.Fatalf("macOS system alias %q = false, want true", path)
			}
		}
		if isAppDiagnosticBundleSystemAlias("/usr/local") {
			t.Fatal("non-allowlisted macOS symlink alias = true, want false")
		}
	})

	t.Run("Should preserve an existing bundle output directory mode", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("file mode assertions are not portable on Windows")
		}
		t.Parallel()
		homePaths := appTestHome(t)
		writeAppTestRecord(t, homePaths, map[string]any{
			"schema_version":    2,
			"pid":               4242,
			"started_at":        "2026-08-10T03:00:00Z",
			"state":             "resolving",
			"diagnostic_report": appDiagnosticReportFixture(),
		})
		outputDirectory := filepath.Join(t.TempDir(), "operator-output")
		if err := os.Mkdir(outputDirectory, 0o755); err != nil {
			t.Fatalf("Mkdir(operator output) error = %v", err)
		}
		if err := os.Chmod(outputDirectory, 0o755); err != nil {
			t.Fatalf("Chmod(operator output) error = %v", err)
		}
		deps := appTestDeps(homePaths)
		deps.callAppControl = func(context.Context, string, string, any) (any, error) {
			return nil, newAppCommandError(appNotRunningCode, "the CompozyOS desktop app is not running", nil)
		}
		_, err := executeAppTestCommand(
			t,
			deps,
			"app",
			"diagnose",
			"--bundle",
			"--yes",
			"--bundle-output",
			filepath.Join(outputDirectory, "diagnostic.tar.gz"),
		)
		if err != nil {
			t.Fatalf("app diagnose explicit bundle output error = %v", err)
		}
		info, err := os.Stat(outputDirectory)
		if err != nil {
			t.Fatalf("Stat(operator output) error = %v", err)
		}
		if info.Mode().Perm() != 0o755 {
			t.Fatalf("operator output directory permissions = %#o, want 0755", info.Mode().Perm())
		}
	})

	t.Run("Should leave an existing bundle destination unchanged", func(t *testing.T) {
		t.Parallel()
		homePaths := appTestHome(t)
		writeAppTestRecord(t, homePaths, map[string]any{
			"schema_version":    2,
			"pid":               4242,
			"started_at":        "2026-08-10T03:00:00Z",
			"state":             "resolving",
			"diagnostic_report": appDiagnosticReportFixture(),
		})
		destination := filepath.Join(t.TempDir(), "existing.tar.gz")
		existing := []byte("existing diagnostic bundle")
		if err := os.WriteFile(destination, existing, 0o600); err != nil {
			t.Fatalf("WriteFile(existing bundle) error = %v", err)
		}
		deps := appTestDeps(homePaths)
		deps.callAppControl = func(context.Context, string, string, any) (any, error) {
			return nil, newAppCommandError(appNotRunningCode, "the CompozyOS desktop app is not running", nil)
		}
		_, err := executeAppTestCommand(
			t,
			deps,
			"app",
			"diagnose",
			"--bundle",
			"--yes",
			"--bundle-output",
			destination,
		)
		if err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("existing bundle destination error = %v, want rejection", err)
		}
		actual, readErr := os.ReadFile(destination)
		if readErr != nil {
			t.Fatalf("ReadFile(existing bundle) error = %v", readErr)
		}
		if !bytes.Equal(actual, existing) {
			t.Fatalf("existing bundle contents = %q, want %q", actual, existing)
		}
	})

	t.Run("Should not remove a replaced destination during failed cleanup", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("hard-link semantics are not portable on Windows")
		}
		t.Parallel()
		directory := t.TempDir()
		temporaryPath := filepath.Join(directory, ".diagnostic.tmp")
		destination := filepath.Join(directory, "diagnostic.tar.gz")
		if err := os.WriteFile(temporaryPath, []byte("temporary bundle"), 0o600); err != nil {
			t.Fatalf("WriteFile(temporary bundle) error = %v", err)
		}
		if err := os.Link(temporaryPath, destination); err != nil {
			t.Fatalf("Link(temporary bundle) error = %v", err)
		}
		if err := os.Remove(destination); err != nil {
			t.Fatalf("Remove(linked destination) error = %v", err)
		}
		replacement := []byte("replacement bundle")
		if err := os.WriteFile(destination, replacement, 0o600); err != nil {
			t.Fatalf("WriteFile(replacement bundle) error = %v", err)
		}
		if err := removeLinkedAppDiagnosticBundle(temporaryPath, destination); err != nil {
			t.Fatalf("remove linked bundle cleanup error = %v", err)
		}
		actual, err := os.ReadFile(destination)
		if err != nil {
			t.Fatalf("ReadFile(replacement bundle) error = %v", err)
		}
		if !bytes.Equal(actual, replacement) {
			t.Fatalf("replacement bundle contents = %q, want %q", actual, replacement)
		}
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
		for _, target := range []string{
			"../evil",
			"http://evil.example",
			"/sessions/%2e%2e/admin",
			"/sessions/%2e%2e%2fadmin",
			"/sessions/abc#fragment",
		} {
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

	t.Run("Should reject a future app state before navigating a live process", func(t *testing.T) {
		t.Parallel()
		homePaths := appTestHome(t)
		writeAppTestRecord(t, homePaths, map[string]any{
			"schema_version": 3,
			"pid":            4242,
			"started_at":     "2026-08-10T03:00:00Z",
			"state":          "product",
		})
		deps := appTestDeps(homePaths)
		deps.resolveAppInstallation = func(context.Context, compozyconfig.HomePaths) (appInstallation, error) {
			return appInstallation{Installed: true, Version: "0.3.0"}, nil
		}
		deps.processAlive = func(int) bool { return true }
		deps.processMatchesStartTime = func(int, time.Time) bool { return true }
		controlCalled := false
		deps.callAppControl = func(context.Context, string, string, any) (any, error) {
			controlCalled = true
			return nil, nil
		}

		_, err := executeAppTestCommand(t, deps, "app", "open")
		assertAppCommandError(t, err, appStateSchemaUnknownCode)
		if controlCalled {
			t.Fatal("app control called for an unsupported app state schema")
		}
	})
}

func TestAppControlReportsDeterministicTransportErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should keep long control deadlines for retry and diagnostics", func(t *testing.T) {
		t.Parallel()
		for _, test := range []struct {
			name   string
			method string
		}{
			{name: "Should extend retry", method: appRetryMethod},
			{name: "Should extend diagnostics export", method: appExportDiagnosticsMethod},
		} {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				if got := appControlTimeoutForMethod(test.method); got != appControlLongTimeout {
					t.Fatalf("appControlTimeoutForMethod(%q) = %s, want %s", test.method, got, appControlLongTimeout)
				}
			})
		}
	})

	t.Run("Should report app_not_running when the socket is absent", func(t *testing.T) {
		t.Parallel()
		_, err := callAppControl(t.Context(), filepath.Join(t.TempDir(), "app.sock"), "retry", nil)
		assertAppCommandError(t, err, appNotRunningCode)
	})

	t.Run("Should authenticate the request with the current control token", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == platformWindows {
			t.Skip("Unix control sockets are not available on Windows")
		}
		directory, err := os.MkdirTemp("/tmp", "ca-")
		if err != nil {
			t.Fatalf("MkdirTemp(app control) error = %v", err)
		}
		t.Cleanup(func() {
			if removeErr := os.RemoveAll(directory); removeErr != nil {
				t.Errorf("RemoveAll(app control directory) error = %v", removeErr)
			}
		})
		socketPath := filepath.Join(directory, "app.sock")
		if err := os.WriteFile(filepath.Join(directory, "app.token"), []byte("token-current\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(app token) error = %v", err)
		}
		var listenConfig net.ListenConfig
		listener, err := listenConfig.Listen(t.Context(), "unix", socketPath)
		if err != nil {
			t.Fatalf("Listen(app socket) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := listener.Close(); closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
				t.Errorf("Close(app listener) error = %v", closeErr)
			}
		})
		requests := make(chan appControlRequest, 1)
		serverErrors := make(chan error, 1)
		go func() {
			connection, acceptErr := listener.Accept()
			if acceptErr != nil {
				serverErrors <- acceptErr
				return
			}
			var request appControlRequest
			if decodeErr := json.NewDecoder(connection).Decode(&request); decodeErr != nil {
				serverErrors <- errors.Join(decodeErr, connection.Close())
				return
			}
			requests <- request
			response := appControlResponse{
				SchemaVersion: appControlSchemaVersion,
				ID:            request.ID,
				Result:        json.RawMessage(`{"ok":true}`),
			}
			if encodeErr := json.NewEncoder(connection).Encode(response); encodeErr != nil {
				serverErrors <- errors.Join(encodeErr, connection.Close())
				return
			}
			serverErrors <- connection.Close()
		}()

		result, err := callAppControl(t.Context(), socketPath, "diagnose", map[string]any{})
		if err != nil {
			t.Fatalf("callAppControl() error = %v", err)
		}
		request := <-requests
		if request.Token != "token-current" || request.Method != "diagnose" {
			t.Fatalf("control request = %#v, want current token and diagnose method", request)
		}
		if result == nil {
			t.Fatal("callAppControl() result = nil, want success body")
		}
		if serverErr := <-serverErrors; serverErr != nil {
			t.Fatalf("app control server error = %v", serverErr)
		}
	})

	t.Run("Should report app_control_unavailable when the socket is present but unresponsive", func(t *testing.T) {
		t.Parallel()
		socketPath := filepath.Join(t.TempDir(), "app.sock")
		if err := os.WriteFile(socketPath, []byte("stale"), 0o600); err != nil {
			t.Fatalf("WriteFile(socket) error = %v", err)
		}
		_, err := callAppControl(t.Context(), socketPath, "navigate", map[string]string{"path": "/"})
		assertAppCommandError(t, err, appControlUnavailableCode)
	})

	t.Run("Should reject the removed app update command", func(t *testing.T) {
		t.Parallel()
		homePaths := appTestHome(t)
		_, err := executeAppTestCommand(t, appTestDeps(homePaths), "app", "update", "--check")
		if err == nil || !strings.Contains(err.Error(), `unknown command "update"`) {
			t.Fatalf("app update error = %v, want unknown command", err)
		}
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

	t.Run("Should keep a registered macOS app installed when its version cannot be read", func(t *testing.T) {
		t.Parallel()
		installation, err := resolveDarwinAppInstallation(t.Context(), func(
			_ context.Context,
			name string,
			_ ...string,
		) (string, error) {
			if name == "mdfind" {
				return "/Applications/CompozyOS.app\n", nil
			}
			return "", errors.New("version metadata unavailable")
		})
		if err != nil || !installation.Installed || installation.Version != "" {
			t.Fatalf("macOS installation = %#v error=%v, want installed without version", installation, err)
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

func appDiagnosticReportFixture() map[string]any {
	return map[string]any{
		"schema_version":  1,
		"boot_id":         "boot_123",
		"boot_phase":      "product",
		"app_version":     "0.3.0",
		"runtime_version": "0.3.0",
		"runtime_owned":   true,
		"current_error": map[string]any{
			"code":         "runtime_unhealthy",
			"safe_message": "The runtime is not responding.",
		},
		"previous_crash": nil,
	}
}

func readAppDiagnosticBundleArchive(t *testing.T, path string) map[string][]byte {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open(diagnostic bundle) error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("Close(diagnostic bundle) error = %v", closeErr)
		}
	})
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("NewReader(diagnostic bundle) error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := gzipReader.Close(); closeErr != nil {
			t.Errorf("Close(diagnostic bundle gzip reader) error = %v", closeErr)
		}
	})

	contents := make(map[string][]byte)
	reader := tar.NewReader(gzipReader)
	for {
		header, nextErr := reader.Next()
		if errors.Is(nextErr, io.EOF) {
			return contents
		}
		if nextErr != nil {
			t.Fatalf("Next(diagnostic bundle archive) error = %v", nextErr)
		}
		data, readErr := io.ReadAll(reader)
		if readErr != nil {
			t.Fatalf("ReadAll(diagnostic bundle entry %q) error = %v", header.Name, readErr)
		}
		contents[header.Name] = data
	}
}
