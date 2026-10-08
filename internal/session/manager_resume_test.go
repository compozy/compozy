package session

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	atlasmigrate "ariga.io/atlas/sql/migrate"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/procutil"
	skillspkg "github.com/compozy/compozy/internal/skills"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/transcript"
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
		if err := store.WriteSessionMeta(metaPath, &meta); err != nil {
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
	if err := store.WriteSessionMeta(session.MetaPath(), &meta); err != nil {
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
	if err := store.WriteSessionMeta(session.MetaPath(), &meta); err != nil {
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
	if err := store.WriteSessionMeta(session.MetaPath(), &meta); err != nil {
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

	for _, historyAvailable := range []bool{false, true} {
		t.Run(
			fmt.Sprintf("Should bound resume history with tool availability %t", historyAvailable),
			func(t *testing.T) {
				t.Parallel()
				h := newHarness(t)
				session := createSession(t, h)
				for index := range 300 {
					role := acp.EventTypeUserMessage
					if index%2 == 1 {
						role = acp.EventTypeAgentMessage
					}
					if err := h.manager.recordEvent(t.Context(), session, acp.AgentEvent{
						Type: role, TurnID: fmt.Sprintf("bounded-%d", index/2),
						Text:      fmt.Sprintf("message-%d ", index) + strings.Repeat("x", 1024),
						Timestamp: time.Now().UTC().Add(time.Duration(index) * time.Second),
					}); err != nil {
						t.Fatal(err)
					}
				}
				workspace, err := h.resolver.Resolve(t.Context(), h.workspaceID)
				if err != nil {
					t.Fatal(err)
				}
				workspace.Config.Session.Derive = compozyconfig.SessionDeriveConfig{
					MaxReplayBytes:  8192,
					MaxMessageBytes: 16384,
				}
				h.resolver.upsert(&workspace)
				block, _, err := h.manager.buildResumeReplay(t.Context(), session, rebuildReplayContext{
					workspace: &workspace, historyAvailable: historyAvailable, reason: "test",
				})
				if err != nil {
					t.Fatal(err)
				}
				messages := resumeReplayMessagesFromPrompt(t, block)
				if replayArrayBytes(messages) > 8192 || !strings.HasPrefix(messages[0].Content, "message-0 ") ||
					messages[1].ID != deriveOmittedMessageID {
					t.Fatalf("bounded replay is invalid: bytes=%d messages=%+v", replayArrayBytes(messages), messages)
				}
				if !strings.Contains(block, deriveWorkspaceLine) ||
					strings.Contains(block, "compozy__session_history tool") != historyAvailable {
					t.Fatalf("header tool availability or workspace authority invalid: %s", block)
				}
				if historyAvailable && !strings.Contains(block, "session_id: "+session.ID) {
					t.Fatal("history pointer does not name the session")
				}
			},
		)
	}

	t.Run("Should bound the rewound baseline before replay", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		session := createSession(t, h)
		baseline := deriveTestMessages(300, 1024)
		encoded, err := json.Marshal(baseline)
		if err != nil {
			t.Fatal(err)
		}
		recorder := &rewindBaselineRecorder{fakeEventRecorder: &fakeEventRecorder{},
			state: store.ConversationRewindState{MessagesJSON: string(encoded), CoveredThroughSequence: 600}}
		session.mu.Lock()
		original := session.recorder
		session.recorder = recorder
		session.mu.Unlock()
		defer func() { session.mu.Lock(); session.recorder = original; session.mu.Unlock() }()
		block, _, err := h.manager.buildResumeReplay(t.Context(), session)
		if err != nil {
			t.Fatal(err)
		}
		messages := resumeReplayMessagesFromPrompt(t, block)
		if replayArrayBytes(messages) > compozyconfig.DefaultSessionDeriveMaxReplayBytes ||
			messages[0].ID != baseline[0].ID ||
			messages[1].ID != deriveOmittedMessageID ||
			messages[len(messages)-1].ID != baseline[len(baseline)-1].ID {
			t.Fatalf(
				"rewound replay lost bound, pin, or tail: bytes=%d messages=%d",
				replayArrayBytes(messages),
				len(messages),
			)
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
	if err := store.WriteSessionMeta(session.MetaPath(), &meta); err != nil {
		t.Fatal(err)
	}
	h.manager = newManagerWithHarness(t, h)
	cleanupTestManager(t, h.manager)
	if err := h.manager.RecoverPendingStops(testutil.Context(t)); err != nil {
		t.Fatal(err)
	}
}

// TestResumeUpgradesPreFeatureLineageMetadata owns IT-019: session directories whose
// metadata predates lineage kinds resume with the kind the catalog backfill writes,
// keep their history, and persist the kind only on the next lifecycle write.
func TestResumeUpgradesPreFeatureLineageMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		lineage func(parentID string) *store.SessionLineage
		want    store.LineageKind
	}{
		{
			name:    "Should resume a parented session as provenance",
			lineage: func(parentID string) *store.SessionLineage { return &store.SessionLineage{ParentSessionID: parentID} },
			want:    store.LineageKindProvenance,
		},
		{
			name:    "Should resume a spawn-role session as spawn",
			lineage: func(string) *store.SessionLineage { return &store.SessionLineage{SpawnRole: "worker"} },
			want:    store.LineageKindSpawn,
		},
		{
			name:    "Should resume a root session without a kind",
			lineage: func(string) *store.SessionLineage { return nil },
			want:    store.LineageKindRoot,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := newHarness(t)
			parent := createSession(t, h)
			child, err := h.manager.Create(testutil.Context(t), CreateOpts{
				AgentName: "coder", Name: "pre-feature", Workspace: h.workspaceID, Lineage: tt.lineage(parent.ID),
			})
			if err != nil {
				t.Fatalf("Create(child) error = %v", err)
			}
			events, err := h.manager.Prompt(testutil.Context(t), child.ID, "history survives the upgrade")
			if err != nil {
				t.Fatalf("Prompt(child) error = %v", err)
			}
			collectEvents(t, events)
			if err := h.manager.Stop(testutil.Context(t), child.ID); err != nil {
				t.Fatalf("Stop(child) error = %v", err)
			}
			metaPath := child.MetaPath()
			stripLineageKind(t, metaPath)

			resumed, err := h.manager.Resume(testutil.Context(t), child.ID)
			if err != nil {
				t.Fatalf("Resume(pre-feature child) error = %v", err)
			}
			if got := resumed.Info().Lineage; got == nil || got.Kind != tt.want {
				t.Fatalf("resumed lineage = %#v, want kind %q", got, tt.want)
			}
			page, err := h.manager.TranscriptPage(testutil.Context(t), child.ID, transcript.PageQuery{Limit: 20})
			if err != nil {
				t.Fatalf("TranscriptPage(resumed) error = %v", err)
			}
			if !slices.ContainsFunc(page.Entries, func(entry transcript.Entry) bool {
				return transcript.UIMessageText(entry.Message) == "history survives the upgrade"
			}) {
				t.Fatalf("resumed transcript = %#v, want the pre-upgrade user message", page.Entries)
			}
			if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
				t.Fatalf("Stop(resumed) error = %v", err)
			}
			if got := readRawLineageKind(t, metaPath); got != string(tt.want) {
				t.Fatalf("persisted lineage kind after lifecycle write = %q, want %q", got, tt.want)
			}
		})
	}
}

func stripLineageKind(t *testing.T, metaPath string) {
	t.Helper()

	raw, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("ReadFile(meta) error = %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("json.Unmarshal(meta) error = %v", err)
	}
	if lineage, ok := document["lineage"].(map[string]any); ok {
		delete(lineage, "kind")
	}
	stripped, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent(meta) error = %v", err)
	}
	if err := os.WriteFile(metaPath, stripped, 0o644); err != nil {
		t.Fatalf("WriteFile(meta) error = %v", err)
	}
	if got := readRawLineageKind(t, metaPath); got != "" {
		t.Fatalf("stripped lineage kind = %q, want absent", got)
	}
}

