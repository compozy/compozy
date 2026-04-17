package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/core/model"
	workspacecfg "github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/rundb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

const (
	defaultKeepTerminalDays     = 14
	defaultKeepMax              = 200
	defaultShutdownDrainTimeout = 30 * time.Second
	defaultRunCloseTimeout      = time.Second
	sqliteHeader                = "SQLite format 3\x00"
)

// RunLifecycleSettings captures the daemon-owned retention and shutdown bounds
// resolved from the effective home-scoped config.
type RunLifecycleSettings struct {
	KeepTerminalDays     int
	KeepMax              int
	ShutdownDrainTimeout time.Duration
}

// ReconcileConfig controls startup crash reconciliation.
type ReconcileConfig struct {
	HomePaths    compozyconfig.HomePaths
	Now          func() time.Time
	OpenGlobalDB func(context.Context, string) (*globaldb.GlobalDB, error)
	OpenRunDB    func(context.Context, string) (*rundb.RunDB, error)
}

// ReconcileResult summarizes one startup reconciliation pass.
type ReconcileResult struct {
	ReconciledRuns      int
	CrashEventAppended  int
	CrashEventFailures  int
	LastReconciledRunID string
}

// LoadRunLifecycleSettings reads the home-scoped config and resolves the daemon
// run lifecycle defaults required for retention and forced shutdown behavior.
func LoadRunLifecycleSettings(ctx context.Context) (RunLifecycleSettings, string, error) {
	cfg, path, err := workspacecfg.LoadGlobalConfig(ctx)
	if err != nil {
		return RunLifecycleSettings{}, "", err
	}
	settings, err := resolveRunLifecycleSettings(cfg.Runs)
	if err != nil {
		return RunLifecycleSettings{}, path, err
	}
	return settings, path, nil
}

func resolveRunLifecycleSettings(cfg workspacecfg.RunsConfig) (RunLifecycleSettings, error) {
	settings := RunLifecycleSettings{
		KeepTerminalDays:     defaultKeepTerminalDays,
		KeepMax:              defaultKeepMax,
		ShutdownDrainTimeout: defaultShutdownDrainTimeout,
	}

	if cfg.KeepTerminalDays != nil {
		settings.KeepTerminalDays = *cfg.KeepTerminalDays
	}
	if cfg.KeepMax != nil {
		settings.KeepMax = *cfg.KeepMax
	}
	if cfg.ShutdownDrainTimeout != nil {
		duration, err := time.ParseDuration(strings.TrimSpace(*cfg.ShutdownDrainTimeout))
		if err != nil {
			return RunLifecycleSettings{}, fmt.Errorf("daemon: parse runs.shutdown_drain_timeout: %w", err)
		}
		settings.ShutdownDrainTimeout = duration
	}
	return settings, nil
}

// ReconcileStartup marks interrupted runs as crashed before the daemon reports
// ready. Missing or corrupt per-run databases do not block readiness; their
// failure is folded into the durable global error summary instead.
func ReconcileStartup(ctx context.Context, cfg ReconcileConfig) (ReconcileResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	paths := cfg.HomePaths
	if strings.TrimSpace(paths.HomeDir) == "" {
		resolvedPaths, err := resolveDaemonHomePaths()
		if err != nil {
			return ReconcileResult{}, fmt.Errorf("daemon: resolve home paths: %w", err)
		}
		paths = resolvedPaths
	}

	openGlobalDB := cfg.OpenGlobalDB
	if openGlobalDB == nil {
		openGlobalDB = globaldb.Open
	}
	db, err := openGlobalDB(ctx, paths.GlobalDBPath)
	if err != nil {
		return ReconcileResult{}, err
	}
	defer func() {
		_ = db.Close()
	}()

	now := cfg.Now
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}
	reconciledAt := now().UTC()

	interrupted, err := db.ListInterruptedRuns(ctx)
	if err != nil {
		return ReconcileResult{}, err
	}

	openRunDB := cfg.OpenRunDB
	if openRunDB == nil {
		openRunDB = rundb.Open
	}

	result := ReconcileResult{}
	for i := range interrupted {
		row := &interrupted[i]
		baseErrorText := buildCrashErrorText(row)
		errorText := baseErrorText
		if appendErr := appendSyntheticCrashEvent(
			ctx,
			openRunDB,
			row,
			reconciledAt,
			baseErrorText,
		); appendErr != nil {
			result.CrashEventFailures++
			errorText = buildCrashAppendFailureText(baseErrorText, appendErr)
		} else {
			result.CrashEventAppended++
		}

		if _, err := db.MarkRunCrashed(ctx, row.RunID, reconciledAt, errorText); err != nil {
			return result, err
		}
		result.ReconciledRuns++
		result.LastReconciledRunID = row.RunID
	}
	return result, nil
}

func appendSyntheticCrashEvent(
	ctx context.Context,
	openRunDB func(context.Context, string) (*rundb.RunDB, error),
	row *globaldb.Run,
	reconciledAt time.Time,
	errorText string,
) error {
	runArtifacts, err := model.ResolveHomeRunArtifacts(row.RunID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(runArtifacts.RunDBPath); err != nil {
		return err
	}
	if err := ensureSQLiteDatabaseFile(runArtifacts.RunDBPath); err != nil {
		return err
	}

	runDB, err := openRunDB(ctx, runArtifacts.RunDBPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = runDB.Close()
	}()

	durationMS := reconciledAt.Sub(row.StartedAt).Milliseconds()
	if durationMS < 0 {
		durationMS = 0
	}
	_, err = runDB.AppendSyntheticEvent(ctx, eventspkg.EventKindRunCrashed, kinds.RunCrashedPayload{
		ArtifactsDir: runArtifacts.RunDir,
		DurationMs:   durationMS,
		Error:        errorText,
		ResultPath:   runArtifacts.ResultPath,
	})
	return err
}

func buildCrashErrorText(row *globaldb.Run) string {
	base := "daemon stopped before run reached terminal state"
	if existing := strings.TrimSpace(row.ErrorText); existing != "" {
		return existing + "; " + base
	}
	return base
}

func buildCrashAppendFailureText(base string, err error) string {
	trimmedBase := strings.TrimSpace(base)
	if trimmedBase == "" {
		trimmedBase = "daemon stopped before run reached terminal state"
	}
	return fmt.Sprintf("%s; synthetic crash event unavailable: %v", trimmedBase, err)
}

func ensureSQLiteDatabaseFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	header := make([]byte, len(sqliteHeader))
	if _, err := io.ReadFull(file, header); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return fmt.Errorf("invalid sqlite header")
		}
		return err
	}
	if string(header) != sqliteHeader {
		return fmt.Errorf("invalid sqlite header")
	}
	return nil
}
