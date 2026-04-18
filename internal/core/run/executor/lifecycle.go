package executor

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/journal"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

type jobLifecycle struct {
	index          int
	job            *job
	execCtx        *jobExecutionContext
	state          jobPhase
	attempt        int
	startedAt      time.Time
	currentTimeout time.Duration
	lastExitCode   int
	lastFailure    *failInfo
}

func newJobLifecycle(index int, jb *job, execCtx *jobExecutionContext) *jobLifecycle {
	return &jobLifecycle{
		index:   index,
		job:     jb,
		execCtx: execCtx,
		state:   jobPhaseQueued,
	}
}

func (l *jobLifecycle) schedule() {
	l.state = jobPhaseScheduled
}

func (l *jobLifecycle) startAttempt(attempt int, maxAttempts int, timeout time.Duration) {
	l.attempt = attempt
	l.currentTimeout = timeout
	l.state = jobPhaseRunning
	if l.startedAt.IsZero() {
		l.startedAt = time.Now().UTC()
	}
	cfg := l.execConfig()
	if l.attempt == 1 {
		l.execCtx.submitEventOrWarn(
			events.EventKindJobStarted,
			kinds.JobStartedPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{
					Index:       l.index,
					Attempt:     attempt,
					MaxAttempts: maxAttempts,
				},
				IDE:             l.configIDE(),
				Model:           l.configModel(),
				ReasoningEffort: l.configReasoningEffort(),
				AccessMode:      l.configAccessMode(),
			},
		)
		notifyJobStart(
			cfg != nil && cfg.HumanOutputEnabled() && l.execCtx.ui == nil,
			l.job,
			l.configIDE(),
			l.configModel(),
			l.configAddDirs(),
			l.configReasoningEffort(),
			l.configAccessMode(),
		)
	}
}

func (l *jobLifecycle) markRetry(failure failInfo, nextAttempt int, maxAttempts int) {
	l.lastFailure = &failure
	l.lastExitCode = failure.ExitCode
	l.state = jobPhaseRetrying
	l.attempt = nextAttempt
	l.job.ExitCode = failure.ExitCode
	l.job.Failure = failure.Err.Error()
	l.execCtx.submitEventOrWarn(
		events.EventKindJobRetryScheduled,
		kinds.JobRetryScheduledPayload{
			JobAttemptInfo: kinds.JobAttemptInfo{
				Index:       l.index,
				Attempt:     nextAttempt,
				MaxAttempts: maxAttempts,
			},
			Reason: failure.Err.Error(),
		},
	)
}

func (l *jobLifecycle) markGiveUp(failure failInfo) {
	l.lastFailure = &failure
	l.lastExitCode = failure.ExitCode
	l.state = jobPhaseFailed
	l.job.Status = runStatusFailed
	l.job.ExitCode = failure.ExitCode
	l.job.Failure = failure.Err.Error()
	if l.lastFailure != nil {
		recordFailure(&l.execCtx.failuresMu, &l.execCtx.failures, *l.lastFailure)
	}
	atomic.AddInt32(&l.execCtx.failed, 1)
	l.execCtx.submitEventOrWarn(
		events.EventKindJobFailed,
		kinds.JobFailedPayload{
			JobAttemptInfo: kinds.JobAttemptInfo{
				Index:       l.index,
				Attempt:     l.attempt,
				MaxAttempts: l.maxAttempts(),
			},
			CodeFile: l.job.CodeFileLabel(),
			ExitCode: l.lastExitCode,
			OutLog:   l.job.OutLog,
			ErrLog:   l.job.ErrLog,
			Error:    l.job.Failure,
		},
	)
	if l.lastFailure != nil && l.humanOutputEnabled() {
		fmt.Fprintf(
			os.Stderr,
			"\n❌ Job %d (%s) failed with exit code %d: %v\n",
			l.index+1,
			l.job.CodeFileLabel(),
			l.lastExitCode,
			l.lastFailure.Err,
		)
	}
}

