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

func newActivityMonitor() *activityMonitor {
	return runshared.NewActivityMonitor()
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
