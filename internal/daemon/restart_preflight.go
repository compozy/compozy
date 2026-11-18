package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/procutil"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
)

const (
	// RestartPreflightCommand is the hidden `compozy daemon` subcommand a daemon runs on the
	// replacement binary before it signals itself for a restart.
	RestartPreflightCommand = "preflight"
	// RestartPreflightSchema identifies the single-line report RestartPreflightCommand prints.
	RestartPreflightSchema = "compozy.daemon.preflight/v1"
	// RestartPreflightStageConfig names a configuration the replacement cannot load or validate.
	RestartPreflightStageConfig = "config"
	// RestartPreflightStageMigrations names a global database the replacement refuses to migrate.
	RestartPreflightStageMigrations = "migration_plan"

	defaultRestartPreflightTimeout = 5 * time.Second
	restartPreflightKillWait       = time.Second
	restartPreflightOutputLimit    = 64 << 10
	// restartPreflightTextLimit bounds each free-text field of a report. JSON escaping can grow a
	// field sixfold, so the error and the warning together still fit restartPreflightOutputLimit.
	restartPreflightTextLimit      = 4 << 10
	restartPreflightTextTruncation = " [truncated]"
	restartPreflightFailureAction  = "restart pre-flight"
)

// ErrRestartPreflightFailed reports a replacement binary that positively cannot boot this home.
// The restart is refused before the running daemon is signaled.
var ErrRestartPreflightFailed = errors.New("replacement runtime cannot boot; the running daemon was kept")

// RestartPreflightReport is what one binary reports about booting a home without mutating it.
type RestartPreflightReport struct {
	Schema string `json:"schema"`
	OK     bool   `json:"ok"`
	// Stage and Error describe the first check that failed.
	Stage      string                      `json:"stage,omitempty"`
	Error      string                      `json:"error,omitempty"`
	Migrations []RestartPreflightMigration `json:"migrations,omitempty"`
	// Warnings list checks that could not conclude; they never fail the pre-flight.
	Warnings []string `json:"warnings,omitempty"`
}

// RestartPreflightMigration is the non-mutating migration plan of one daemon-global stream.
type RestartPreflightMigration struct {
	Stream      string `json:"stream"`
	Version     int64  `json:"version"`
	HeadVersion int64  `json:"head_version"`
	Pending     int    `json:"pending"`
}

// RestartPreflightConfig configures one RunRestartPreflight execution.
type RestartPreflightConfig struct {
	HomePaths compozyconfig.HomePaths
	Getenv    func(string) string
	// LoadLaunchConfig loads the configuration the launch commands read before the daemon boots.
	LoadLaunchConfig func() error
}

// RunRestartPreflight reports whether this binary can boot the home: it plans the global database
// migrations through a read-only handle, then loads and validates the configuration as launch and
// boot do. It never applies a migration, never creates the database, and never opens
// per-session databases.
func RunRestartPreflight(ctx context.Context, cfg *RestartPreflightConfig) RestartPreflightReport {
	report := RestartPreflightReport{Schema: RestartPreflightSchema}
	if cfg == nil {
		return report.failed(RestartPreflightStageConfig, errors.New("daemon: pre-flight config is required"))
	}
	if ctx == nil {
		return report.failed(RestartPreflightStageConfig, errors.New("daemon: pre-flight context is required"))
	}
	// The config loaders apply the lossless retired-key migrations every CLI command applies, so the
	// read-only database plan runs first: a refused plan leaves the config files untouched.
	summary, err := store.PlanMigrationsAtPath(ctx, cfg.HomePaths.DatabaseFile, globaldb.MigrationStream())
	switch {
	case err == nil:
		report.Migrations = []RestartPreflightMigration{{
			Stream:      summary.Stream,
			Version:     summary.Version,
			HeadVersion: summary.HeadVersion,
			Pending:     summary.Pending,
		}}
	case restartPreflightMigrationRefused(err):
		return report.failed(RestartPreflightStageMigrations, err)
	default:
		// A busy or unreadable database is not evidence that the replacement cannot boot.
		report.Warnings = append(
			report.Warnings,
			boundRestartPreflightText("migration plan was not conclusive: "+err.Error()),
		)
	}
	if cfg.LoadLaunchConfig != nil {
		if err := cfg.LoadLaunchConfig(); err != nil {
			return report.failed(RestartPreflightStageConfig, err)
		}
	}
	getenv := cfg.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	if _, err := loadConfigFromHome(cfg.HomePaths, getenv); err != nil {
		return report.failed(RestartPreflightStageConfig, err)
	}

	report.OK = true
	return report
}

// restartPreflightMigrationRefused keeps only the refusals boot repeats deterministically.
func restartPreflightMigrationRefused(err error) bool {
	return errors.Is(err, store.ErrSchemaAhead) ||
		errors.Is(err, store.ErrLegacyDatabase) ||
		errors.Is(err, store.ErrMigrationPlanInvalid)
}

func (r RestartPreflightReport) failed(stage string, err error) RestartPreflightReport {
	r.OK = false
	r.Stage = stage
	r.Error = boundRestartPreflightText(err.Error())
	return r
}

// boundRestartPreflightText keeps a report field small enough that the daemon reading the report
// never receives a truncated, unreadable line.
func boundRestartPreflightText(text string) string {
	if len(text) <= restartPreflightTextLimit {
		return text
	}
	cut := restartPreflightTextLimit - len(restartPreflightTextTruncation)
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut] + restartPreflightTextTruncation
}

