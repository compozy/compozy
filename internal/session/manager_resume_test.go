package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/procutil"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestResumeLoadsMetaAndPassesStoredACPSessionID(t *testing.T) {
	t.Parallel()

	t.Run("Should bind a runtime before resuming its stored ACP session", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		events, err := h.manager.Prompt(testutil.Context(t), session.ID, "bind the runtime")
		if err != nil {
			t.Fatalf("Prompt(bind runtime) error = %v", err)
		}
		collectEvents(t, events)
		originalACP := session.Info().ACPSessionID
		if originalACP == "" {
			t.Fatal("bound session ACP session id is empty")
		}

		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Resume() error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
				t.Errorf("Stop(resumed) error = %v", err)
			}
		})

		if got := h.driver.startCalls[1].ResumeSessionID; got != originalACP {
			t.Fatalf("resume start ResumeSessionID = %q, want %q", got, originalACP)
		}
		if got := resumed.Info().ACPSessionID; got != originalACP {
			t.Fatalf("resumed ACPSessionID = %q, want %q", got, originalACP)
		}
		if got := resumed.Info().State; got != StateActive {
			t.Fatalf("resumed state = %q, want %q", got, StateActive)
		}
		if got := resumed.Info().StopReason; got != "" {
			t.Fatalf("resumed stop reason = %q, want empty", got)
		}
		if got := resumed.Info().StopDetail; got != "" {
			t.Fatalf("resumed stop detail = %q, want empty", got)
		}
	})
}

func TestResumeRejectsTerminalProcessFailureBeforeStartingACP(t *testing.T) {
	t.Parallel()

	t.Run("Should keep a dead process-exited session read-only", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		const sessionID = "dead-session-resume"
		writeStoppedSessionArtifacts(t, h, sessionID, true)
		metaPath := store.SessionMetaFile(filepath.Join(h.homePaths.SessionsDir, sessionID))
		meta := readMeta(t, metaPath)
		meta.Failure = &store.SessionFailure{
			Kind:    store.FailureProcess,
			Summary: "Codex exited before the response completed",
		}
		if err := store.WriteSessionMeta(metaPath, meta); err != nil {
			t.Fatalf("WriteSessionMeta(%q) error = %v", metaPath, err)
		}

		if _, err := h.manager.Resume(testutil.Context(t), sessionID); !errors.Is(err, store.ErrSessionNotAttachable) {
			t.Fatalf("Resume(dead session) error = %v, want ErrSessionNotAttachable", err)
		}
		if got := len(h.driver.startCalls); got != 0 {
			t.Fatalf("Resume(dead session) started ACP %d times, want 0", got)
		}
	})
}

func TestResumeRepairsIncompleteStartAndStartsFreshACPClient(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	originalACP := session.Info().ACPSessionID

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	meta := readMeta(t, session.MetaPath())
	meta.State = string(StateStarting)
	meta.StopReason = nil
	meta.StopDetail = ""
	meta.ACPSessionID = stringPointer(originalACP)
	if err := store.WriteSessionMeta(session.MetaPath(), meta); err != nil {
		t.Fatalf("WriteSessionMeta() error = %v", err)
	}

	prepareExitedResumeRecovery(t, h, session)

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume(incomplete start) error = %v", err)
	}
	t.Cleanup(func() {
		reportSessionStop(t, h, resumed.ID)
	})

	if got := h.driver.startCalls[1].ResumeSessionID; got != "" {
		t.Fatalf("resume start ResumeSessionID = %q, want empty for repaired start", got)
	}
	if got := resumed.Info().ACPSessionID; got == "" || got == originalACP {
		t.Fatalf("resumed ACPSessionID = %q, want fresh ACP session id distinct from %q", got, originalACP)
	}
	if got := resumed.Info().State; got != StateActive {
		t.Fatalf("resumed state = %q, want %q", got, StateActive)
	}
	if got := resumed.Info().StopReason; got != "" {
		t.Fatalf("resumed stop reason = %q, want empty", got)
	}
	if got := resumed.Info().StopDetail; got != "" {
		t.Fatalf("resumed stop detail = %q, want empty", got)
	}
}

