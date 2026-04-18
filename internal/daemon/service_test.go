package daemon

import (
	"context"
	"strings"
	"testing"
	"time"

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
			"run-a": {runID: "run-a"},
			"run-b": {runID: "run-b"},
		},
	}
	service := NewService(ServiceConfig{
		Host:            host,
		GlobalDB:        db,
		RunManager:      manager,
		ReconcileResult: ReconcileResult{ReconciledRuns: 3, CrashEventFailures: 1},
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
	if len(health.Details) != 1 || health.Details[0].Code != "startup_reconcile_warnings" {
		t.Fatalf("health.Details = %#v, want startup reconcile warning", health.Details)
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
		"runs_reconciled_crashed_total 3",
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

	metrics, err := service.Metrics(context.Background())
	if err != nil {
		t.Fatalf("Metrics() error = %v", err)
	}
	for _, fragment := range []string{
		"daemon_active_runs 0",
		"daemon_registered_workspaces 0",
		"daemon_shutdown_conflicts_total 0",
		"runs_reconciled_crashed_total 0",
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
