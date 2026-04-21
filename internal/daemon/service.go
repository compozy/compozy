package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store/globaldb"
)

var (
	daemonMetricModes          = []string{runModeExec, runModeReview, runModeTask}
	daemonTerminalMetricStatus = []string{runStatusCancelled, runStatusCompleted, runStatusCrashed, runStatusFailed}
)

// ServiceConfig wires the daemon-wide status, health, metrics, and stop
// surface exposed to transports.
type ServiceConfig struct {
	Host              *Host
	GlobalDB          *globaldb.GlobalDB
	RunManager        *RunManager
	RequestStop       func(context.Context) error
	ReconcileResult   ReconcileResult
	LifecycleSettings RunLifecycleSettings
	Now               func() time.Time
}

// Service implements the shared transport-facing daemon control surface.
type Service struct {
	host            *Host
	globalDB        *globaldb.GlobalDB
	runManager      *RunManager
	requestStop     func(context.Context) error
	reconcileResult ReconcileResult
	settings        RunLifecycleSettings
	now             func() time.Time

	shutdownConflicts atomic.Int64
}

var _ apicore.DaemonService = (*Service)(nil)

// NewService constructs the daemon-wide control service.
func NewService(cfg ServiceConfig) *Service {
	now := cfg.Now
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}
	settings := cfg.LifecycleSettings
	if settings.ShutdownDrainTimeout <= 0 {
		settings.ShutdownDrainTimeout = defaultShutdownDrainTimeout
	}
	return &Service{
		host:            cfg.Host,
		globalDB:        cfg.GlobalDB,
		runManager:      cfg.RunManager,
		requestStop:     cfg.RequestStop,
		reconcileResult: cfg.ReconcileResult,
		settings:        settings,
		now:             now,
	}
}

// Status returns the current daemon status snapshot.
func (s *Service) Status(ctx context.Context) (apicore.DaemonStatus, error) {
	info := s.currentInfo()
	workspaceCount, err := s.countWorkspaces(ctx)
	if err != nil {
		return apicore.DaemonStatus{}, err
	}

	status := apicore.DaemonStatus{
		PID:            info.PID,
		Version:        info.Version,
		StartedAt:      info.StartedAt,
		SocketPath:     info.SocketPath,
		HTTPPort:       info.HTTPPort,
		ActiveRunCount: s.activeRunCount(),
		WorkspaceCount: workspaceCount,
	}
	return status, nil
}

// Health reports readiness and any degraded state known to the daemon.
func (s *Service) Health(ctx context.Context) (apicore.DaemonHealth, error) {
	workspaceCount, err := s.countWorkspaces(ctx)
	if err != nil {
		return apicore.DaemonHealth{}, err
	}

	info := s.currentInfo()
	databases, err := s.databaseDiagnostics()
	if err != nil {
		return apicore.DaemonHealth{}, err
	}
	activeByMode := sortedModeCounts(s.activeRunCountsByMode())
	activeRunCount := s.activeRunCount()
	health := apicore.DaemonHealth{
		Ready:            strings.EqualFold(string(info.State), string(ReadyStateReady)),
		StartedAt:        info.StartedAt,
		UptimeSeconds:    uptimeSeconds(info.StartedAt, s.now()),
		ActiveRunCount:   activeRunCount,
		ActiveRunsByMode: activeByMode,
		WorkspaceCount:   workspaceCount,
		Databases:        databases,
		Reconcile: apicore.DaemonReconcileDiagnostics{
			ReconciledRuns:     s.reconcileResult.ReconciledRuns,
			CrashEventAppended: s.reconcileResult.CrashEventAppended,
			CrashEventMissing:  s.reconcileResult.CrashEventFailures,
			LastRunID:          strings.TrimSpace(s.reconcileResult.LastReconciledRunID),
		},
		IntegrityIssueCount: s.incompleteRunCount(),
	}
	if !health.Ready {
		health.Details = []apicore.HealthDetail{{
			Code:     "daemon_not_ready",
			Message:  fmt.Sprintf("daemon is %s", info.State),
			Severity: "error",
		}}
		return health, nil
	}
	if s.reconcileResult.CrashEventFailures > 0 {
		health.Degraded = true
		health.Details = append(health.Details, apicore.HealthDetail{
			Code:     "startup_reconcile_warnings",
			Message:  "one or more recovered runs could not persist a synthetic crash event",
			Severity: "warning",
		})
	}
	if health.IntegrityIssueCount > 0 {
		health.Degraded = true
		health.Details = append(health.Details, apicore.HealthDetail{
			Code:     "run_integrity_issues",
			Message:  fmt.Sprintf("%d run(s) have persisted integrity issues", health.IntegrityIssueCount),
			Severity: "warning",
		})
	}
	return health, nil
}

