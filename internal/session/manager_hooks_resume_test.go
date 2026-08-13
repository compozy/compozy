package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/testutil"
)

func TestResumeUsesPatchedPreResumePayloadAndFiresPostResume(t *testing.T) {
	t.Parallel()
	t.Run("Should resume a bound worktree with patched hook state", func(t *testing.T) {
		t.Parallel()

		worktreeRoot := filepath.Join(t.TempDir(), "resume-hook-worktree")
		if err := os.MkdirAll(worktreeRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll(worktree root) error = %v", err)
		}
		resolver := &fakeSessionWorktreeResolver{id: "wt-resume-hook", root: worktreeRoot}
		h := newHarness(t, WithWorktreeResolver(resolver))
		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Worktree: "wt-resume-hook",
		})
		if err != nil {
			t.Fatalf("Create(bound) error = %v", err)
		}
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		const patchedName = "resumed-patched"
		postResumeCh := make(chan hookspkg.SessionPostResumePayload, 1)
		dispatcher := &spyHookDispatcher{
			dispatchSessionPreResumeFn: func(_ context.Context, payload hookspkg.SessionPreResumePayload) (hookspkg.SessionPreResumePayload, error) {
				if payload.WorktreeID != "wt-resume-hook" {
					t.Fatalf("session.pre_resume worktree_id = %q, want wt-resume-hook", payload.WorktreeID)
				}
				payload.SessionName = patchedName
				return payload, nil
			},
			dispatchSessionPostResumeFn: func(_ context.Context, payload hookspkg.SessionPostResumePayload) (hookspkg.SessionPostResumePayload, error) {
				postResumeCh <- payload
				return payload, nil
			},
		}

		h.manager = newManagerWithHarness(
			t,
			h,
			WithWorktreeResolver(resolver),
			WithHookSet(fullHookSet(dispatcher)),
		)
		resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Resume() error = %v", err)
		}
		t.Cleanup(func() {
			reportSessionStop(t, h, resumed.ID)
		})

		if got := resumed.Info().Name; got != patchedName {
			t.Fatalf("resumed name = %q, want %q", got, patchedName)
		}

		select {
		case payload := <-postResumeCh:
			if payload.SessionID != resumed.ID {
				t.Fatalf("payload.SessionID = %q, want %q", payload.SessionID, resumed.ID)
			}
			if payload.State != string(StateActive) {
				t.Fatalf("payload.State = %q, want %q", payload.State, StateActive)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for session.post_resume hook")
		}
	})
}

func TestResumeRejectsPatchedWorkspaceForExistingSession(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	patchedWorkspace, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
	if err != nil {
		t.Fatalf("Resolve(primary workspace) error = %v", err)
	}
	patchedWorkspace.ID = "ws-pre-resume-patched"
	patchedWorkspace.WorkspaceID = patchedWorkspace.ID
	patchedWorkspace.Name = "pre-resume-patched"
	h.resolver.upsert(&patchedWorkspace)

	dispatcher := &spyHookDispatcher{
		dispatchSessionPreResumeFn: func(
			_ context.Context,
			payload hookspkg.SessionPreResumePayload,
		) (hookspkg.SessionPreResumePayload, error) {
			payload.WorkspaceID = patchedWorkspace.ID
			return payload, nil
		},
	}
	h.manager = newManagerWithHarness(t, h, WithHookSet(fullHookSet(dispatcher)))
	startCallsBeforeResume := len(h.driver.startCalls)

	_, err = h.manager.Resume(testutil.Context(t), session.ID)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("Resume(workspace mismatch) error = %v, want %v", err, ErrValidation)
	}
	if got := len(h.driver.startCalls); got != startCallsBeforeResume {
		t.Fatalf("driver start calls = %d, want unchanged %d", got, startCallsBeforeResume)
	}
	if got := len(h.manager.List()); got != 0 {
		t.Fatalf("active sessions after rejected resume = %d, want 0", got)
	}
}
