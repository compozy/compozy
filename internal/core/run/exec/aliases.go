package exec

import (
	"context"
	"log/slog"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/internal/acpshared"
	"github.com/compozy/compozy/internal/core/run/internal/runshared"
	"github.com/compozy/compozy/internal/core/run/internal/runtimeevents"
	"github.com/compozy/compozy/internal/core/run/transcript"
	uipkg "github.com/compozy/compozy/internal/core/run/ui"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
)

type config = runshared.Config
type job = runshared.Job
type uiSession = runshared.UISession
type activityMonitor = runshared.ActivityMonitor
type lineBuffer = runshared.LineBuffer
type SessionViewSnapshot = transcript.SessionViewSnapshot
type sessionExecution = acpshared.SessionExecution
type sessionSetupRequest = acpshared.SessionSetupRequest

const (
	runStatusSucceeded = runshared.RunStatusSucceeded
	runStatusFailed    = runshared.RunStatusFailed
	runStatusCanceled  = runshared.RunStatusCanceled
)

func newConfig(src *model.RuntimeConfig, runArtifacts model.RunArtifacts) *config {
	return runshared.NewConfig(src, runArtifacts)
}

func atLeastOne(value int) int {
	return runshared.AtLeastOne(value)
}

func setupUI(
	ctx context.Context,
	jobs []job,
	cfg *config,
	enabled bool,
) uiSession {
	return uipkg.Setup(ctx, jobs, cfg, nil, enabled)
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

func newActivityMonitor() *activityMonitor {
	return runshared.NewActivityMonitor()
}

func newLineBuffer(n int) *lineBuffer {
	return runshared.NewLineBuffer(n)
}

func startACPActivityWatchdog(
	ctx context.Context,
	monitor *activityMonitor,
	timeout time.Duration,
	cancel context.CancelCauseFunc,
) func() {
	return acpshared.StartACPActivityWatchdog(ctx, monitor, timeout, cancel)
}

func setupSessionExecution(req sessionSetupRequest) (*sessionExecution, error) {
	return acpshared.SetupSessionExecution(req)
}

func isActivityTimeout(err error) bool {
	return acpshared.IsActivityTimeout(err)
}

func newActivityTimeoutError(timeout time.Duration) error {
	return acpshared.NewActivityTimeoutError(timeout)
}

func sessionErrorCode(err error) int {
	return acpshared.SessionErrorCode(err)
}

func runtimeLoggerFor(cfg *config, useUI bool) *slog.Logger {
	return runshared.RuntimeLoggerFor(cfg, useUI)
}

const transcriptEntryAssistantMessage = transcript.EntryKindAssistantMessage

func renderContentBlocks(blocks []model.ContentBlock) ([]string, []string) {
	return transcript.RenderContentBlocks(blocks)
}

func providerStatusCode(err error) int {
	return runtimeevents.ProviderStatusCode(err)
}

func issueIDFromPath(path string) string {
	return runtimeevents.IssueIDFromPath(path)
}

func newRuntimeEvent(runID string, kind eventspkg.EventKind, payload any) (eventspkg.Event, error) {
	return runtimeevents.NewRuntimeEvent(runID, kind, payload)
}
