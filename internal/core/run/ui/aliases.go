package ui

import (
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/internal/runshared"
	"github.com/compozy/compozy/internal/core/run/transcript"
)

type config = runshared.Config
type job = runshared.Job
type lineBuffer = runshared.LineBuffer
type failInfo = runshared.FailInfo

type shutdownPhase = runshared.ShutdownPhase
type shutdownSource = runshared.ShutdownSource
type shutdownState = runshared.ShutdownState

const (
	shutdownPhaseIdle     = runshared.ShutdownPhaseIdle
	shutdownPhaseDraining = runshared.ShutdownPhaseDraining
	shutdownPhaseForcing  = runshared.ShutdownPhaseForcing

	shutdownSourceUI     = runshared.ShutdownSourceUI
	shutdownSourceSignal = runshared.ShutdownSourceSignal
	shutdownSourceTimer  = runshared.ShutdownSourceTimer
)

type uiQuitRequest = runshared.UIQuitRequest
type QuitRequest = runshared.UIQuitRequest

const (
	uiQuitRequestDrain = runshared.UIQuitRequestDrain
	uiQuitRequestForce = runshared.UIQuitRequestForce

	QuitRequestDrain = runshared.UIQuitRequestDrain
	QuitRequestForce = runshared.UIQuitRequestForce
)

type uiSession = runshared.UISession
type Session = runshared.UISession

const exitCodeCanceled = runshared.ExitCodeCanceled
const gracefulShutdownTimeout = runshared.GracefulShutdownTimeout

type SessionViewSnapshot = transcript.SessionViewSnapshot
type SessionPlanState = transcript.SessionPlanState
type SessionMetaState = transcript.SessionMetaState
type TranscriptEntry = transcript.Entry
type transcriptEntryKind = transcript.EntryKind

const (
	transcriptEntryAssistantMessage  = transcript.EntryKindAssistantMessage
	transcriptEntryAssistantThinking = transcript.EntryKindAssistantThinking
	transcriptEntryToolCall          = transcript.EntryKindToolCall
	transcriptEntryStderrEvent       = transcript.EntryKindStderrEvent
	transcriptEntryRuntimeNotice     = transcript.EntryKindRuntimeNotice
)

type sessionViewModel = transcript.ViewModel

func newSessionViewModel() *sessionViewModel {
	return transcript.NewViewModel()
}

func splitRenderedText(text string) []string {
	return transcript.SplitRenderedText(text)
}

func renderContentBlocks(blocks []model.ContentBlock) ([]string, []string) {
	return transcript.RenderContentBlocks(blocks)
}

func hasUsage(usage intUsage) bool {
	return usage.Total() > 0
}

type intUsage interface {
	Total() int
}