func TestResumePreservesCrashStopClassificationFromRepairedMetadata(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	meta := readMeta(t, session.MetaPath())
	meta.State = string(StateActive)
	meta.StopReason = nil
	meta.StopDetail = ""
	if err := store.WriteSessionMeta(session.MetaPath(), meta); err != nil {
		t.Fatalf("WriteSessionMeta() error = %v", err)
	}

	prepareExitedResumeRecovery(t, h, session)

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	t.Cleanup(func() {
		reportSessionStop(t, h, resumed.ID)
	})

	if got := resumed.Info().State; got != StateActive {
		t.Fatalf("resumed state = %q, want %q", got, StateActive)
	}
	if got := resumed.Info().StopReason; got != store.StopAgentCrashed {
		t.Fatalf("resumed stop reason = %q, want %q", got, store.StopAgentCrashed)
	}
	if got := resumed.Info().StopDetail; got != resumeStopDetailAgentCrashed {
		t.Fatalf("resumed stop detail = %q, want %q", got, resumeStopDetailAgentCrashed)
	}

	repaired := readMeta(t, resumed.MetaPath())
	if repaired.StopReason == nil {
		t.Fatal("meta.StopReason = nil, want non-nil")
	}
	if *repaired.StopReason != store.StopAgentCrashed {
		t.Fatalf("meta.StopReason = %q, want %q", *repaired.StopReason, store.StopAgentCrashed)
	}
	if got := repaired.StopDetail; got != resumeStopDetailAgentCrashed {
		t.Fatalf("meta.StopDetail = %q, want %q", got, resumeStopDetailAgentCrashed)
	}
}

func TestResumeFallsBackToFreshStartWhenStoredACPSessionIsMissing(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	originalACP := session.Info().ACPSessionID

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
		if opts.ResumeSessionID != "" {
			return nil, fmt.Errorf(
				"%w: load session %q for %q: %w",
				acp.ErrLoadSessionFailed,
				opts.ResumeSessionID,
				opts.AgentName,
				&acpsdk.RequestError{
					Code:    -32002,
					Message: "Resource not found: " + opts.ResumeSessionID,
				},
			)
		}
		return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("acp-new-%d", sequence)), nil
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume(missing ACP session) error = %v", err)
	}
	t.Cleanup(func() {
		reportSessionStop(t, h, resumed.ID)
	})

	if got := h.driver.startCalls[1].ResumeSessionID; got != originalACP {
		t.Fatalf("first resume start ResumeSessionID = %q, want %q", got, originalACP)
	}
	if got := h.driver.startCalls[2].ResumeSessionID; got != "" {
		t.Fatalf("fallback resume start ResumeSessionID = %q, want empty", got)
	}
	if got := resumed.Info().ACPSessionID; got == "" || got == originalACP {
		t.Fatalf("resumed ACPSessionID = %q, want fresh ACP session id distinct from %q", got, originalACP)
	}
	if got := resumed.Info().State; got != StateActive {
		t.Fatalf("resumed state = %q, want %q", got, StateActive)
	}
	if meta := readMeta(t, session.MetaPath()); meta.State != string(StateActive) {
		t.Fatalf("meta state after fallback resume = %q, want %q", meta.State, StateActive)
	}
}

