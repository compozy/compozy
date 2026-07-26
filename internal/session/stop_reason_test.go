package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestClassifyStopReason(t *testing.T) {
	t.Parallel()

	waitErr := errors.New("boom")
	tests := []struct {
		name       string
		cause      StopCause
		waitErr    error
		detail     string
		wantReason store.StopReason
		wantDetail string
	}{
		{
			name:       "Should classify shutdown without error as shutdown",
			cause:      CauseShutdown,
			wantReason: store.StopShutdown,
			wantDetail: "daemon shutdown",
		},
		{
			name:       "Should prefer shutdown over process error",
			cause:      CauseShutdown,
			waitErr:    waitErr,
			wantReason: store.StopShutdown,
			wantDetail: "daemon shutdown",
		},
		{
			name:       "Should classify user requested stop as user canceled",
			cause:      CauseUserRequested,
			wantReason: store.StopUserCanceled,
		},
		{
			name:       "Should classify max iterations subreason",
			cause:      CauseUserRequested,
			detail:     "max_iterations",
			wantReason: store.StopMaxIterations,
			wantDetail: "max_iterations",
		},
		{
			name:       "Should classify loop detected subreason",
			cause:      CauseUserRequested,
			detail:     "loop_detected",
			wantReason: store.StopLoopDetected,
			wantDetail: "loop_detected",
		},
		{
			name:       "Should classify budget exceeded subreason",
			cause:      CauseUserRequested,
			detail:     "budget_exceeded",
			wantReason: store.StopBudgetExceeded,
			wantDetail: "budget_exceeded",
		},
		{
			name:       "Should classify process exit with wait error as agent crashed",
			cause:      CauseProcessExited,
			waitErr:    waitErr,
			wantReason: store.StopAgentCrashed,
			wantDetail: "boom",
		},
		{
			name:       "Should classify process exit without wait error as generic error",
			cause:      CauseProcessExited,
			wantReason: store.StopError,
			wantDetail: "process exited unexpectedly",
		},
		{
			name:       "Should classify completion",
			cause:      CauseCompleted,
			wantReason: store.StopCompleted,
		},
		{
			name:       "Should classify clear conversation as completed with detail",
			cause:      CauseClearConversation,
			wantReason: store.StopCompleted,
			wantDetail: "conversation cleared",
		},
		{
			name:       "Should classify failure",
			cause:      CauseFailed,
			detail:     "automation prompt failed",
			wantReason: store.StopError,
			wantDetail: "automation prompt failed",
		},
		{
			name:       "Should classify hook denial as hook stopped",
			cause:      CauseHookDenied,
			detail:     "reason",
			wantReason: store.StopHookStopped,
			wantDetail: "reason",
		},
		{
			name:       "Should classify unknown cause with error as generic error",
			waitErr:    waitErr,
			wantReason: store.StopError,
			wantDetail: "boom",
		},
		{
			name:       "Should classify unknown cause without error as completed",
			wantReason: store.StopCompleted,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotReason, gotDetail := classifyStopReason(tc.cause, tc.waitErr, tc.detail)
			if gotReason != tc.wantReason {
				t.Fatalf("classifyStopReason() reason = %q, want %q", gotReason, tc.wantReason)
			}
			if gotDetail != tc.wantDetail {
				t.Fatalf("classifyStopReason() detail = %q, want %q", gotDetail, tc.wantDetail)
			}
		})
	}
}

func TestStopMethodsRejectNilManager(t *testing.T) {
	t.Parallel()

	t.Run("Should reject request stop on nil manager", func(t *testing.T) {
		t.Parallel()

		var nilManager *Manager
		err := nilManager.RequestStopWithCause(testutil.Context(t), "sess-1", CauseUserRequested, "")
		if err == nil || !strings.Contains(err.Error(), "manager is required") {
			t.Fatalf("RequestStopWithCause(nil manager) error = %v, want manager is required", err)
		}
	})

	t.Run("Should reject forced stop on nil manager", func(t *testing.T) {
		t.Parallel()

		var nilManager *Manager
		err := nilManager.StopWithCause(testutil.Context(t), "sess-1", CauseUserRequested, "")
		if err == nil || !strings.Contains(err.Error(), "manager is required") {
			t.Fatalf("StopWithCause(nil manager) error = %v, want manager is required", err)
		}
	})
}

func TestStopWithCauseFinalizesAlreadyExitedProcess(t *testing.T) {
	t.Parallel()

	t.Run("Should finalize already exited process with crash classification", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		proc := h.driver.lastProcess()
		if proc == nil {
			t.Fatal("lastProcess() = nil")
		}

		crashErr := errors.New("agent crashed before daemon shutdown")
		proc.crash(crashErr, "stderr: invalid frame")

		err := h.manager.StopWithCause(
			testutil.Context(t),
			session.ID,
			CauseShutdown,
			"daemon shutdown",
		)
		if err != nil {
			t.Fatalf("StopWithCause() error = %v, want nil after finalizing exited process", err)
		}

		meta := readMeta(t, session.MetaPath())
		if got, want := meta.State, string(StateStopped); got != want {
			t.Fatalf("meta state = %q, want %q", got, want)
		}
		if meta.StopReason == nil {
			t.Fatal("meta stop_reason = nil, want agent_crashed")
		}
		if got, want := *meta.StopReason, store.StopAgentCrashed; got != want {
			t.Fatalf("meta stop_reason = %q, want %q", got, want)
		}
		if got, want := meta.StopDetail, crashErr.Error(); got != want {
			t.Fatalf("meta stop_detail = %q, want %q", got, want)
		}
	})
}

