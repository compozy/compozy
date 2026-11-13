package cli

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	compozydaemon "github.com/compozy/compozy/internal/daemon"
)

func TestDaemonRelaunchCommandInvokesHelper(t *testing.T) {
	// not parallel: relaunch reads process environment through os.Getenv.
	t.Run("Should invoke relaunch helper with restart operation environment", func(t *testing.T) {
		deps := newTestDeps(t, &stubClient{})
		deps.executable = func() (string, error) { return "/usr/bin/compozy", nil }

		var captured compozydaemon.RelaunchHelperConfig
		deps.runRelaunchHelper = func(_ context.Context, cfg *compozydaemon.RelaunchHelperConfig) error {
			captured = *cfg
			return nil
		}

		t.Setenv(compozydaemon.RestartOperationEnvKey, "restart-op-123")

		if _, _, err := executeRootCommand(t, deps, "daemon", "relaunch"); err != nil {
			t.Fatalf("executeRootCommand() error = %v", err)
		}
		if got, want := strings.TrimSpace(captured.OperationID), "restart-op-123"; got != want {
			t.Fatalf("captured.OperationID = %q, want %q", got, want)
		}
		homePaths, err := deps.resolveHome()
		if err != nil {
			t.Fatalf("deps.resolveHome() error = %v", err)
		}
		if got, want := captured.HomePaths.HomeDir, homePaths.HomeDir; got != want {
			t.Fatalf("captured.HomePaths.HomeDir = %q, want %q", got, want)
		}
		if captured.Executable == nil {
			t.Fatal("captured.Executable = nil, want forwarded executable resolver")
		}
		// The replacement must wait through the entire configured shutdown ladder.
		runtime, err := loadRuntimeContext(deps)
		if err != nil {
			t.Fatal(err)
		}
		if captured.ReleaseTimeout <= compozydaemon.GracefulShutdownTimeout(&runtime.Config) ||
			captured.ReleaseTimeout > compozydaemon.GracefulShutdownTimeout(&runtime.Config)+5*time.Second {
			t.Fatalf("relaunch release budget = %s", captured.ReleaseTimeout)
		}
	})
}

func TestDaemonPreflightCommandReportsBootReadiness(t *testing.T) {
	// not parallel: this file's relaunch test uses t.Setenv.
	decodeReport := func(t *testing.T, stdout string) compozydaemon.RestartPreflightReport {
		t.Helper()
		line := strings.TrimSpace(stdout)
		if line == "" || strings.Contains(line, "\n") {
			t.Fatalf("stdout = %q, want exactly one report line", stdout)
		}
		var report compozydaemon.RestartPreflightReport
		if err := json.Unmarshal([]byte(line), &report); err != nil {
			t.Fatalf("json.Unmarshal(%q) error = %v", line, err)
		}
		if got, want := report.Schema, compozydaemon.RestartPreflightSchema; got != want {
			t.Fatalf("report.Schema = %q, want %q", got, want)
		}
		return report
	}

	t.Run("Should report a home this binary can boot and exit zero", func(t *testing.T) {
		deps := newTestDeps(t, &stubClient{})

		stdout, _, err := executeRootCommand(t, deps, "daemon", compozydaemon.RestartPreflightCommand)
		if err != nil {
			t.Fatalf("executeRootCommand() error = %v", err)
		}
		if report := decodeReport(t, stdout); !report.OK || len(report.Migrations) != 1 {
			t.Fatalf("report = %+v, want a passing report with the global migration plan", report)
		}
	})

	t.Run("Should report the config failure on one line and exit non-zero", func(t *testing.T) {
		deps := newTestDeps(t, &stubClient{})
		deps.loadConfig = func() (compozyconfig.Config, error) {
			return compozyconfig.Config{}, errors.New("config: unknown key agents.bogus")
		}

		stdout, _, err := executeRootCommand(t, deps, "daemon", compozydaemon.RestartPreflightCommand)
		assertErrorContains(t, err, "unknown key agents.bogus")
		report := decodeReport(t, stdout)
		if report.OK || report.Stage != compozydaemon.RestartPreflightStageConfig ||
			!strings.Contains(report.Error, "unknown key agents.bogus") {
			t.Fatalf("report = %+v, want the config failure", report)
		}
	})
}