// Metrics returns the minimal daemon metrics required by the current transport
// contract and lifecycle task set.
func (s *Service) Metrics(ctx context.Context) (apicore.MetricsPayload, error) {
	workspaceCount, err := s.countWorkspaces(ctx)
	if err != nil {
		return apicore.MetricsPayload{}, err
	}

	var builder strings.Builder
	s.writeCoreMetrics(&builder, workspaceCount)
	s.writeReconcileMetrics(&builder)
	s.writeJournalDropMetrics(&builder)
	s.writeRunTerminalMetrics(&builder)
	s.writeACPStallMetrics(&builder)
	s.writeUptimeMetric(&builder)
	return apicore.MetricsPayload{
		Body:        builder.String(),
		ContentType: "text/plain; version=0.0.4; charset=utf-8",
	}, nil
}

func (s *Service) writeCoreMetrics(builder *strings.Builder, workspaceCount int) {
	writePrometheusMetricPrelude(builder, "daemon_active_runs", "gauge", "Current live runs owned by the daemon")
	fmt.Fprintf(builder, "daemon_active_runs %d\n", s.activeRunCount())

	writePrometheusMetricPrelude(builder, "daemon_registered_workspaces", "gauge", "Current registered workspaces")
	fmt.Fprintf(builder, "daemon_registered_workspaces %d\n", workspaceCount)

	writePrometheusMetricPrelude(
		builder,
		"daemon_shutdown_conflicts_total",
		"counter",
		"Stop requests rejected due to active runs",
	)
	fmt.Fprintf(builder, "daemon_shutdown_conflicts_total %d\n", s.shutdownConflicts.Load())
}

func (s *Service) writeReconcileMetrics(builder *strings.Builder) {
	writePrometheusMetricPrelude(
		builder,
		"daemon_reconcile_runs_total",
		"counter",
		"Runs processed by reconcile logic",
	)
	for _, item := range []struct {
		crashEvent     string
		classification string
		total          int
	}{
		{crashEvent: "appended", classification: "crashed", total: s.reconcileResult.CrashEventAppended},
		{crashEvent: "missing", classification: "crashed", total: s.reconcileResult.CrashEventFailures},
		{crashEvent: "appended", classification: "orphaned", total: 0},
		{crashEvent: "missing", classification: "orphaned", total: 0},
	} {
		fmt.Fprintf(
			builder,
			"daemon_reconcile_runs_total{crash_event=%q,classification=%q} %d\n",
			item.crashEvent,
			item.classification,
			item.total,
		)
	}
}

func (s *Service) writeJournalDropMetrics(builder *strings.Builder) {
	terminalDrops, nonTerminalDrops := s.journalSubmitDropTotals()
	writePrometheusMetricPrelude(
		builder,
		"daemon_journal_submit_drops_total",
		"counter",
		"Event submissions dropped by journal backpressure",
	)
	fmt.Fprintf(builder, "daemon_journal_submit_drops_total{kind=%q} %d\n", "terminal", terminalDrops)
	fmt.Fprintf(builder, "daemon_journal_submit_drops_total{kind=%q} %d\n", "non_terminal", nonTerminalDrops)
}

func (s *Service) writeRunTerminalMetrics(builder *strings.Builder) {
	terminalTotals := s.terminalTotalsByModeAndStatus()
	writePrometheusMetricPrelude(
		builder,
		"daemon_run_terminal_total",
		"counter",
		"Terminal run outcomes",
	)
	for _, mode := range daemonMetricModes {
		for _, status := range daemonTerminalMetricStatus {
			fmt.Fprintf(
				builder,
				"daemon_run_terminal_total{mode=%q,status=%q} %d\n",
				mode,
				status,
				readNestedMetricTotal(terminalTotals, mode, status),
			)
		}
	}
}

func (s *Service) writeACPStallMetrics(builder *strings.Builder) {
	stallTotals := s.acpStallTotalsByMode()
	writePrometheusMetricPrelude(
		builder,
		"daemon_acp_stall_total",
		"counter",
		"Jobs classified as stalled by liveness monitoring",
	)
	for _, mode := range daemonMetricModes {
		fmt.Fprintf(builder, "daemon_acp_stall_total{mode=%q} %d\n", mode, stallTotals[mode])
	}
}

func (s *Service) writeUptimeMetric(builder *strings.Builder) {
	writePrometheusMetricPrelude(
		builder,
		"daemon_uptime_seconds",
		"gauge",
		"Uptime since daemon start",
	)
	fmt.Fprintf(builder, "daemon_uptime_seconds %d\n", uptimeSeconds(s.currentInfo().StartedAt, s.now()))
}

// Stop enforces the daemon stop contract, delegating active-run ownership to
// the daemon run manager and then invoking the host stop callback.
func (s *Service) Stop(ctx context.Context, force bool) error {
	activeRunCount := s.activeRunCount()
	slog.Default().Info("daemon stop requested", "force", force, "active_run_count", activeRunCount)
	if s.runManager != nil {
		if err := s.runManager.Shutdown(ctx, force); err != nil {
			s.shutdownConflicts.Add(1)
			slog.Default().
				Warn("daemon stop rejected", "force", force, "active_run_count", activeRunCount, "error", err)
			return err
		}
	}
	if s.requestStop != nil {
		if err := s.requestStop(detachContext(ctx)); err != nil {
			slog.Default().Error("daemon stop callback failed", "force", force, "error", err)
			return err
		}
	}
	slog.Default().Info("daemon stop accepted", "force", force, "active_run_count", activeRunCount)
	return nil
}