func TestResumeMissingACPStateFallbackPreservesRecoveredCrashClassification(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	originalACP := session.Info().ACPSessionID

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	meta := readMeta(t, session.MetaPath())
	meta.State = string(StateActive)
	meta.StopReason = nil
	meta.StopDetail = ""
	if err := store.WriteSessionMeta(session.MetaPath(), meta); err != nil {
		t.Fatalf("WriteSessionMeta() error = %v", err)
	}

	prepareExitedResumeRecovery(t, h, session)

	h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
		if opts.ResumeSessionID != "" {
			return nil, fmt.Errorf(
				"%w: load session %q for %q: %w",
				acp.ErrLoadSessionFailed,
				opts.ResumeSessionID,
				opts.AgentName,
				&acpsdk.RequestError{
					Code:    -32002,
					Message: "Resource not found: " + opts.ResumeSessionID,
				},
			)
		}
		return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("acp-new-%d", sequence)), nil
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume(missing ACP state after crash repair) error = %v", err)
	}
	t.Cleanup(func() {
		reportSessionStop(t, h, resumed.ID)
	})

	if got := h.driver.startCalls[1].ResumeSessionID; got != originalACP {
		t.Fatalf("first resume start ResumeSessionID = %q, want %q", got, originalACP)
	}
	if got := h.driver.startCalls[2].ResumeSessionID; got != "" {
		t.Fatalf("fallback resume start ResumeSessionID = %q, want empty", got)
	}
	if got := resumed.Info().StopReason; got != store.StopAgentCrashed {
		t.Fatalf("resumed StopReason = %q, want %q", got, store.StopAgentCrashed)
	}
	if got := resumed.Info().StopDetail; got != resumeStopDetailAgentCrashed {
		t.Fatalf("resumed StopDetail = %q, want %q", got, resumeStopDetailAgentCrashed)
	}
}

func TestResumeMissingACPStateFallbackLogsAtInfoLevel(t *testing.T) {
	t.Parallel()

	logs := newCaptureLogHandler()
	h := newHarness(t, WithLogger(slog.New(logs)))
	session := createSession(t, h)
	originalACP := session.Info().ACPSessionID

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
		if opts.ResumeSessionID != "" {
			return nil, fmt.Errorf(
				"%w: load session %q for %q: %w",
				acp.ErrLoadSessionFailed,
				opts.ResumeSessionID,
				opts.AgentName,
				&acpsdk.RequestError{
					Code:    -32002,
					Message: "Resource not found: " + opts.ResumeSessionID,
				},
			)
		}
		return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("acp-new-%d", sequence)), nil
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	t.Cleanup(func() {
		reportSessionStop(t, h, resumed.ID)
	})

	if got := h.driver.startCalls[1].ResumeSessionID; got != originalACP {
		t.Fatalf("first resume start ResumeSessionID = %q, want %q", got, originalACP)
	}

	record, ok := logs.FindByMessage("session.resume.context_replay_fallback")
	if !ok {
		t.Fatalf("missing context_replay_fallback log: %#v", logs.Records())
	}
	if got, want := record.Level, slog.LevelInfo; got != want {
		t.Fatalf("fallback log level = %v, want %v", got, want)
	}
	assertCapturedLogAttr(t, record, "session_id", session.ID)
	assertCapturedLogAttr(t, record, "agent_name", "coder")
	assertCapturedLogAttr(t, record, "provider", "claude")
	assertCapturedLogAttr(t, record, "phase", "resume")
	assertCapturedLogAttr(t, record, "fallback_reason", "load_session_resource_missing")
}

func TestResumeFailureRestoresStoppedMetadata(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	originalACP := session.Info().ACPSessionID

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	metaBefore := readMeta(t, session.MetaPath())
	h.driver.startHook = func(_ acp.StartOpts, _ int) (*fakeProcess, error) {
		return nil, errors.New("start failed")
	}

	if _, err := h.manager.Resume(testutil.Context(t), session.ID); err == nil {
		t.Fatal("Resume(generic failure) error = nil, want non-nil")
	}

	metaAfter := readMeta(t, session.MetaPath())
	if got := metaAfter.State; got != string(StateStopped) {
		t.Fatalf("meta state after failed resume = %q, want %q", got, StateStopped)
	}
	if got := derefString(metaAfter.ACPSessionID); got != originalACP {
		t.Fatalf("meta ACPSessionID after failed resume = %q, want %q", got, originalACP)
	}
	assertOptionalStopReasonEqual(t, metaAfter.StopReason, metaBefore.StopReason)
	if got := metaAfter.StopDetail; got != metaBefore.StopDetail {
		t.Fatalf("meta stop detail after failed resume = %q, want %q", got, metaBefore.StopDetail)
	}
}

