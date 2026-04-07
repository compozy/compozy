package executor

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/internal/acpshared"
	"github.com/compozy/compozy/internal/core/run/internal/runshared"
	"github.com/compozy/compozy/internal/core/run/internal/runtimeevents"
	"github.com/compozy/compozy/internal/core/run/journal"
	uipkg "github.com/compozy/compozy/internal/core/run/ui"
	"github.com/compozy/compozy/pkg/compozy/events"
)

type config = runshared.Config
type job = runshared.Job
type failInfo = runshared.FailInfo
type jobPhase = runshared.JobPhase
type jobAttemptResult = runshared.JobAttemptResult
type shutdownSource = runshared.ShutdownSource
type shutdownState = runshared.ShutdownState
type uiQuitRequest = runshared.UIQuitRequest
type uiSession = runshared.UISession

const (
	exitCodeCanceled = runshared.ExitCodeCanceled

	jobPhaseQueued    = runshared.JobPhaseQueued
	jobPhaseScheduled = runshared.JobPhaseScheduled
	jobPhaseRunning   = runshared.JobPhaseRunning
	jobPhaseRetrying  = runshared.JobPhaseRetrying
	jobPhaseSucceeded = runshared.JobPhaseSucceeded
	jobPhaseFailed    = runshared.JobPhaseFailed
	jobPhaseCanceled  = runshared.JobPhaseCanceled

	shutdownPhaseIdle     = runshared.ShutdownPhaseIdle
	shutdownPhaseDraining = runshared.ShutdownPhaseDraining
	shutdownPhaseForcing  = runshared.ShutdownPhaseForcing

	shutdownSourceUI     = runshared.ShutdownSourceUI
	shutdownSourceSignal = runshared.ShutdownSourceSignal
	shutdownSourceTimer  = runshared.ShutdownSourceTimer

	uiQuitRequestDrain = runshared.UIQuitRequestDrain
	uiQuitRequestForce = runshared.UIQuitRequestForce

	gracefulShutdownTimeout = runshared.GracefulShutdownTimeout
)

func newConfig(src *model.RuntimeConfig, runArtifacts model.RunArtifacts) *config {
	return runshared.NewConfig(src, runArtifacts)
}

func newJobs(src []model.Job) []job {
	return runshared.NewJobs(src)
}

func atLeastOne(value int) int {
	return runshared.AtLeastOne(value)
}

func setupUI(
	ctx context.Context,
	jobs []job,
	cfg *config,
	bus *events.Bus[events.Event],
	enabled bool,
) uiSession {
	return uipkg.Setup(ctx, jobs, cfg, bus, enabled)
}

func notifyJobStart(
	emitHuman bool,
	job *job,
	ide string,
	model string,
	addDirs []string,
	reasoningEffort string,
	accessMode string,
) {
	acpshared.NotifyJobStart(emitHuman, job, ide, model, addDirs, reasoningEffort, accessMode)
}

func newLineBuffer(n int) *runshared.LineBuffer {
	return runshared.NewLineBuffer(n)
}

func runtimeLogger(enabled bool) *slog.Logger {
	return runshared.RuntimeLogger(enabled)
}

func runtimeLoggerFor(cfg *config, useUI bool) *slog.Logger {
	return runshared.RuntimeLoggerFor(cfg, useUI)
}

func recordFailure(mu *sync.Mutex, list *[]failInfo, f failInfo) {
	acpshared.RecordFailure(mu, list, f)
}

func executeJobWithTimeout(
	ctx context.Context,
	cfg *config,
	j *job,
	cwd string,
	useUI bool,
	index int,
	timeoutDuration time.Duration,
	runJournal *journal.Journal,
	aggregateUsage *model.Usage,
	aggregateMu *sync.Mutex,
	trackClient func(agent.Client) func(),
) jobAttemptResult {
	return acpshared.ExecuteJobWithTimeout(
		ctx,
		cfg,
		j,
		cwd,
		useUI,
		index,
		timeoutDuration,
		runJournal,
		aggregateUsage,
		aggregateMu,
		trackClient,
	)
}

func recordFailureWithContext(
	failuresMu *sync.Mutex,
	j *job,
	failures *[]failInfo,
	err error,
	exitCode int,
) failInfo {
	return acpshared.RecordFailureWithContext(failuresMu, j, failures, err, exitCode)
}

func retryableSetupFailure(err error) bool {
	return acpshared.RetryableSetupFailure(err)
}

func newRuntimeEvent(runID string, kind events.EventKind, payload any) (events.Event, error) {
	return runtimeevents.NewRuntimeEvent(runID, kind, payload)
}

func providerStatusCode(err error) int {
	return runtimeevents.ProviderStatusCode(err)
}

func issueIDFromPath(path string) string {
	return runtimeevents.IssueIDFromPath(path)
}
