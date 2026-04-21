package daemon

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestServiceStatusHealthAndMetricsReflectRuntimeState(t *testing.T) {
	paths := mustHomePaths(t)
	t.Setenv("HOME", paths.HomeDir)
	db := openDaemonGlobalDB(t, paths)
	workspace := registerDaemonWorkspace(t, db)

	now := time.Date(2026, 4, 17, 23, 0, 0, 0, time.UTC)
	host := &Host{
		info: Info{
			PID:        3030,
			Version:    "v1.2.3",
			SocketPath: paths.SocketPath,
			HTTPPort:   8787,
			StartedAt:  now,
			State:      ReadyStateReady,
		},
	}
	manager := &RunManager{
		active: map[string]*activeRun{
			"run-a": {runID: "run-a", mode: runModeTask},
			"run-b": {runID: "run-b", mode: runModeReview},
		},
		terminalTotals: map[string]uint64{
			joinRunManagerMetricKey(runModeTask, runStatusCompleted): 2,
			joinRunManagerMetricKey(runModeExec, runStatusFailed):    1,
		},
		acpStallTotals: map[string]uint64{
			runModeReview: 4,
		},
		journalTerminalDrops:    2,
		journalNonTerminalDrops: 3,
		journalDropsByRun: map[string]journalDropTotals{
			"run-gap": {terminal: 2, nonTerminal: 3},
		},
		incompleteRunIDs: map[string]struct{}{
			"run-gap": {},
		},
	}
	service := NewService(ServiceConfig{
		Host:            host,
		GlobalDB:        db,
		RunManager:      manager,
		ReconcileResult: ReconcileResult{ReconciledRuns: 3, CrashEventAppended: 2, CrashEventFailures: 1},
		Now:             func() time.Time { return now.Add(5 * time.Minute) },
	})

	status, err := service.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.PID != host.info.PID {
		t.Fatalf("status.PID = %d, want %d", status.PID, host.info.PID)
	}
	if status.Version != host.info.Version {
		t.Fatalf("status.Version = %q, want %q", status.Version, host.info.Version)
	}
	if status.HTTPPort != host.info.HTTPPort {
		t.Fatalf("status.HTTPPort = %d, want %d", status.HTTPPort, host.info.HTTPPort)
	}
	if status.ActiveRunCount != 2 {
		t.Fatalf("status.ActiveRunCount = %d, want 2", status.ActiveRunCount)
	}
	if status.WorkspaceCount != 1 || workspace.ID == "" {
		t.Fatalf("status.WorkspaceCount = %d, want 1", status.WorkspaceCount)
	}

	health, err := service.Health(context.Background())
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	if !health.Ready {
		t.Fatalf("health.Ready = false, want true")
	}
	if !health.Degraded {
		t.Fatalf("health.Degraded = false, want true")
	}
	if !health.StartedAt.Equal(now) {
		t.Fatalf("health.StartedAt = %v, want %v", health.StartedAt, now)
	}
	if health.UptimeSeconds != 300 {
		t.Fatalf("health.UptimeSeconds = %d, want 300", health.UptimeSeconds)
	}
	if health.ActiveRunCount != 2 {
		t.Fatalf("health.ActiveRunCount = %d, want 2", health.ActiveRunCount)
	}
	if got := health.ActiveRunsByMode; !slices.Equal(got, []apicore.DaemonModeCount{
		{Mode: runModeReview, Count: 1},
		{Mode: runModeTask, Count: 1},
	}) {
		t.Fatalf("health.ActiveRunsByMode = %#v, want review/task counts", got)
	}
	if health.WorkspaceCount != 1 {
		t.Fatalf("health.WorkspaceCount = %d, want 1", health.WorkspaceCount)
	}
	if health.IntegrityIssueCount != 1 {
		t.Fatalf("health.IntegrityIssueCount = %d, want 1", health.IntegrityIssueCount)
	}
	if health.Databases.GlobalBytes < 0 {
		t.Fatalf("health.Databases.GlobalBytes = %d, want >= 0", health.Databases.GlobalBytes)
	}
	if health.Databases.RunDBBytes != 0 {
		t.Fatalf("health.Databases.RunDBBytes = %d, want 0", health.Databases.RunDBBytes)
	}
	if health.Reconcile.ReconciledRuns != 3 || health.Reconcile.CrashEventAppended != 2 ||
		health.Reconcile.CrashEventMissing != 1 {
		t.Fatalf("health.Reconcile = %#v, want 3/2/1 counters", health.Reconcile)
	}
	if got := []string{health.Details[0].Code, health.Details[1].Code}; !slices.Equal(
		got,
		[]string{"startup_reconcile_warnings", "run_integrity_issues"},
	) {
		t.Fatalf("health.Details = %#v, want reconcile/integrity warnings", health.Details)
	}

	metrics, err := service.Metrics(context.Background())
	if err != nil {
		t.Fatalf("Metrics() error = %v", err)
	}
	if metrics.ContentType != "text/plain; version=0.0.4; charset=utf-8" {
		t.Fatalf("metrics.ContentType = %q", metrics.ContentType)
	}
	for _, fragment := range []string{
		"daemon_active_runs 2",
		"daemon_registered_workspaces 1",
		"daemon_shutdown_conflicts_total 0",
		`daemon_reconcile_runs_total{crash_event="appended",classification="crashed"} 2`,
		`daemon_reconcile_runs_total{crash_event="missing",classification="crashed"} 1`,
		`daemon_journal_submit_drops_total{kind="terminal"} 2`,
		`daemon_journal_submit_drops_total{kind="non_terminal"} 3`,
		`daemon_run_terminal_total{mode="task",status="completed"} 2`,
		`daemon_run_terminal_total{mode="exec",status="failed"} 1`,
		`daemon_acp_stall_total{mode="review"} 4`,
		"daemon_uptime_seconds 300",
	} {
		if !strings.Contains(metrics.Body, fragment) {
			t.Fatalf("metrics.Body missing %q in %q", fragment, metrics.Body)
		}
	}

	if got := manager.ActiveRunCount(); got != 2 {
		t.Fatalf("ActiveRunCount() = %d, want 2", got)
	}
}