func TestRequestStopWithCauseFinalizesAlreadyExitedProcess(t *testing.T) {
	t.Parallel()

	t.Run("Should finalize already exited process with crash classification", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		proc := h.driver.lastProcess()
		if proc == nil {
			t.Fatal("lastProcess() = nil")
		}

		crashErr := errors.New("agent crashed before cancel")
		proc.crash(crashErr, "stderr: disconnect")

		if err := h.manager.RequestStopWithCause(testutil.Context(t), session.ID, CauseUserRequested, ""); err != nil {
			t.Fatalf("RequestStopWithCause() error = %v, want nil after finalizing exited process", err)
		}

		meta := readMeta(t, session.MetaPath())
		if got, want := meta.State, string(StateStopped); got != want {
			t.Fatalf("meta state = %q, want %q", got, want)
		}
		if meta.StopReason == nil {
			t.Fatal("meta stop_reason = nil, want agent_crashed")
		}
		if got, want := *meta.StopReason, store.StopAgentCrashed; got != want {
			t.Fatalf("meta stop_reason = %q, want %q", got, want)
		}
		if got, want := meta.StopDetail, crashErr.Error(); got != want {
			t.Fatalf("meta stop_detail = %q, want %q", got, want)
		}
	})
}

func TestPrepareStopWithCauseWrapsStageFailures(t *testing.T) {
	t.Parallel()

	hookErr := errors.New("hook boom")

	tests := []struct {
		name          string
		setup         func(t *testing.T) (*Manager, *Session, context.Context)
		wantStage     string
		wantErr       error
		wantPathError bool
	}{
		{
			name: "Should wrap pre-stop hook failures",
			setup: func(t *testing.T) (*Manager, *Session, context.Context) {
				t.Helper()

				dispatcher := &spyHookDispatcher{
					dispatchSessionPreStopFn: func(
						_ context.Context,
						payload hookspkg.SessionPreStopPayload,
					) (hookspkg.SessionPreStopPayload, error) {
						return payload, hookErr
					},
				}
				h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))
				return h.manager, createSession(t, h), testutil.Context(t)
			},
			wantStage: "prepare stop pre-stop hooks",
			wantErr:   hookErr,
		},
		{
			name: "Should wrap state synchronization failures",
			setup: func(t *testing.T) (*Manager, *Session, context.Context) {
				t.Helper()

				h := newHarness(t)
				session := createSession(t, h)
				session.mu.Lock()
				session.State = State("invalid")
				session.mu.Unlock()
				return h.manager, session, testutil.Context(t)
			},
			wantStage: "prepare stop state sync",
			wantErr:   ErrInvalidStateTransition,
		},
		{
			name: "Should wrap metadata write failures",
			setup: func(t *testing.T) (*Manager, *Session, context.Context) {
				t.Helper()

				h := newHarness(t)
				session := createSession(t, h)
				blockingPath := filepath.Join(t.TempDir(), "meta-parent")
				if err := os.WriteFile(blockingPath, []byte("block"), 0o644); err != nil {
					t.Fatalf("WriteFile(blockingPath) error = %v", err)
				}
				session.mu.Lock()
				session.metaPath = filepath.Join(blockingPath, "session.json")
				session.mu.Unlock()
				return h.manager, session, testutil.Context(t)
			},
			wantStage:     "prepare stop lifecycle write",
			wantPathError: true,
		},
		{
			name: "Should wrap prompt setup wait failures",
			setup: func(t *testing.T) (*Manager, *Session, context.Context) {
				t.Helper()

				h := newHarness(t)
				session := createSession(t, h)
				if err := session.beginPromptSetup(); err != nil {
					t.Fatalf("beginPromptSetup() error = %v", err)
				}
				ctx, cancel := context.WithCancel(testutil.Context(t))
				cancel()
				return h.manager, session, ctx
			},
			wantStage: "prepare stop prompt setup wait",
			wantErr:   context.Canceled,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			manager, session, ctx := tc.setup(t)
			_, _, _, _, _, err := manager.prepareStopWithCause(ctx, session.ID, CauseUserRequested, "")
			if err == nil {
				t.Fatal("prepareStopWithCause() error = nil, want wrapped stage failure")
			}
			if !strings.Contains(err.Error(), tc.wantStage) {
				t.Fatalf("prepareStopWithCause() error = %v, want stage context %q", err, tc.wantStage)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("prepareStopWithCause() error = %v, want wrapped %v", err, tc.wantErr)
			}
			if tc.wantPathError {
				var pathErr *os.PathError
				if !errors.As(err, &pathErr) {
					t.Fatalf("prepareStopWithCause() error = %v, want wrapped *os.PathError", err)
				}
			}
		})
	}
}