func TestResumeRejectsMissingWorktreeWithoutMutatingMetadata(t *testing.T) {
	t.Parallel()
	t.Run("Should reject a missing worktree without mutating metadata", func(t *testing.T) {
		t.Parallel()

		worktreeRoot := filepath.Join(t.TempDir(), "worktree")
		if err := os.MkdirAll(worktreeRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll(worktree root) error = %v", err)
		}
		resolver := &fakeSessionWorktreeResolver{id: "wt-resume", root: worktreeRoot}
		h := newHarness(t, WithWorktreeResolver(resolver))
		created, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Worktree: "wt-resume",
		})
		if err != nil {
			t.Fatalf("Create(bound) error = %v", err)
		}
		if err := h.manager.Stop(testutil.Context(t), created.ID); err != nil {
			t.Fatalf("Stop(bound) error = %v", err)
		}

		metaBefore := readMeta(t, created.MetaPath())
		encodedBefore, err := json.Marshal(metaBefore)
		if err != nil {
			t.Fatalf("json.Marshal(meta before) error = %v", err)
		}
		missingErr := errors.New("worktree_missing")
		resolver.setError(missingErr)

		if _, err := h.manager.Resume(testutil.Context(t), created.ID); !errors.Is(err, missingErr) {
			t.Fatalf("Resume(missing worktree) error = %v, want %v", err, missingErr)
		}
		if got := len(h.driver.startCalls); got != 1 {
			t.Fatalf("driver starts after missing resume = %d, want original create only", got)
		}
		metaAfter := readMeta(t, created.MetaPath())
		encodedAfter, err := json.Marshal(metaAfter)
		if err != nil {
			t.Fatalf("json.Marshal(meta after) error = %v", err)
		}
		if !bytes.Equal(encodedAfter, encodedBefore) {
			t.Fatalf("metadata changed after missing resume:\nbefore: %s\nafter:  %s", encodedBefore, encodedAfter)
		}
	})
}

