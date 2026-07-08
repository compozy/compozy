package acpshared

import (
	"io"
	"log/slog"
	"os"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/internal/runshared"
	"github.com/compozy/compozy/internal/core/run/transcript"
)

type config = runshared.Config
type job = runshared.Job
type failInfo = runshared.FailInfo
type jobAttemptResult = runshared.JobAttemptResult
type activityMonitor = runshared.ActivityMonitor
type clock = runshared.Clock
type lineBuffer = runshared.LineBuffer
type reusableAgentExecution = runshared.ReusableAgentExecution
type SessionViewSnapshot = transcript.SessionViewSnapshot
type FailInfo = runshared.FailInfo
type JobAttemptResult = runshared.JobAttemptResult

const (
	exitCodeTimeout       = runshared.ExitCodeTimeout
	exitCodeCanceled      = runshared.ExitCodeCanceled
	activityCheckInterval = runshared.ActivityCheckInterval
)

const (
	attemptStatusSuccess     = runshared.AttemptStatusSuccess
	attemptStatusFailure     = runshared.AttemptStatusFailure
	attemptStatusTimeout     = runshared.AttemptStatusTimeout
	attemptStatusCanceled    = runshared.AttemptStatusCanceled
	attemptStatusSetupFailed = runshared.AttemptStatusSetupFailed
)

// activityClock is the clock powering the activity monitor and the stall
// watchdog. It is a package var so tests can drive idle windows deterministically
// via SetActivityClockForTest, mirroring the newAgentClient override seam.
var activityClock clock = runshared.RealClock{}

// SetActivityClockForTest overrides the clock used by newly created activity
// monitors and stall watchdogs, returning a restore func. Test-only seam.
func SetActivityClockForTest(c clock) func() {
	previous := activityClock
	if c == nil {
		c = runshared.RealClock{}
	}
	activityClock = c
	return func() {
		activityClock = previous
	}
}

func newActivityMonitor() *activityMonitor {
	return runshared.NewActivityMonitorWithClock(activityClock)
}

// terminalCloser is the optional capability the watchdog uses to reap a stalled
// session's terminal commands on fire. *agent clientImpl satisfies it; fakes that
// do not implement it simply skip terminal teardown.
type terminalCloser interface {
	CloseTerminals() error
}

func appendLinesToBuffer(buf *lineBuffer, lines []string) {
	runshared.AppendLinesToBuffer(buf, lines)
}

func createLogWriters(outFile *os.File, errFile *os.File, useUI bool, emitHuman bool) (io.Writer, io.Writer) {
	return runshared.CreateLogWriters(outFile, errFile, useUI, emitHuman)
}

func runtimeLoggerFor(cfg *config, useUI bool) *slog.Logger {
	return runshared.RuntimeLoggerFor(cfg, useUI)
}

func runtimeLogger(enabled bool) *slog.Logger {
	return runshared.RuntimeLogger(enabled)
}

func silentLogger() *slog.Logger {
	return runshared.SilentLogger()
}

type sessionViewModel = transcript.ViewModel

func newSessionViewModel() *sessionViewModel {
	return transcript.NewViewModel()
}

func writeRenderedLines(dst io.Writer, lines []string) error {
	return transcript.WriteRenderedLines(dst, lines)
}

func renderContentBlocks(blocks []model.ContentBlock) ([]string, []string) {
	return transcript.RenderContentBlocks(blocks)
}
