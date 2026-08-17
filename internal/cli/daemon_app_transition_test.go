package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	compozyupdate "github.com/compozy/compozy/internal/update"
)

func TestExecuteDaemonAppTransition(t *testing.T) {
	t.Run("Should recover every interrupted shell-owned app phase", func(t *testing.T) {
		t.Parallel()

		store := newCLIUpdateOperationStore(t)
		for _, phase := range []compozyupdate.OperationPhase{
			compozyupdate.PhaseStaged,
			compozyupdate.PhaseApplying,
			compozyupdate.PhaseInstallerHandoff,
		} {
			operation := &compozyupdate.Operation{
				ID: "operation-1", Revision: 7, ActiveTarget: compozyupdate.TargetApp,
				App: &compozyupdate.AppOperationState{Phase: phase},
			}
			if !appOperationRecoverableByShell(store, operation, operation.ID, operation.Revision) {
				t.Fatalf("appOperationRecoverableByShell(%s) = false, want recovery", phase)
			}
		}
	})

	t.Run("Should acquire only an explicit staged app handoff", func(t *testing.T) {
		t.Parallel()

		store := newCLIUpdateOperationStore(t)
		operation := acquireCLIUpdateOperation(t, store, []compozyupdate.Target{compozyupdate.TargetApp})
		staged, err := store.Transition(
			t.Context(),
			operation.ID,
			operation.Holder.ExecutorGeneration,
			operation.Revision,
			compozyupdate.Transition{
				Kind:    compozyupdate.TransitionPhase,
				Actor:   compozyupdate.ActorCLI,
				Target:  compozyupdate.TargetApp,
				Phase:   compozyupdate.PhaseStaged,
				Percent: 100,
			},
		)
		if err != nil {
			t.Fatalf("Transition(staged) error = %v", err)
		}
		waiting, err := store.Transition(
			t.Context(),
			staged.ID,
			staged.Holder.ExecutorGeneration,
			staged.Revision,
			compozyupdate.Transition{
				Kind:    compozyupdate.TransitionWaitForApp,
				Actor:   compozyupdate.ActorCLI,
				Target:  compozyupdate.TargetApp,
				Percent: -1,
			},
		)
		if err != nil {
			t.Fatalf("Transition(waiting) error = %v", err)
		}

		acquired, err := executeDaemonAppTransition(
			t.Context(),
			store,
			time.Now().UTC(),
			os.Getpid(),
			daemonAppTransitionOptions{
				action:             "acquire",
				operationID:        waiting.ID,
				executorGeneration: "electron-generation",
				expectedRevision:   waiting.Revision,
			},
		)
		if err != nil {
			t.Fatalf("executeDaemonAppTransition(acquire) error = %v", err)
		}
		if acquired.ActiveTarget != compozyupdate.TargetApp || acquired.Waiting != compozyupdate.WaitingNone ||
			acquired.Holder == nil || acquired.Holder.Surface != compozyupdate.ActorShell ||
			acquired.Holder.ExecutorGeneration != "electron-generation" {
			t.Fatalf("acquired operation = %#v, want shell-owned app operation", acquired)
		}
	})

	t.Run("Should refuse an app operation before its handoff", func(t *testing.T) {
		t.Parallel()

		store := newCLIUpdateOperationStore(t)
		operation := acquireCLIUpdateOperation(t, store, []compozyupdate.Target{compozyupdate.TargetApp})

		_, err := executeDaemonAppTransition(
			t.Context(),
			store,
			time.Now().UTC(),
			os.Getpid(),
			daemonAppTransitionOptions{
				action:             "acquire",
				operationID:        operation.ID,
				executorGeneration: "electron-generation",
				expectedRevision:   operation.Revision,
			},
		)
		if !errors.Is(err, compozyupdate.ErrExecutorFenced) {
			t.Fatalf("executeDaemonAppTransition(acquire) error = %v, want ErrExecutorFenced", err)
		}
	})

	t.Run("Should wire renew phase progress digest verification and fence actions", func(t *testing.T) {
		t.Parallel()

		artifact := []byte("verified desktop installer")
		digest := sha256.Sum256(artifact)
		store := newCLIUpdateOperationStore(t)
		now := time.Now().UTC()
		operation, err := store.Acquire(t.Context(), compozyupdate.OperationRequest{
			RequestedBy: compozyupdate.ActorShell,
			Targets:     []compozyupdate.Target{compozyupdate.TargetApp},
			App: &compozyupdate.AppOperationState{
				ArtifactIdentity: compozyupdate.ArtifactIdentity{
					FromVersion: "v1.0.0", ToVersion: "v1.1.0", ReleaseTag: "v1.1.0",
					Asset: "app.zip", Digest: "sha256:" + hex.EncodeToString(digest[:]),
				},
				AttemptID: "attempt-1", Phase: compozyupdate.PhasePending,
			},
			Holder: compozyupdate.Holder{
				PID: os.Getpid(), PIDStartTime: mustProcessStartedAt(t), Surface: compozyupdate.ActorShell,
				ExecutorGeneration: "shell-generation", LeaseExpiresAt: now.Add(time.Minute),
			},
			Deadline: now.Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}
		staged, err := store.Transition(
			t.Context(), operation.ID, operation.Holder.ExecutorGeneration, operation.Revision,
			compozyupdate.Transition{
				Kind: compozyupdate.TransitionPhase, Actor: compozyupdate.ActorCLI,
				Target: compozyupdate.TargetApp, Phase: compozyupdate.PhaseStaged, Percent: 100,
			},
		)
		if err != nil {
			t.Fatalf("Transition(staged) error = %v", err)
		}
		waiting, err := store.Transition(
			t.Context(), staged.ID, staged.Holder.ExecutorGeneration, staged.Revision,
			compozyupdate.Transition{
				Kind: compozyupdate.TransitionWaitForApp, Actor: compozyupdate.ActorCLI,
				Target: compozyupdate.TargetApp, Percent: -1,
			},
		)
		if err != nil {
			t.Fatalf("Transition(waiting) error = %v", err)
		}
		operation, err = executeDaemonAppTransition(t.Context(), store, now, os.Getpid(), daemonAppTransitionOptions{
			action: appTransitionActionAcquire, operationID: waiting.ID,
			executorGeneration: "shell-generation", expectedRevision: waiting.Revision,
		})
		if err != nil {
			t.Fatalf("executeDaemonAppTransition(acquire) error = %v", err)
		}

		renewed, err := executeDaemonAppTransition(t.Context(), store, now, os.Getpid(), daemonAppTransitionOptions{
			action: appTransitionActionRenew, operationID: operation.ID,
			executorGeneration: "shell-generation", expectedRevision: operation.Revision,
		})
		if err != nil {
			t.Fatalf("executeDaemonAppTransition(renew) error = %v", err)
		}
		watchdog := now.Add(5 * time.Minute).Format(time.RFC3339Nano)
		applying, err := executeDaemonAppTransition(t.Context(), store, now, os.Getpid(), daemonAppTransitionOptions{
			action: appTransitionActionPhase, operationID: renewed.ID,
			executorGeneration: "shell-generation", expectedRevision: renewed.Revision,
			phase: string(compozyupdate.PhaseApplying), percent: 10, watchdogDeadline: watchdog,
		})
		if err != nil {
			t.Fatalf("executeDaemonAppTransition(phase) error = %v", err)
		}
		progressed, err := executeDaemonAppTransition(t.Context(), store, now, os.Getpid(), daemonAppTransitionOptions{
			action: appTransitionActionProgress, operationID: applying.ID,
			executorGeneration: "shell-generation", expectedRevision: applying.Revision, percent: 100,
		})
		if err != nil {
			t.Fatalf("executeDaemonAppTransition(progress) error = %v", err)
		}
		artifactPath := filepath.Join(t.TempDir(), "app.zip")
		if err := os.WriteFile(artifactPath, artifact, 0o600); err != nil {
			t.Fatalf("WriteFile(artifact) error = %v", err)
		}
		verified, err := executeDaemonAppTransition(t.Context(), store, now, os.Getpid(), daemonAppTransitionOptions{
			action: appTransitionActionVerifyArtifact, operationID: progressed.ID,
			executorGeneration: "shell-generation", expectedRevision: progressed.Revision,
			attemptID: "attempt-1", artifactPath: artifactPath,
		})
		if err != nil {
			t.Fatalf("executeDaemonAppTransition(verify-artifact) error = %v", err)
		}
		if verified.App == nil || !verified.App.DigestVerified {
			t.Fatalf("verified operation = %#v, want durable digest proof", verified)
		}
		fenced, err := executeDaemonAppTransition(t.Context(), store, now, os.Getpid(), daemonAppTransitionOptions{
			action: appTransitionActionFence, operationID: verified.ID,
			executorGeneration: "shell-generation", expectedRevision: verified.Revision,
		})
		if err != nil {
			t.Fatalf("executeDaemonAppTransition(fence) error = %v", err)
		}
		if fenced.Revision != verified.Revision || fenced.App == nil || !fenced.App.DigestVerified {
			t.Fatalf("fenced operation = %#v, want unchanged verified revision", fenced)
		}
	})

	t.Run("Should reject an invalid watchdog deadline", func(t *testing.T) {
		t.Parallel()
		_, err := executeDaemonAppTransition(
			t.Context(),
			newCLIUpdateOperationStore(t),
			time.Now().UTC(),
			os.Getpid(),
			daemonAppTransitionOptions{action: appTransitionActionPhase, watchdogDeadline: "not-a-timestamp"},
		)
		assertErrorContains(t, err, "parse app update watchdog deadline")
	})
}