func (l *jobLifecycle) markSuccess() {
	if l.startedAt.IsZero() {
		l.startedAt = time.Now().UTC()
	}
	l.lastFailure = nil
	l.lastExitCode = 0
	l.state = jobPhaseSucceeded
	l.job.Status = runStatusSucceeded
	l.job.ExitCode = 0
	l.job.Failure = ""
	l.execCtx.submitEventOrWarn(
		events.EventKindJobCompleted,
		kinds.JobCompletedPayload{
			JobAttemptInfo: kinds.JobAttemptInfo{
				Index:       l.index,
				Attempt:     l.attempt,
				MaxAttempts: l.maxAttempts(),
			},
			ExitCode:   0,
			DurationMs: time.Since(l.startedAt).Milliseconds(),
		},
	)
}

func (l *jobLifecycle) markCanceled(exitCode int) {
	l.lastExitCode = exitCode
	l.state = jobPhaseCanceled
	l.job.Status = runStatusCanceled
	l.job.ExitCode = exitCode
	if exitCode == exitCodeCanceled {
		l.lastFailure = &failInfo{
			CodeFile: l.job.CodeFileLabel(),
			ExitCode: exitCodeCanceled,
			OutLog:   l.job.OutLog,
			ErrLog:   l.job.ErrLog,
			Err:      fmt.Errorf("job canceled by shutdown"),
		}
	} else {
		l.lastFailure = nil
	}
	if l.lastFailure != nil {
		l.job.Failure = l.lastFailure.Err.Error()
	}

	if l.lastFailure != nil {
		recordFailure(&l.execCtx.failuresMu, &l.execCtx.failures, *l.lastFailure)
	}
	atomic.AddInt32(&l.execCtx.failed, 1)
	reason := ""
	if l.lastFailure != nil && l.lastFailure.Err != nil {
		reason = l.lastFailure.Err.Error()
	}
	l.execCtx.submitEventOrWarn(
		events.EventKindJobCancelled,
		kinds.JobCancelledPayload{
			JobAttemptInfo: kinds.JobAttemptInfo{
				Index:       l.index,
				Attempt:     l.attempt,
				MaxAttempts: l.maxAttempts(),
			},
			Reason: reason,
		},
	)
	if l.lastFailure != nil && l.humanOutputEnabled() {
		fmt.Fprintf(
			os.Stderr,
			"\n⚠️ Job %d (%s) canceled: %v\n",
			l.index+1,
			l.job.CodeFileLabel(),
			l.lastFailure.Err,
		)
	}
}

func (l *jobLifecycle) execConfig() *config {
	if l == nil || l.execCtx == nil {
		return nil
	}
	return l.execCtx.cfg
}

func (l *jobLifecycle) maxAttempts() int {
	cfg := l.execConfig()
	if cfg == nil {
		return 1
	}
	return atLeastOne(cfg.MaxRetries + 1)
}

func (l *jobLifecycle) humanOutputEnabled() bool {
	cfg := l.execConfig()
	return cfg != nil && cfg.HumanOutputEnabled()
}

func (l *jobLifecycle) configIDE() string {
	if l != nil && l.job != nil && strings.TrimSpace(l.job.IDE) != "" {
		return l.job.IDE
	}
	cfg := l.execConfig()
	if cfg == nil {
		return ""
	}
	return cfg.IDE
}

func (l *jobLifecycle) configModel() string {
	if l != nil && l.job != nil && strings.TrimSpace(l.job.Model) != "" {
		return l.job.Model
	}
	cfg := l.execConfig()
	if cfg == nil {
		return ""
	}
	return cfg.Model
}

func (l *jobLifecycle) configAddDirs() []string {
	cfg := l.execConfig()
	if cfg == nil {
		return nil
	}
	return cfg.AddDirs
}

