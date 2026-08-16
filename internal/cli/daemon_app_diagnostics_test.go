package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDaemonAppDiagnosticBundle(t *testing.T) {
	t.Run("Should export the canonical saved desktop diagnostics", func(t *testing.T) {
		t.Parallel()

		homePaths := appTestHome(t)
		writeAppTestRecord(t, homePaths, map[string]any{
			"schema_version": 2,
			"pid":            os.Getpid(),
			"started_at":     fixedTestNow,
			"app_version":    "0.3.0",
			"channel":        "development",
			"state":          "error",
			"error": map[string]any{
				"code": "runtime_unhealthy", "safe_message": "The runtime is not responding.",
				"log_path": filepath.Join(homePaths.LogsDir, "desktop.log"),
			},
			"diagnostic_report": appDiagnosticReportFixture(),
			"update": map[string]any{
				"app_state": "idle", "runtime_state": "idle",
			},
		})
		deps := appTestDeps(homePaths)
		stdout, _, err := executeRootCommand(t, deps, "daemon", "app-diagnostic-bundle")
		if err != nil {
			t.Fatalf("executeRootCommand(app-diagnostic-bundle) error = %v", err)
		}
		var result appDiagnosticBundleExportResult
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatalf("Unmarshal(bundle result) error = %v", err)
		}
		if result.BundlePath == "" || result.Bytes <= 0 {
			t.Fatalf("bundle result = %#v, want path and bytes", result)
		}
		manifest, err := readAppDiagnosticBundleManifest(result.BundlePath)
		if err != nil {
			t.Fatalf("readAppDiagnosticBundleManifest() error = %v", err)
		}
		if manifest.Report.BootID != "boot_123" {
			t.Fatalf("manifest report = %#v, want saved diagnostics", manifest.Report)
		}
	})
}