func TestResumeReplayFallback(t *testing.T) {
	t.Parallel()

	t.Run("Should inject the checkpoint summary before the persisted transcript", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		checkpoint := "<compozy_checkpoint_summary>\n## Goal\nPreserve the cobalt decision.\n</compozy_checkpoint_summary>"
		h.manager = newManagerWithHarness(
			t,
			h,
			WithPromptAssembler(&resumeContextPromptAssembler{checkpoint: checkpoint}),
		)
		session := createSession(t, h)
		recordResumeReplayFixture(t, h.manager, session, "local-only-context")
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
			if opts.ResumeSessionID != "" {
				return nil, fmt.Errorf("%w: unsupported", acp.ErrAgentDoesNotSupportSession)
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("acp-new-%d", sequence)), nil
		}

		resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Resume() error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
				t.Fatalf("Stop(resumed) error = %v", err)
			}
		})
		events, err := h.manager.Prompt(testutil.Context(t), resumed.ID, "continue")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		collectEvents(t, events)
		got := h.driver.promptCalls[0].Message
		checkpointIndex := strings.Index(got, "<compozy_checkpoint_summary>")
		replayIndex := strings.Index(got, resumeReplayOpenTag)
		if checkpointIndex < 0 || replayIndex < 0 || checkpointIndex >= replayIndex {
			t.Fatalf("resume prompt checkpoint/replay order invalid:\n%s", got)
		}
	})

	t.Run("Should stage a pruned replay when session load is unsupported", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		recordResumeReplayFixture(t, h.manager, session, "local-only-context")
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		eventsBeforeResume := readStoredEvents(t, session)
		h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
			if opts.ResumeSessionID != "" {
				return nil, fmt.Errorf(
					"%w: agent %q does not support session/load",
					acp.ErrAgentDoesNotSupportSession,
					opts.AgentName,
				)
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("acp-new-%d", sequence)), nil
		}

		resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Resume(load unsupported) error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
				t.Fatalf("Stop(resumed) error = %v", err)
			}
		})

		firstPrompt, err := h.manager.Prompt(testutil.Context(t), resumed.ID, "continue after restart")
		if err != nil {
			t.Fatalf("Prompt(first resumed turn) error = %v", err)
		}
		collectEvents(t, firstPrompt)
		assertResumeReplayEqualsPrunedEvents(t, h.driver.promptCalls[0].Message, eventsBeforeResume)

		secondPrompt, err := h.manager.Prompt(testutil.Context(t), resumed.ID, "continue again")
		if err != nil {
			t.Fatalf("Prompt(second resumed turn) error = %v", err)
		}
		collectEvents(t, secondPrompt)
		if strings.Contains(h.driver.promptCalls[1].Message, "<compozy_context_replay>") {
			t.Fatalf("second resumed prompt contains replay block: %q", h.driver.promptCalls[1].Message)
		}
		assertContextRebuiltMarkerCount(t, readStoredEvents(t, resumed), 1)
	})

	t.Run("Should stage a pruned replay when the stored ACP session is stale", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		recordResumeReplayFixture(t, h.manager, session, "stale-session-context")
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		eventsBeforeResume := readStoredEvents(t, session)
		h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
			if opts.ResumeSessionID != "" {
				return nil, fmt.Errorf(
					"%w: load session %q for %q: %w",
					acp.ErrLoadSessionFailed,
					opts.ResumeSessionID,
					opts.AgentName,
					&acpsdk.RequestError{Code: -32002, Message: "Resource not found"},
				)
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("acp-new-%d", sequence)), nil
		}

		resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Resume(stale ACP session) error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
				t.Fatalf("Stop(resumed) error = %v", err)
			}
		})

		prompt, err := h.manager.Prompt(testutil.Context(t), resumed.ID, "continue after stale load")
		if err != nil {
			t.Fatalf("Prompt(resumed) error = %v", err)
		}
		collectEvents(t, prompt)
		assertResumeReplayEqualsPrunedEvents(t, h.driver.promptCalls[0].Message, eventsBeforeResume)
		assertContextRebuiltMarkerCount(t, readStoredEvents(t, resumed), 1)
	})

	t.Run("Should not stage replay when session load succeeds", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		recordResumeReplayFixture(t, h.manager, session, "loaded-context")
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Resume(load succeeds) error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
				t.Fatalf("Stop(resumed) error = %v", err)
			}
		})

		prompt, err := h.manager.Prompt(testutil.Context(t), resumed.ID, "continue loaded session")
		if err != nil {
			t.Fatalf("Prompt(resumed) error = %v", err)
		}
		collectEvents(t, prompt)
		if strings.Contains(h.driver.promptCalls[0].Message, "<compozy_context_replay>") {
			t.Fatalf("successful load prompt contains replay block: %q", h.driver.promptCalls[0].Message)
		}
		assertContextRebuiltMarkerCount(t, readStoredEvents(t, resumed), 0)
	})

	t.Run("Should retain replay until prompt delivery is fully prepared", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		recordResumeReplayFixture(t, h.manager, session, "retry-context")
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		eventsBeforeResume := readStoredEvents(t, session)
		h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
			if opts.ResumeSessionID != "" {
				return nil, fmt.Errorf("%w: unsupported", acp.ErrAgentDoesNotSupportSession)
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("acp-new-%d", sequence)), nil
		}

		resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Resume(load unsupported) error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
				t.Fatalf("Stop(resumed) error = %v", err)
			}
		})

		h.driver.promptHook = func(_ *fakeProcess, _ acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			return nil, errors.New("prompt dispatch rejected")
		}
		if _, err := h.manager.Prompt(testutil.Context(t), resumed.ID, "first attempt"); err == nil ||
			!strings.Contains(err.Error(), "prompt dispatch rejected") {
			t.Fatalf("Prompt(rejected) error = %v, want dispatch rejection", err)
		}
		assertResumeReplayEqualsPrunedEvents(t, h.driver.promptCalls[0].Message, eventsBeforeResume)
		if replay := h.manager.pendingResumeReplay(resumed.ID); replay == "" {
			t.Fatal("pending resume replay = empty after rejected dispatch")
		}

		h.driver.promptHook = nil
		prepareErr := errors.New("delivery preparation failed")
		if _, err := h.manager.PromptWithOpts(testutil.Context(t), resumed.ID, PromptOpts{
			Message: "delivery attempt",
			PrepareDelivery: func(context.Context, PromptDelivery) error {
				return prepareErr
			},
		}); !errors.Is(err, prepareErr) {
			t.Fatalf("PromptWithOpts(delivery failure) error = %v, want %v", err, prepareErr)
		}
		assertResumeReplayEqualsPrunedEvents(t, h.driver.promptCalls[1].Message, eventsBeforeResume)
		if replay := h.manager.pendingResumeReplay(resumed.ID); replay == "" {
			t.Fatal("pending resume replay = empty after delivery preparation failure")
		}

		retry, err := h.manager.Prompt(testutil.Context(t), resumed.ID, "retry accepted")
		if err != nil {
			t.Fatalf("Prompt(retry) error = %v", err)
		}
		collectEvents(t, retry)
		assertResumeReplayEqualsPrunedEvents(t, h.driver.promptCalls[2].Message, eventsBeforeResume)
		if replay := h.manager.pendingResumeReplay(resumed.ID); replay != "" {
			t.Fatalf("pending resume replay = %q, want consumed after accepted dispatch", replay)
		}
	})

	t.Run("Should isolate replay to the resumed session", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		localSession := createSession(t, h)
		recordResumeReplayFixture(t, h.manager, localSession, "local-session-secret")
		if err := h.manager.Stop(testutil.Context(t), localSession.ID); err != nil {
			t.Fatalf("Stop(local) error = %v", err)
		}

		foreignRoot := filepath.Join(h.homePaths.HomeDir, "foreign-workspace")
		if err := os.MkdirAll(foreignRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll(foreign workspace) error = %v", err)
		}
		foreignWorkspace, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
		if err != nil {
			t.Fatalf("Resolve(primary workspace) error = %v", err)
		}
		foreignWorkspace.ID = "ws-foreign"
		foreignWorkspace.WorkspaceID = "ws-foreign"
		foreignWorkspace.RootDir = foreignRoot
		foreignWorkspace.Name = "foreign-workspace"
		h.resolver.upsert(&foreignWorkspace)

		foreignSession, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: "ws-foreign",
		})
		if err != nil {
			t.Fatalf("Create(foreign session) error = %v", err)
		}
		recordResumeReplayFixture(t, h.manager, foreignSession, "foreign-session-secret")
		if err := h.manager.Stop(testutil.Context(t), foreignSession.ID); err != nil {
			t.Fatalf("Stop(foreign) error = %v", err)
		}

		h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
			if opts.ResumeSessionID != "" {
				return nil, fmt.Errorf("%w: unsupported", acp.ErrAgentDoesNotSupportSession)
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("acp-new-%d", sequence)), nil
		}

		resumed, err := h.manager.Resume(testutil.Context(t), localSession.ID)
		if err != nil {
			t.Fatalf("Resume(local) error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
				t.Fatalf("Stop(resumed) error = %v", err)
			}
		})

		prompt, err := h.manager.Prompt(testutil.Context(t), resumed.ID, "continue isolated session")
		if err != nil {
			t.Fatalf("Prompt(resumed) error = %v", err)
		}
		collectEvents(t, prompt)
		replay := resumeReplayMessagesFromPrompt(t, h.driver.promptCalls[0].Message)
		encoded, err := json.Marshal(replay)
		if err != nil {
			t.Fatalf("json.Marshal(replay) error = %v", err)
		}
		if !strings.Contains(string(encoded), "local-session-secret") {
			t.Fatalf("replay = %s, want local session context", encoded)
		}
		if strings.Contains(string(encoded), "foreign-session-secret") {
			t.Fatalf("replay = %s, contains foreign session context", encoded)
		}
	})

	t.Run("Should release fallback resources when marker persistence fails", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		stopCallsBeforeResume := h.driver.stopCalls

		h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
			if opts.ResumeSessionID != "" {
				return nil, fmt.Errorf("%w: unsupported", acp.ErrAgentDoesNotSupportSession)
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("acp-new-%d", sequence)), nil
		}
		openCalls := 0
		var fallbackRecorder *markerFailingRecorder
		h.manager = newManagerWithHarness(t, h, WithStore(func(
			ctx context.Context,
			owner store.SessionDBOwner,
			path string,
		) (EventRecorder, error) {
			openCalls++
			recorder, err := sessiondb.OpenSessionDB(ctx, owner, path)
			if err != nil {
				return nil, err
			}
			if openCalls == 2 {
				fallbackRecorder = &markerFailingRecorder{
					EventRecorder: recorder,
					failErr:       errors.New("marker write failed"),
				}
				return fallbackRecorder, nil
			}
			return recorder, nil
		}))

		if _, err := h.manager.Resume(testutil.Context(t), session.ID); err == nil ||
			!strings.Contains(err.Error(), "marker write failed") {
			t.Fatalf("Resume(marker failure) error = %v, want marker write failure", err)
		}
		if fallbackRecorder == nil {
			t.Fatal("fallback recorder = nil, want opened fallback recorder")
		}
		if got, want := fallbackRecorder.closeCalls, 1; got != want {
			t.Fatalf("fallback recorder close calls = %d, want %d", got, want)
		}
		if got, want := h.driver.stopCalls, stopCallsBeforeResume+1; got != want {
			t.Fatalf("driver stop calls = %d, want %d", got, want)
		}
		if _, ok := h.manager.Get(session.ID); ok {
			t.Fatalf("Get(%q) found failed fallback session", session.ID)
		}
		if replay := h.manager.pendingResumeReplay(session.ID); replay != "" {
			t.Fatalf("pending resume replay = %q, want empty after failed start", replay)
		}
		if meta := readMeta(t, session.MetaPath()); meta.State != string(StateStopped) {
			t.Fatalf("meta state after marker failure = %q, want %q", meta.State, StateStopped)
		}
	})
}

