package session

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

// Invariant: a failure retained on an interrupted session blocks restart continuation only when it
// records a genuine agent failure. A start-infrastructure failure continues, and the successful
// resume clears it from persisted metadata.
// Owner: session manager; restart qualification. The ACP boundary is fake; checkpoints and admissions use SQLite.
func TestRestartContinuationRetainedFailure(t *testing.T) {
	for _, tc := range []struct {
		name       string
		failure    store.SessionFailure
		wantResume bool
	}{
		{
			name:       "Should continue past a hosted MCP bind deadline",
			failure:    store.SessionFailure{Kind: store.FailureTimeout, Summary: "hosted MCP bind deadline exceeded"},
			wantResume: true,
		},
		{
			name:       "Should continue past a start canceled by daemon shutdown",
			failure:    store.SessionFailure{Kind: store.FailureCanceled, Summary: "session start canceled"},
			wantResume: true,
		},
		{
			name: "Should continue past an adapter prepare failure",
			failure: store.SessionFailure{
				Kind: store.FailureStartup, ReasonCode: store.FailureReasonNPXPrepare, Summary: "adapter prepare failed",
			},
			wantResume: true,
		},
		{
			name: "Should continue past an initialize timeout",
			failure: store.SessionFailure{
				Kind: store.FailureHandshake, ReasonCode: store.FailureReasonNPXInitialize, Summary: "initialize timed out",
			},
			wantResume: true,
		},
		{
			name:       "Should continue past a provider transport loss during start",
			failure:    store.SessionFailure{Kind: store.FailureTransport, Summary: "provider closed the pipe"},
			wantResume: true,
		},
		{
			name:    "Should require operator input after an agent crash",
			failure: store.SessionFailure{Kind: store.FailureProcess, Summary: "agent exited"},
		},
		{
			name:    "Should require operator input after a prompt failure",
			failure: store.SessionFailure{Kind: store.FailurePrompt, Summary: "prompt failed"},
		},
		{
			name: "Should require operator input after an incomplete prompt stream",
			failure: store.SessionFailure{
				Kind:       store.FailureTransport,
				ReasonCode: store.FailureReasonPromptStreamIncomplete,
				Summary:    "prompt stream ended early",
			},
		},
		{
			name:    "Should require operator input after a provider authentication failure",
			failure: store.SessionFailure{Kind: store.FailureProviderAuth, Summary: "login required"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			admissions := openManagerInputQueueStore(t)
			h := newHarness(
				t,
				WithSessionStopConfig(compozyconfig.SessionStopConfig{CooperativeGrace: time.Millisecond}),
				WithSessionPromptAdmissionStore(admissions),
				WithSessionInputQueueStore(admissions),
			)
			registerManagerInputQueueWorkspace(t, admissions, h)
			target, err := h.manager.Create(t.Context(), new(CreateOpts{
				AgentName: "coder", Name: "session", Workspace: h.workspaceID,
			}))
			if err != nil {
				t.Fatal(err)
			}
			registerManagerInputQueueSession(t, admissions, h, target)
			t.Cleanup(func() { reportSessionStop(t, h, target.ID) })
			source := make(chan acp.AgentEvent)
			h.driver.promptHook = func(_ *fakeProcess, _ acp.PromptRequest) (<-chan acp.AgentEvent, error) {
				return source, nil
			}
			stream, err := h.manager.PromptWithOpts(t.Context(), target.ID, PromptOpts{Message: "finish the work"})
			if err != nil {
				t.Fatal(err)
			}
			if err := h.manager.PrepareDaemonShutdown(t.Context(), true); err != nil {
				t.Fatal(err)
			}
			if err := h.manager.StopWithCause(t.Context(), target.ID, CauseShutdown, "daemon restart"); err != nil {
				t.Fatal(err)
			}
			close(source)
			collectEvents(t, stream)
			h.driver.mu.Lock()
			h.driver.promptHook = nil
			h.driver.mu.Unlock()
			infos, err := h.manager.ListAll(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if err := h.manager.CaptureRestartInterruptions(t.Context(), infos); err != nil {
				t.Fatal(err)
			}
			// A start attempt between the interruption and this recovery left its failure behind.
			meta := readMeta(t, target.MetaPath())
			meta.Failure = new(tc.failure)
			if err := store.WriteSessionMeta(target.MetaPath(), &meta); err != nil {
				t.Fatal(err)
			}

			resumed, err := h.manager.ResumeInterruptedSession(t.Context(), target.ID)
			if resumed != tc.wantResume {
				t.Fatalf("resumed = %t, want %t; error %v", resumed, tc.wantResume, err)
			}
			after := readMeta(t, target.MetaPath())
			if tc.wantResume {
				if err != nil {
					t.Fatal(err)
				}
				if after.Failure != nil {
					t.Fatalf("meta failure after resume = %+v, want cleared", after.Failure)
				}
				return
			}
			if !errors.Is(err, ErrRestartContinuationSkipped) || !strings.Contains(err.Error(), "agent_failure") {
				t.Fatalf("continuation error = %v, want agent_failure skip", err)
			}
			if after.Failure == nil || after.Failure.Kind != tc.failure.Kind {
				t.Fatalf("meta failure after skip = %+v, want kind %q", after.Failure, tc.failure.Kind)
			}
		})
	}
}

// Invariant: resuming a crashed session keeps its crash stop classification for display but clears
// the retained failure once a runtime is bound, in memory and in persisted metadata.
// Owner: session manager; resume activation.
func TestResumeClearsRetainedCrashFailure(t *testing.T) {
	t.Parallel()

	t.Run("Should keep the crash stop reason and drop the failure after a successful resume", func(t *testing.T) {
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
		t.Cleanup(func() { reportSessionStop(t, h, resumed.ID) })

		info := resumed.Info()
		if info.StopReason != store.StopAgentCrashed || info.Failure != nil {
			t.Fatalf("resumed stop reason = %q, failure = %+v; want crash reason without failure",
				info.StopReason, info.Failure)
		}
		persisted := readMeta(t, resumed.MetaPath())
		if persisted.StopReason == nil || *persisted.StopReason != store.StopAgentCrashed || persisted.Failure != nil {
			t.Fatalf("persisted stop reason = %v, failure = %+v; want crash reason without failure",
				persisted.StopReason, persisted.Failure)
		}
	})
}