// FailureReason renders the failed check for the restart journal and transport errors.
func (r RestartPreflightReport) FailureReason() string {
	stage := strings.TrimSpace(r.Stage)
	reason := strings.TrimSpace(r.Error)
	switch {
	case stage == "" && reason == "":
		return "replacement reported a failed pre-flight without a reason"
	case stage == "":
		return reason
	case reason == "":
		return stage
	default:
		return stage + ": " + reason
	}
}

type restartPreflightStatus string

const (
	restartPreflightPassed restartPreflightStatus = "passed"
	// restartPreflightFailed is the only status that refuses a restart.
	restartPreflightFailed restartPreflightStatus = "failed"
	// restartPreflightUnavailable covers binaries without the subcommand and runs that never reported.
	restartPreflightUnavailable restartPreflightStatus = "unavailable"
	restartPreflightTimedOut    restartPreflightStatus = "timed_out"
)

type restartPreflightRequest struct {
	binary  string
	args    []string
	sandbox []string
}

type restartPreflightOutcome struct {
	status restartPreflightStatus
	reason string
}

type restartPreflightFunc func(context.Context, restartPreflightRequest) restartPreflightOutcome

// preflightReplacement runs the replacement binary's pre-flight before anything is signaled.
// Only a positive failure report refuses the restart; every other outcome proceeds as before.
func (r restartRequestRuntime) preflightReplacement(ctx context.Context, operationID string) error {
	if r.preflight == nil {
		return nil
	}
	binary, err := r.executable()
	if err != nil {
		// The helper launch reports an unresolvable executable with its own failure reason.
		return nil
	}
	outcome := r.preflight(ctx, restartPreflightRequest{
		binary:  binary,
		args:    []string{harnessSummaryDefaultAgentName, RestartPreflightCommand},
		sandbox: procutil.WithoutInternalHandoffEnv(os.Environ()),
	})
	switch outcome.status {
	case restartPreflightFailed:
		return fmt.Errorf("%w: %s", ErrRestartPreflightFailed, outcome.reason)
	case restartPreflightPassed:
		slog.InfoContext(ctx, "daemon: replacement pre-flight passed",
			"operation_id", operationID, "binary", binary, "detail", outcome.reason)
	default:
		slog.WarnContext(ctx, "daemon: replacement pre-flight did not conclude; restarting without it",
			"operation_id", operationID, "binary", binary, "status", string(outcome.status), "reason", outcome.reason)
	}
	return nil
}

// classifyRestartPreflight trusts a report over the exit state, so a positive failure refuses the
// restart even when the run also timed out, and a binary that printed nothing never blocks one.
func classifyRestartPreflight(stdout []byte, waitErr error, ctxErr error) restartPreflightOutcome {
	if report, ok := parseRestartPreflightReport(stdout); ok {
		if report.OK {
			return restartPreflightOutcome{
				status: restartPreflightPassed,
				reason: strings.Join(report.Warnings, "; "),
			}
		}
		return restartPreflightOutcome{status: restartPreflightFailed, reason: report.FailureReason()}
	}
	switch {
	case errors.Is(ctxErr, context.DeadlineExceeded):
		return restartPreflightOutcome{
			status: restartPreflightTimedOut,
			reason: "replacement did not report within the pre-flight timeout",
		}
	case ctxErr != nil:
		return restartPreflightOutcome{status: restartPreflightUnavailable, reason: ctxErr.Error()}
	case waitErr != nil:
		return restartPreflightOutcome{
			status: restartPreflightUnavailable,
			reason: "replacement exited without a pre-flight report: " + waitErr.Error(),
		}
	default:
		return restartPreflightOutcome{
			status: restartPreflightUnavailable,
			reason: "replacement does not support pre-flight",
		}
	}
}

// classifyRestartPreflightOutput classifies a captured run. Output past the capture limit that left
// no readable report is never read as a binary without the subcommand: with a failing exit it is
// the failure report of a replacement that cannot boot, so it refuses the restart.
func classifyRestartPreflightOutput(
	output *restartPreflightOutput,
	waitErr error,
	ctxErr error,
) restartPreflightOutcome {
	outcome := classifyRestartPreflight(output.data, waitErr, ctxErr)
	if !output.truncated || outcome.status != restartPreflightUnavailable || ctxErr != nil {
		return outcome
	}
	if waitErr != nil {
		return restartPreflightOutcome{
			status: restartPreflightFailed,
			reason: fmt.Sprintf(
				"replacement failed its pre-flight with a report larger than %d bytes: %v",
				output.limit, waitErr,
			),
		}
	}
	return restartPreflightOutcome{
		status: restartPreflightUnavailable,
		reason: fmt.Sprintf(
			"replacement pre-flight output exceeded %d bytes and held no readable report",
			output.limit,
		),
	}
}

// parseRestartPreflightReport returns the last line that is a report of the schema this daemon reads.
func parseRestartPreflightReport(stdout []byte) (RestartPreflightReport, bool) {
	lines := bytes.Split(stdout, []byte{'\n'})
	for _, raw := range slices.Backward(lines) {
		line := bytes.TrimSpace(raw)
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var report RestartPreflightReport
		if err := json.Unmarshal(line, &report); err != nil {
			continue
		}
		if report.Schema == RestartPreflightSchema {
			return report, true
		}
	}
	return RestartPreflightReport{}, false
}