func TestResumeFailsWhenWorkspaceCannotBeResolved(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	h.resolver.resolveErr = workspacepkg.ErrWorkspaceNotFound
	if _, err := h.manager.Resume(testutil.Context(t), session.ID); err == nil {
		t.Fatal("Resume(missing workspace) error = nil, want non-nil")
	} else if !errors.Is(err, workspacepkg.ErrWorkspaceNotFound) {
		t.Fatalf("Resume(missing workspace) error = %v, want ErrWorkspaceNotFound", err)
	}
}

func TestResumePassesMergedSkillMCPServers(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	skillRegistry := newFakeSkillRegistry()
	skillRegistry.setSkills(h.workspaceID, []*skillspkg.Skill{
		{
			Enabled: true,
			Source:  skillspkg.SourceUser,
			Meta:    skillspkg.SkillMeta{Name: "resume-skill"},
			MCPServers: []skillspkg.MCPServerDecl{
				{Name: "resume-extra", Command: "resume-extra-command"},
			},
		},
	})
	h.manager = newManagerWithHarness(
		t,
		h,
		WithSkillRegistry(skillRegistry),
		WithMCPResolver(skillspkg.NewMCPResolver(nil)),
	)

	session := createSession(t, h)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	t.Cleanup(func() {
		reportSessionStop(t, h, resumed.ID)
	})

	got := h.driver.startCalls[1].MCPServers
	if len(got) != 1 {
		t.Fatalf("resume start MCPServers = %#v, want 1 entry", got)
	}
	if got[0].Name != "resume-extra" || got[0].Command != "resume-extra-command" {
		t.Fatalf("resume MCP server = %#v", got[0])
	}
	if got := skillRegistry.callCount(); got != 2 {
		t.Fatalf("skill registry call count after resume = %d, want 2", got)
	}
}

// prepareExitedResumeRecovery supplies real identity proof and the boot boundary
// for tests whose invariant is resuming a definitively ended runtime.
func prepareExitedResumeRecovery(t *testing.T, h *harness, session *Session) {
	t.Helper()
	started, err := procutil.StartedAt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	previousIdentity := started.Add(-time.Hour)
	meta := readMeta(t, session.MetaPath())
	meta.Liveness = &store.SessionLivenessMeta{SubprocessPID: os.Getpid(), SubprocessStartedAt: &previousIdentity}
	if err := store.WriteSessionMeta(session.MetaPath(), meta); err != nil {
		t.Fatal(err)
	}
	h.manager = newManagerWithHarness(t, h)
	cleanupTestManager(t, h.manager)
	if err := h.manager.RecoverPendingStops(testutil.Context(t)); err != nil {
		t.Fatal(err)
	}
}