func TestServiceDefaultsReportStoppedAndZeroCounts(t *testing.T) {
	t.Parallel()

	service := NewService(ServiceConfig{
		Host: &Host{
			paths: compozyconfig.HomePaths{},
			info:  Info{State: ReadyStateStopped},
		},
	})

	status, err := service.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.ActiveRunCount != 0 || status.WorkspaceCount != 0 {
		t.Fatalf("status = %#v, want zero counts", status)
	}

	health, err := service.Health(context.Background())
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	if health.Ready {
		t.Fatalf("health.Ready = true, want false")
	}
	if len(health.Details) != 1 || health.Details[0].Code != "daemon_not_ready" {
		t.Fatalf("health.Details = %#v, want daemon_not_ready", health.Details)
	}
	if health.UptimeSeconds != 0 || health.ActiveRunCount != 0 || health.WorkspaceCount != 0 ||
		health.IntegrityIssueCount != 0 {
		t.Fatalf("health counts = %#v, want zeros", health)
	}
	if len(health.ActiveRunsByMode) != 0 {
		t.Fatalf("health.ActiveRunsByMode = %#v, want empty", health.ActiveRunsByMode)
	}
	if health.Databases.GlobalBytes != 0 || health.Databases.RunDBBytes != 0 {
		t.Fatalf("health.Databases = %#v, want zero diagnostics", health.Databases)
	}

	metrics, err := service.Metrics(context.Background())
	if err != nil {
		t.Fatalf("Metrics() error = %v", err)
	}
	for _, fragment := range []string{
		"daemon_active_runs 0",
		"daemon_registered_workspaces 0",
		"daemon_shutdown_conflicts_total 0",
		`daemon_reconcile_runs_total{crash_event="missing",classification="crashed"} 0`,
		`daemon_journal_submit_drops_total{kind="terminal"} 0`,
		`daemon_run_terminal_total{mode="task",status="completed"} 0`,
		`daemon_acp_stall_total{mode="task"} 0`,
		"daemon_uptime_seconds 0",
	} {
		if !strings.Contains(metrics.Body, fragment) {
			t.Fatalf("metrics.Body missing %q in %q", fragment, metrics.Body)
		}
	}

	var nilManager *RunManager
	if got := nilManager.ActiveRunCount(); got != 0 {
		t.Fatalf("nil ActiveRunCount() = %d, want 0", got)
	}
}
