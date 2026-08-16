package cli

import (
	"errors"
	"os"
	"testing"
	"time"

	compozyupdate "github.com/compozy/compozy/internal/update"
)

func TestExecuteDaemonAppTransition(t *testing.T) {
	t.Run("Should recover only an interrupted irreversible app phase", func(t *testing.T) {
		t.Parallel()

		store := newCLIUpdateOperationStore(t)
		operation := &compozyupdate.Operation{
			ID: "operation-1", Revision: 7, ActiveTarget: compozyupdate.TargetApp,
			App: &compozyupdate.AppOperationState{Phase: compozyupdate.PhaseInstallerHandoff},
		}
		if !appOperationRecoverableByShell(store, operation, operation.ID, operation.Revision) {
			t.Fatal("appOperationRecoverableByShell() = false, want interrupted handoff recovery")
		}
		operation.App.Phase = compozyupdate.PhaseStaged
		if appOperationRecoverableByShell(store, operation, operation.ID, operation.Revision) {
			t.Fatal("appOperationRecoverableByShell(staged) = true, want refusal")
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
}