func readRawLineageKind(t *testing.T, metaPath string) string {
	t.Helper()

	raw, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("ReadFile(meta) error = %v", err)
	}
	var document struct {
		Lineage *struct {
			Kind string `json:"kind"`
		} `json:"lineage"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("json.Unmarshal(meta) error = %v", err)
	}
	if document.Lineage == nil {
		return ""
	}
	return document.Lineage.Kind
}

func setReviewerChain(t *testing.T, h *harness, chain ...compozyconfig.RoleFallback) {
	t.Helper()
	workspace, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	for index := range workspace.Agents {
		if workspace.Agents[index].Name == "reviewer" {
			workspace.Agents[index].FallbackChain = chain
		}
	}
	h.resolver.upsert(&workspace)
}

func lastStartCall(h *harness) acp.StartOpts {
	h.driver.mu.Lock()
	defer h.driver.mu.Unlock()
	return h.driver.startCalls[len(h.driver.startCalls)-1]
}

// Invariant: resume loads the native id only on a configured route that matches the
// accepted binding record; otherwise the id is cleared and context replay runs on the
// primary route (IT-005, UT-026, UT-027, UT-029).
func TestResumeAcceptedRouteAffinity(t *testing.T) {
	t.Parallel()

	createOnSeatOne := func(t *testing.T, h *harness) *Session {
		t.Helper()
		installFallbackAgent(t, h, claudeSeatOneRoute(h), codexRoute(h))
		refuseStartCommands(h, map[string]error{fallbackSeatZero: rateLimitRefusal()})
		created, err := h.manager.Create(
			testutil.Context(t),
			CreateOpts{AgentName: "reviewer", Workspace: h.workspaceID},
		)
		if err != nil {
			t.Fatalf("Create(reviewer) error = %v", err)
		}
		meta := readMeta(t, created.MetaPath())
		if meta.AcceptedRoute == nil || meta.AcceptedRoute.Attempt != 1 || derefString(meta.ACPSessionID) != "acp-2" {
			t.Fatalf(
				"accepted route = %#v acp=%v, want seat one (attempt 1) with acp-2",
				meta.AcceptedRoute,
				meta.ACPSessionID,
			)
		}
		if err := h.manager.Stop(testutil.Context(t), created.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		return created
	}

	t.Run("Should resume on the matching route by record even when the chain is reordered", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		created := createOnSeatOne(t, h)
		setReviewerChain(t, h, codexRoute(h), claudeSeatOneRoute(h))
		resumed, err := h.manager.Resume(testutil.Context(t), created.ID)
		if err != nil {
			t.Fatalf("Resume() error = %v", err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, resumed.ID) })
		call := lastStartCall(h)
		if call.Command != fallbackSeatOne || call.ResumeSessionID != "acp-2" {
			t.Fatalf("resume start = %q/%q, want seat one loading acp-2", call.Command, call.ResumeSessionID)
		}
		meta := readMeta(t, resumed.MetaPath())
		if meta.AcceptedRoute == nil ||
			meta.AcceptedRoute.CommandFingerprint != providerCommandFingerprint(fallbackSeatOne) {
			t.Fatalf("accepted route after resume = %#v, want seat one", meta.AcceptedRoute)
		}
	})

	for _, tc := range []struct {
		name  string
		chain func(*harness) []compozyconfig.RoleFallback
	}{
		{name: "Should replay context on the primary route when the accepted route was removed", chain: func(h *harness) []compozyconfig.RoleFallback {
			return []compozyconfig.RoleFallback{codexRoute(h)}
		}},
		{name: "Should replay context on the primary route when the accepted route command changed", chain: func(h *harness) []compozyconfig.RoleFallback {
			edited := claudeSeatOneRoute(h)
			edited.Command = "SEAT=9 claude --acp"
			return []compozyconfig.RoleFallback{edited, codexRoute(h)}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			logs := newCaptureLogHandler()
			h := newHarness(t, WithLogger(slog.New(logs)))
			created := createOnSeatOne(t, h)
			setReviewerChain(t, h, tc.chain(h)...)
			refuseStartCommands(h, nil)
			resumed, err := h.manager.Resume(testutil.Context(t), created.ID)
			if err != nil {
				t.Fatalf("Resume() error = %v", err)
			}
			t.Cleanup(func() { reportSessionStop(t, h, resumed.ID) })
			call := lastStartCall(h)
			if call.Command != fallbackSeatZero || call.ResumeSessionID != "" {
				t.Fatalf(
					"resume start = %q/%q, want the primary route without a native id",
					call.Command,
					call.ResumeSessionID,
				)
			}
			record, ok := logs.FindByMessage("session.fallback.route_missing")
			if !ok || record.Level != slog.LevelInfo {
				t.Fatalf("route_missing log = %#v (found %t), want one info record", record, ok)
			}
			assertCapturedLogAttr(
				t,
				record,
				"accepted_route.command_fingerprint",
				providerCommandFingerprint(fallbackSeatOne),
			)
			marker := requireTranscriptMarker(t, h.manager, created.ID, transcript.MarkerSessionRecovered)
			if got := marker.Evidence["fallback_reason"]; got != acceptedRouteMissingReason {
				t.Fatalf("context rebuilt reason = %v, want %s", got, acceptedRouteMissingReason)
			}
			if _, found := logs.FindByMessage("session.resume.context_replay_fallback"); found {
				t.Fatal("recoverFailedResumeStart ran for an affinity mismatch")
			}
		})
	}

	t.Run("Should reset the removed route's speed and ACP options when resuming on the primary route",
		func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			seatOne := claudeSeatOneRoute(h)
			seatOne.Speed = speedpkg.SpeedFast
			seatOne.ACPOptions = []compozyconfig.ACPOptionSelection{{ID: "route_mode", ValueID: "seat_one"}}
			installFallbackAgent(t, h, seatOne, codexRoute(h))
			refuseStartCommands(h, map[string]error{fallbackSeatZero: rateLimitRefusal()})
			created, err := h.manager.Create(
				testutil.Context(t),
				CreateOpts{AgentName: "reviewer", Workspace: h.workspaceID},
			)
			if err != nil {
				t.Fatalf("Create(reviewer) error = %v", err)
			}
			if call := lastStartCall(h); call.Command != fallbackSeatOne || call.Speed != speedpkg.SpeedFast ||
				!hasACPOption(call.ACPOptions, "route_mode") {
				t.Fatalf("accepted start = %q speed=%q options=%+v, want seat one fast with its option",
					call.Command, call.Speed, call.ACPOptions)
			}
			if err := h.manager.Stop(testutil.Context(t), created.ID); err != nil {
				t.Fatalf("Stop() error = %v", err)
			}
			setReviewerChain(t, h, codexRoute(h))
			refuseStartCommands(h, nil)
			resumed, err := h.manager.Resume(testutil.Context(t), created.ID)
			if err != nil {
				t.Fatalf("Resume() error = %v", err)
			}
			t.Cleanup(func() { reportSessionStop(t, h, resumed.ID) })
			call := lastStartCall(h)
			if call.Command != fallbackSeatZero || call.ResumeSessionID != "" {
				t.Fatalf("resume start = %q/%q, want the primary route without a native id",
					call.Command, call.ResumeSessionID)
			}
			if call.Speed == speedpkg.SpeedFast || hasACPOption(call.ACPOptions, "route_mode") {
				t.Fatalf("primary resume speed=%q options=%+v, want the primary runtime without the removed route's",
					call.Speed, call.ACPOptions)
			}
		})

	t.Run("Should resume old metadata without an accepted route unchanged", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		session := createSession(t, h)
		originalACP := session.Info().ACPSessionID
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		meta := readMeta(t, session.MetaPath())
		meta.AcceptedRoute = nil
		if err := store.WriteSessionMeta(session.MetaPath(), &meta); err != nil {
			t.Fatalf("WriteSessionMeta() error = %v", err)
		}
		raw, err := os.ReadFile(session.MetaPath())
		if err != nil || strings.Contains(string(raw), "accepted_route") {
			t.Fatalf("previous-release meta fixture = %s (error %v), want no accepted_route", raw, err)
		}
		resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Resume() error = %v", err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, resumed.ID) })
		if got := lastStartCall(h).ResumeSessionID; got != originalACP || originalACP == "" {
			t.Fatalf("resume ResumeSessionID = %q, want stored %q", got, originalACP)
		}
	})
}

func hasACPOption(options []acp.SessionConfigOptionSelection, id string) bool {
	for _, option := range options {
		if option.ID == id {
			return true
		}
	}
	return false
}

func TestResumeFailsWhenWorkspaceDirectoryMissing(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	session := createSession(t, h)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if err := os.RemoveAll(h.workspace); err != nil {
		t.Fatalf("os.RemoveAll(%q) error = %v", h.workspace, err)
	}

	if _, err := h.manager.Resume(testutil.Context(t), session.ID); err == nil {
		t.Fatal("Resume(missing workspace dir) error = nil, want non-nil")
	} else if !strings.Contains(err.Error(), h.workspace) {
		t.Fatalf("Resume(missing workspace dir) error = %v, want workspace path %q", err, h.workspace)
	}
}

func TestResumeFailsWhenAgentRemoved(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	session := createSession(t, h)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
		ID:      h.workspaceID,
		RootDir: h.workspace,
		Name:    h.workspaceName,
		Config:  h.cfg,
		Agents: []compozyconfig.AgentDef{{
			Name:     compozyconfig.DefaultAgentName,
			Provider: "claude",
			Prompt:   "You are a coding assistant.",
		}},
	})

	if _, err := h.manager.Resume(testutil.Context(t), session.ID); err == nil {
		t.Fatal("Resume(missing agent) error = nil, want non-nil")
	} else if !strings.Contains(err.Error(), "coder") {
		t.Fatalf("Resume(missing agent) error = %v, want agent name", err)
	}
}

func TestResumeFailsWhenEventStoreIsEmpty(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	session := createSession(t, h)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if err := os.WriteFile(session.DBPath(), nil, 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", session.DBPath(), err)
	}

	if _, err := h.manager.Resume(testutil.Context(t), session.ID); err == nil {
		t.Fatal("Resume(empty event store) error = nil, want non-nil")
	} else if !strings.Contains(err.Error(), session.DBPath()) || !strings.Contains(err.Error(), "file is empty") {
		t.Fatalf("Resume(empty event store) error = %v, want db path and empty-file detail", err)
	}
}

// Invariant: session events remain authoritative across provider restarts and
// failed resume attempts. The session-manager resume suite owns this boundary.
func TestSessionEventHistoryAcrossResume(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve durable event history across resume and stop", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		active := createSession(t, h)
		first, err := h.manager.Prompt(testutil.Context(t), active.ID, "before resume")
		if err != nil {
			t.Fatal(err)
		}
		collectEvents(t, first)
		if err := h.manager.Stop(testutil.Context(t), active.ID); err != nil {
			t.Fatal(err)
		}
		before := readStoredEvents(t, active)
		resumed, err := h.manager.Resume(testutil.Context(t), active.ID)
		if err != nil {
			t.Fatal(err)
		}
		second, err := h.manager.Prompt(testutil.Context(t), resumed.ID, "after resume")
		if err != nil {
			t.Fatal(err)
		}
		collectEvents(t, second)
		if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
			t.Fatal(err)
		}
		after := readStoredEvents(t, resumed)
		for _, original := range before {
			found := false
			for _, event := range after {
				if event.ID == original.ID && event.Content == original.Content {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("resume lost durable event %q", original.ID)
			}
		}
		foundPrompt := false
		for _, event := range after {
			if strings.Contains(event.Content, "after resume") {
				foundPrompt = true
			}
		}
		if !foundPrompt || countEventType(after, EventTypeSessionStopped) != 2 {
			t.Fatalf("resumed history lost its new prompt or terminal event: %#v", after)
		}
	})
	t.Run("Should preserve stopped history when provider resume fails", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		active := createSession(t, h)
		events, err := h.manager.Prompt(testutil.Context(t), active.ID, "before failed resume")
		if err != nil {
			t.Fatal(err)
		}
		collectEvents(t, events)
		if err := h.manager.Stop(testutil.Context(t), active.ID); err != nil {
			t.Fatal(err)
		}
		before := readStoredEvents(t, active)
		startErr := errors.New("provider resume unavailable")
		h.driver.startHook = func(acp.StartOpts, int) (*fakeProcess, error) { return nil, startErr }
		if _, err := h.manager.Resume(testutil.Context(t), active.ID); !errors.Is(err, startErr) {
			t.Fatalf("Resume() error = %v, want %v", err, startErr)
		}
		after := readStoredEvents(t, active)
		for _, original := range before {
			found := false
			for _, event := range after {
				if event.ID == original.ID && event.Content == original.Content {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("failed resume lost durable event %q", original.ID)
			}
		}
		if meta := readMeta(t, active.MetaPath()); meta.State != string(StateStopped) {
			t.Fatalf("meta state after failed resume = %q, want %q", meta.State, StateStopped)
		}
	})
}

// Invariant: boot and lazy replay refresh a restored prefix once and retain the rewind exclusion.
// Owner: session replay; canonical suite: manager_resume_test.go.
func TestResumeReplayRefreshesMigratedRewindBaseline(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"boot", "lazy", "already-migrated"} {
		boot := mode != "lazy"
		name := "Should refresh a stale rewind baseline before lazy replay"
		if boot {
			name = "Should refresh a stale rewind baseline during retained database upgrade"
		}
		if mode == "already-migrated" {
			name = "Should refresh an already migrated stale baseline during retained database upgrade"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			accepted, err := h.manager.CreateAccepted(
				t.Context(),
				CreateAcceptedOpts{Session: CreateOpts{AgentName: "coder", Workspace: h.workspaceID}},
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := h.manager.Stop(t.Context(), accepted.ID); err != nil {
				t.Fatal(err)
			}
			owner, err := h.manager.SessionOwner(t.Context(), accepted.ID)
			if err != nil {
				t.Fatal(err)
			}
			path := store.SessionDBFile(filepath.Join(h.manager.homePaths.SessionsDir, accepted.ID))
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			for _, suffix := range []string{"", "-wal", "-shm"} {
				if err := os.Remove(path + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
					t.Fatal(err)
				}
			}
			dbOwner := store.SessionDBOwner{SessionID: owner.SessionID, WorkspaceID: owner.WorkspaceID}
			seedLegacyRewindDatabase(t, path, dbOwner)
			if mode == "already-migrated" {
				migrated, err := sessiondb.OpenSessionDB(t.Context(), dbOwner, path)
				if err != nil {
					t.Fatal(err)
				}
				if err := migrated.Close(t.Context()); err != nil {
					t.Fatal(err)
				}
			}
			if boot {
				if err := h.manager.UpgradeSessionDatabase(t.Context(), accepted.ID); err != nil {
					t.Fatal(err)
				}
			}
			db, err := sessiondb.OpenSessionDB(t.Context(), dbOwner, path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := db.Close(context.Background()); err != nil {
					t.Error(err)
				}
			})
			initial, found, err := db.ConversationRewindState(t.Context())
			if err != nil || !found || initial.BaselineStale == boot {
				t.Fatalf("before replay state=%+v found=%v boot=%v err=%v", initial, found, boot, err)
			}
			replaySession := &Session{
				ID:          accepted.ID,
				WorkspaceID: accepted.WorkspaceID,
				Workspace:   accepted.Workspace,
				AgentName:   accepted.AgentName,
				recorder:    db,
			}
			for range 2 {
				block, _, err := h.manager.buildResumeReplay(t.Context(), replaySession)
				if err != nil {
					t.Fatal(err)
				}
				messages := resumeReplayMessagesFromPrompt(t, block)
				if len(messages) != 149 {
					t.Fatalf("replay messages=%d want=149", len(messages))
				}
				for index, message := range messages {
					if got, want := message.Content, fmt.Sprintf("message-%03d", index+1); got != want {
						t.Fatalf("replay %d=%q want=%q", index, got, want)
					}
				}
				state, found, err := db.ConversationRewindState(t.Context())
				if err != nil || !found || state.BaselineStale {
					t.Fatalf("state=%+v found=%v err=%v", state, found, err)
				}
				rows, err := db.Query(
					t.Context(),
					store.EventQuery{BeforeSequence: 150, Archive: store.EventArchiveUnarchived},
				)
				if err != nil {
					t.Fatal(err)
				}
				expected, err := transcript.Assemble(rows)
				if err != nil {
					t.Fatal(err)
				}
				expected = transcript.Prune(expected, transcript.PruneOptions{Dedup: true})
				encoded, err := json.Marshal(expected)
				if err != nil {
					t.Fatal(err)
				}
				if state.MessagesJSON != string(encoded) {
					t.Fatalf("refreshed baseline differs from retained-prefix commit formula")
				}
			}
		})
	}
}

func seedLegacyRewindDatabase(t *testing.T, path string, owner store.SessionDBOwner) {
	t.Helper()
	stream := sessiondb.MigrationStream()
	files := fstest.MapFS{}
	directory := &atlasmigrate.MemDir{}
	entries, err := fs.ReadDir(stream.FS, stream.Dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") || entry.Name() >= "00009" {
			continue
		}
		data, err := fs.ReadFile(stream.FS, stream.Dir+"/"+entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		files[stream.Dir+"/"+entry.Name()] = &fstest.MapFile{Data: data}
		if err := directory.WriteFile(entry.Name(), data); err != nil {
			t.Fatal(err)
		}
	}
	checksum, err := directory.Checksum()
	if err != nil {
		t.Fatal(err)
	}
	data, err := checksum.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	files[stream.Dir+"/atlas.sum"] = &fstest.MapFile{Data: data}
	stream.FS, stream.Bootstrap = files, nil
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := store.Apply(t.Context(), db, stream); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(
		t.Context(),
		`INSERT INTO session_db_owner (singleton,session_id,workspace_id) VALUES (1,?,?)`,
		owner.SessionID,
		owner.WorkspaceID,
	); err != nil {
		t.Fatal(err)
	}
	at := "2026-09-01T00:00:00Z"
	retained := make([]store.SessionEvent, 0, 149)
	for seq := 1; seq <= 200; seq++ {
		archived := 0
		if seq >= 20 && seq <= 80 || seq >= 150 {
			archived = 1
		}
		content := fmt.Sprintf(`{"type":"user_message","text":"message-%03d","turn_id":"turn-%03d"}`, seq, seq)
		if archived == 0 && seq < 150 {
			retained = append(
				retained,
				store.SessionEvent{
					ID:        fmt.Sprintf("event-%d", seq),
					Sequence:  int64(seq),
					TurnID:    fmt.Sprintf("turn-%d", seq),
					Type:      "user_message",
					AgentName: "coder",
					Content:   content,
					Timestamp: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
				},
			)
		}
		if _, err := db.ExecContext(
			t.Context(),
			`INSERT INTO events (sequence,id,turn_id,type,agent_name,content,timestamp,archived) VALUES (?,?,?,?,?,?,?,?)`,
			seq,
			fmt.Sprintf("event-%d", seq),
			fmt.Sprintf("turn-%d", seq),
			"user_message",
			"coder",
			content,
			at,
			archived,
		); err != nil {
			t.Fatal(err)
		}
	}
	fixture, err := os.ReadFile("../store/sessiondb/testdata/legacy_compaction_fired.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(
		t.Context(),
		`INSERT INTO events (sequence,id,turn_id,type,agent_name,content,timestamp) VALUES (201,'legacy-fired','fixture-turn','session.compaction_fired','coder',?,?)`,
		string(fixture),
		at,
	); err != nil {
		t.Fatal(err)
	}
	messages, err := transcript.Assemble(retained)
	if err != nil {
		t.Fatal(err)
	}
	messages = transcript.Prune(messages, transcript.PruneOptions{Dedup: true})
	baseline, err := json.Marshal(messages)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(
		t.Context(),
		`INSERT INTO conversation_rewind_state (singleton,target_message_id,covered_through_sequence,messages_json,updated_at) VALUES (1,'message-150',149,?,?)`,
		string(baseline),
		at,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(
		t.Context(),
		`INSERT INTO conversation_rewind_receipts (idempotency_key,request_hash,target_message_id,archived_from_sequence,archived_to_sequence,archived_event_count,generation,max_sequence,transcript_epoch,draft_text,created_at) VALUES ('rewind','hash','message-150',150,200,51,1,149,1,'draft',?)`,
		at,
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}