func (l *jobLifecycle) configReasoningEffort() string {
	if l != nil && l.job != nil && strings.TrimSpace(l.job.ReasoningEffort) != "" {
		return l.job.ReasoningEffort
	}
	cfg := l.execConfig()
	if cfg == nil {
		return ""
	}
	return cfg.ReasoningEffort
}

func (l *jobLifecycle) configAccessMode() string {
	cfg := l.execConfig()
	if cfg == nil {
		return ""
	}
	return cfg.AccessMode
}

func refreshTaskMetaOnExit(cfg *config) {
	if cfg == nil || cfg.Mode != model.ExecutionModePRDTasks || strings.TrimSpace(cfg.TasksDir) == "" {
		return
	}

	meta, err := tasks.SnapshotTaskMeta(cfg.TasksDir)
	if err != nil {
		runtimeLoggerFor(cfg, cfg != nil && cfg.UIEnabled()).Warn(
			"failed to refresh task workflow summary at command exit",
			"tasks_dir",
			cfg.TasksDir,
			"error",
			err,
		)
		return
	}

	runtimeLoggerFor(cfg, cfg != nil && cfg.UIEnabled()).Info(
		"refreshed task workflow summary at command exit",
		"tasks_dir",
		cfg.TasksDir,
		"completed",
		meta.Completed,
		"pending",
		meta.Pending,
		"total",
		meta.Total,
	)
}

func submitRunEvent(
	ctx context.Context,
	runJournal *journal.Journal,
	runID string,
	kind events.EventKind,
	payload any,
) error {
	if runJournal == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	event, err := newRuntimeEvent(runID, kind, payload)
	if err != nil {
		return err
	}
	return runJournal.Submit(ctx, event)
}

func emitRunTerminalEvent(
	ctx context.Context,
	runJournal *journal.Journal,
	result executionResult,
	jobs []job,
	startedAt time.Time,
) error {
	if runJournal == nil {
		return nil
	}

	var (
		succeeded int
		failed    int
		canceled  int
	)
	for idx := range jobs {
		switch jobs[idx].Status {
		case runStatusSucceeded:
			succeeded++
		case runStatusCanceled:
			canceled++
		case runStatusFailed:
			failed++
		}
	}

	durationMs := time.Since(startedAt).Milliseconds()
	switch result.Status {
	case runStatusSucceeded:
		return submitRunEvent(
			ctx,
			runJournal,
			result.RunID,
			events.EventKindRunCompleted,
			kinds.RunCompletedPayload{
				ArtifactsDir:   result.ArtifactsDir,
				JobsTotal:      len(jobs),
				JobsSucceeded:  succeeded,
				JobsFailed:     failed,
				JobsCancelled:  canceled,
				DurationMs:     durationMs,
				ResultPath:     result.ResultPath,
				SummaryMessage: "completed",
			},
		)
	case runStatusCanceled:
		reason := strings.TrimSpace(result.Error)
		if reason == "" {
			reason = strings.TrimSpace(result.TeardownError)
		}
		return submitRunEvent(
			ctx,
			runJournal,
			result.RunID,
			events.EventKindRunCancelled,
			kinds.RunCancelledPayload{
				Reason:     reason,
				DurationMs: durationMs,
			},
		)
	default:
		errText := strings.TrimSpace(result.Error)
		if errText == "" {
			errText = strings.TrimSpace(result.TeardownError)
		}
		return submitRunEvent(
			ctx,
			runJournal,
			result.RunID,
			events.EventKindRunFailed,
			kinds.RunFailedPayload{
				ArtifactsDir: result.ArtifactsDir,
				DurationMs:   durationMs,
				Error:        errText,
				ResultPath:   result.ResultPath,
			},
		)
	}
}

func closeRunJournal(runJournal *journal.Journal) error {
	if runJournal == nil {
		return nil
	}
	closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := runJournal.Close(closeCtx); err != nil {
		return fmt.Errorf("close run journal: %w", err)
	}
	return nil
}