func (s *Service) currentInfo() Info {
	if s == nil || s.host == nil {
		return Info{State: ReadyStateStopped}
	}
	return s.host.Info()
}

func (s *Service) activeRunCount() int {
	if s == nil || s.runManager == nil {
		return 0
	}
	return s.runManager.ActiveRunCount()
}

func (s *Service) activeRunCountsByMode() map[string]int {
	if s == nil || s.runManager == nil {
		return nil
	}
	return s.runManager.ActiveRunCountsByMode()
}

func (s *Service) terminalTotalsByModeAndStatus() map[string]map[string]uint64 {
	if s == nil || s.runManager == nil {
		return nil
	}
	return s.runManager.TerminalTotalsByModeAndStatus()
}

func (s *Service) acpStallTotalsByMode() map[string]uint64 {
	if s == nil || s.runManager == nil {
		return nil
	}
	return s.runManager.ACPStallTotalsByMode()
}

func (s *Service) journalSubmitDropTotals() (uint64, uint64) {
	if s == nil || s.runManager == nil {
		return 0, 0
	}
	return s.runManager.JournalSubmitDropTotals()
}

func (s *Service) incompleteRunCount() int {
	if s == nil || s.runManager == nil {
		return 0
	}
	return s.runManager.IncompleteRunCount()
}

func (s *Service) countWorkspaces(ctx context.Context) (int, error) {
	if s == nil || s.globalDB == nil {
		return 0, nil
	}
	return s.globalDB.CountWorkspaces(detachContext(ctx))
}

func (s *Service) databaseDiagnostics() (apicore.DaemonDatabaseDiagnostics, error) {
	paths := s.hostPaths()
	globalBytes, err := databaseSize(paths.GlobalDBPath)
	if err != nil {
		return apicore.DaemonDatabaseDiagnostics{}, fmt.Errorf("daemon: measure global db size: %w", err)
	}
	runDBBytes, err := totalRunDBSize(paths.RunsDir)
	if err != nil {
		return apicore.DaemonDatabaseDiagnostics{}, fmt.Errorf("daemon: measure run db size: %w", err)
	}
	return apicore.DaemonDatabaseDiagnostics{
		GlobalBytes: globalBytes,
		RunDBBytes:  runDBBytes,
	}, nil
}

func (s *Service) hostPaths() compozyconfig.HomePaths {
	if s == nil || s.host == nil {
		return compozyconfig.HomePaths{}
	}
	return s.host.Paths()
}

func databaseSize(path string) (int64, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return 0, nil
	}

	var total int64
	for _, candidate := range []string{cleanPath, cleanPath + "-wal", cleanPath + "-shm"} {
		info, err := os.Stat(candidate)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return 0, err
		}
		if !info.IsDir() {
			total += info.Size()
		}
	}
	return total, nil
}

func totalRunDBSize(runsDir string) (int64, error) {
	cleanDir := strings.TrimSpace(runsDir)
	if cleanDir == "" {
		return 0, nil
	}

	entries, err := os.ReadDir(cleanDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	var total int64
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		size, sizeErr := databaseSize(filepath.Join(cleanDir, entry.Name(), "run.db"))
		if sizeErr != nil {
			return 0, sizeErr
		}
		total += size
	}
	return total, nil
}

func sortedModeCounts(counts map[string]int) []apicore.DaemonModeCount {
	if len(counts) == 0 {
		return nil
	}

	modes := make([]string, 0, len(counts))
	for mode := range counts {
		modes = append(modes, mode)
	}
	slices.Sort(modes)

	result := make([]apicore.DaemonModeCount, 0, len(modes))
	for _, mode := range modes {
		result = append(result, apicore.DaemonModeCount{Mode: mode, Count: counts[mode]})
	}
	return result
}

func uptimeSeconds(startedAt time.Time, now time.Time) int64 {
	if startedAt.IsZero() || now.IsZero() {
		return 0
	}
	if now.Before(startedAt) {
		return 0
	}
	return int64(now.Sub(startedAt).Seconds())
}

func writePrometheusMetricPrelude(builder *strings.Builder, name string, metricType string, help string) {
	builder.WriteString("# HELP ")
	builder.WriteString(name)
	builder.WriteByte(' ')
	builder.WriteString(help)
	builder.WriteByte('\n')
	builder.WriteString("# TYPE ")
	builder.WriteString(name)
	builder.WriteByte(' ')
	builder.WriteString(metricType)
	builder.WriteByte('\n')
}

func readNestedMetricTotal(values map[string]map[string]uint64, outer string, inner string) uint64 {
	if len(values) == 0 {
		return 0
	}
	return values[outer][inner]
}
