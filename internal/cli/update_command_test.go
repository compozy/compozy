package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/procutil"
	compozyupdate "github.com/compozy/compozy/internal/update"
)

func TestUpdateCommandContract(t *testing.T) {
	t.Run("Should reproduce the frozen multi-target check transcript byte for byte", func(t *testing.T) {
		t.Parallel()

		state := compozyupdate.MultiState{
			Aggregate: compozyupdate.StatusAvailable,
			Runtime: compozyupdate.RuntimeTrackState{
				Status: compozyupdate.StatusAvailable, InstallMethod: "desktop-app", Managed: false,
				CurrentVersion: "0.5.0", LatestVersion: "0.5.1",
				ReleaseURL:      "https://github.com/compozy/compozy/releases/tag/v0.5.1",
				DaemonRestarted: false, Message: "CompozyOS runtime 0.5.1 is available.",
			},
			App: &compozyupdate.AppTrackState{
				Status: compozyupdate.StatusAvailable, Running: true,
				CurrentVersion: "0.5.0", LatestVersion: "0.5.1",
				Message: "CompozyOS app 0.5.1 is available.",
			},
		}
		deps := newTestDeps(t, &stubClient{})
		deps.newUpdateManager = func(compozyconfig.HomePaths) (updateManager, error) {
			return stubUpdateManager{
				checkAllFn: func(context.Context, compozyupdate.CheckOptions) (compozyupdate.MultiState, *compozyupdate.Release, error) {
					return state, &compozyupdate.Release{Version: "0.5.1"}, nil
				},
			}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "update", "--check", "-o", "json")
		if err != nil {
			t.Fatalf("executeRootCommand() error = %v", err)
		}
		const want = "{\n" +
			"  \"status\": \"available\",\n" +
			"  \"runtime\": {\n" +
			"    \"status\": \"available\",\n" +
			"    \"install_method\": \"desktop-app\",\n" +
			"    \"managed\": false,\n" +
			"    \"current_version\": \"0.5.0\",\n" +
			"    \"latest_version\": \"0.5.1\",\n" +
			"    \"release_url\": \"https://github.com/compozy/compozy/releases/tag/v0.5.1\",\n" +
			"    \"daemon_restarted\": false,\n" +
			"    \"message\": \"CompozyOS runtime 0.5.1 is available.\"\n" +
			"  },\n" +
			"  \"app\": {\n" +
			"    \"status\": \"available\",\n" +
			"    \"running\": true,\n" +
			"    \"current_version\": \"0.5.0\",\n" +
			"    \"latest_version\": \"0.5.1\",\n" +
			"    \"message\": \"CompozyOS app 0.5.1 is available.\"\n" +
			"  }\n" +
			"}\n"
		if stdout != want {
			t.Fatalf("update --check JSON =\n%s\nwant byte-identical:\n%s", stdout, want)
		}

		human, _, err := executeRootCommand(t, deps, "update", "--check")
		if err != nil {
			t.Fatalf("executeRootCommand(human) error = %v", err)
		}
		toon, _, err := executeRootCommand(t, deps, "update", "--check", "-o", "toon")
		if err != nil {
			t.Fatalf("executeRootCommand(toon) error = %v", err)
		}
		for _, output := range []string{human, toon} {
			assertOrderedSubstrings(t, output, []string{"available", "0.5.0", "0.5.1", "desktop-app", "running"})
		}
	})

	t.Run("Should emit the nested multi-target check record", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
		deps.newUpdateManager = func(compozyconfig.HomePaths) (updateManager, error) {
			return stubUpdateManager{
				checkFn: func(_ context.Context, opts compozyupdate.CheckOptions) (compozyupdate.State, *compozyupdate.Release, error) {
					if !opts.ForceRefresh {
						t.Fatal("CheckOptions.ForceRefresh = false, want true")
					}
					return compozyupdate.State{
						Supported: true, InstallMethod: string(compozyupdate.InstallMethodDirectBinary),
						CurrentVersion: "v1.0.0", LatestVersion: "v1.1.0", Available: true,
						Status: compozyupdate.StatusAvailable, ReleaseURL: "https://example.test/v1.1.0",
						Message: "CompozyOS runtime v1.1.0 is available.",
					}, &compozyupdate.Release{Version: "v1.1.0"}, nil
				},
			}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "update", "--check", "-o", "json")
		if err != nil {
			t.Fatalf("executeRootCommand() error = %v", err)
		}
		var record updateRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(update) error = %v", err)
		}
		if record.Status != compozyupdate.StatusAvailable || record.Runtime.Status != compozyupdate.StatusAvailable ||
			record.Runtime.CurrentVersion != "v1.0.0" || record.Runtime.LatestVersion != "v1.1.0" {
			t.Fatalf("update record = %#v, want nested available runtime", record)
		}
	})

	t.Run("Should reject combining check and cancel", func(t *testing.T) {
		t.Parallel()

		_, _, err := executeRootCommand(t, newTestDeps(t, &stubClient{}), "update", "--check", "--cancel")
		if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
			t.Fatalf("executeRootCommand() error = %v, want mutually exclusive flags", err)
		}
	})

	t.Run("Should decline cancel while an executor lease is live", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
		store := newCLIUpdateOperationStore(t)
		operation := acquireCLIUpdateOperation(t, store, []compozyupdate.Target{compozyupdate.TargetRuntime})
		deps.newUpdateManager = func(compozyconfig.HomePaths) (updateManager, error) {
			return stubUpdateManager{operationStore: store}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "update", "--cancel", "-o", "json")
		if err == nil {
			t.Fatal("executeRootCommand() error = nil, want live-executor refusal")
		}
		var record updateCancelRecord
		if decodeErr := json.Unmarshal([]byte(stdout), &record); decodeErr != nil {
			t.Fatalf("json.Unmarshal(cancel) error = %v", decodeErr)
		}
		if record.Status != compozyupdate.StatusBlocked || record.OperationID != operation.ID || record.Holder == nil {
			t.Fatalf("cancel record = %#v, want blocked holder", record)
		}
	})

	t.Run("Should cancel a dormant staged app operation", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
		store := newCLIUpdateOperationStore(t)
		operation := acquireCLIUpdateOperation(t, store, []compozyupdate.Target{compozyupdate.TargetApp})
		staged, err := store.Transition(
			t.Context(),
			operation.ID,
			operation.Holder.ExecutorGeneration,
			operation.Revision,
			compozyupdate.Transition{
				Kind: compozyupdate.TransitionPhase, Actor: compozyupdate.ActorCLI,
				Target: compozyupdate.TargetApp, Phase: compozyupdate.PhaseStaged, Percent: 100,
			},
		)
		if err != nil {
			t.Fatalf("Transition(staged) error = %v", err)
		}
		_, err = store.Transition(t.Context(), staged.ID, staged.Holder.ExecutorGeneration, staged.Revision,
			compozyupdate.Transition{
				Kind: compozyupdate.TransitionWaitForApp, Actor: compozyupdate.ActorCLI,
				Target: compozyupdate.TargetApp, Percent: -1,
			})
		if err != nil {
			t.Fatalf("Transition(waiting) error = %v", err)
		}
		deps.newUpdateManager = func(compozyconfig.HomePaths) (updateManager, error) {
			return stubUpdateManager{operationStore: store}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "update", "--cancel", "-o", "json")
		if err != nil {
			t.Fatalf("executeRootCommand() error = %v", err)
		}
		var record updateCancelRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(cancel) error = %v", err)
		}
		if record.Status != compozyupdate.StatusCanceled || record.OperationID != operation.ID {
			t.Fatalf("cancel record = %#v, want canceled operation", record)
		}
	})
}

func TestUpdateTerminalProjection(t *testing.T) {
	t.Run("Should report runtime-first success and a staged closed app", func(t *testing.T) {
		t.Parallel()

		record := updateRecord{
			Runtime: compozyupdate.RuntimeTrackState{
				Status: compozyupdate.StatusAvailable, CurrentVersion: "v1.0.0", LatestVersion: "v1.1.0",
			},
			App: &compozyupdate.AppTrackState{
				Status: compozyupdate.StatusAvailable, CurrentVersion: "v1.0.0", LatestVersion: "v1.1.0",
			},
		}
		got := completedUpdateRecord(record, compozyupdate.OperationRequest{
			Runtime: &compozyupdate.RuntimeOperationState{
				ArtifactIdentity: compozyupdate.ArtifactIdentity{ToVersion: "v1.1.0"},
			},
			App: &compozyupdate.AppOperationState{
				ArtifactIdentity: compozyupdate.ArtifactIdentity{ToVersion: "v1.1.0"}, AttemptID: "attempt-1",
			},
		}, nil)
		if got.Status != compozyupdate.StatusStaged || got.Runtime.Status != compozyupdate.StatusUpdated ||
			!got.Runtime.DaemonRestarted || got.App == nil || got.App.Status != compozyupdate.StatusStaged {
			t.Fatalf("completedUpdateRecord() = %#v, want updated runtime and staged app", got)
		}
	})

	t.Run("Should keep app-only failures and blocks off the runtime track", func(t *testing.T) {
		t.Parallel()

		base := updateRecord{
			Runtime: compozyupdate.RuntimeTrackState{Status: compozyupdate.StatusUpToDate},
			App:     &compozyupdate.AppTrackState{Status: compozyupdate.StatusAvailable},
		}
		failed := failedUpdateRecord(base, errors.New("app planning failed"), compozyupdate.TargetApp)
		if failed.Status != compozyupdate.StatusFailed || failed.Runtime.Status != compozyupdate.StatusUpToDate ||
			failed.App == nil || failed.App.Status != compozyupdate.StatusFailed || failed.App.LastError != "app planning failed" {
			t.Fatalf("failedUpdateRecord(app) = %#v, want app-only failure", failed)
		}

		blocked := blockedUpdateRecord(
			base,
			&compozyupdate.Operation{Holder: &compozyupdate.Holder{PID: 4242}},
			compozyupdate.TargetApp,
		)
		if blocked.Status != compozyupdate.StatusBlocked || blocked.Runtime.Status != compozyupdate.StatusUpToDate ||
			blocked.App == nil || blocked.App.Status != compozyupdate.StatusBlocked ||
			!strings.Contains(blocked.App.Message, "holder pid 4242") {
			t.Fatalf("blockedUpdateRecord(app) = %#v, want app-only block with holder", blocked)
		}
	})

	t.Run("Should wait for a running app and report its archived verified outcome", func(t *testing.T) {
		t.Parallel()

		store := newCLIUpdateOperationStore(t)
		operation := acquireCLIUpdateOperation(t, store, []compozyupdate.Target{compozyupdate.TargetApp})
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

		done := make(chan error, 1)
		go func() {
			appRequest := *waiting.App
			appRequest.Phase = compozyupdate.PhasePending
			request := compozyupdate.OperationRequest{
				RequestedBy: compozyupdate.ActorShell,
				Targets:     []compozyupdate.Target{compozyupdate.TargetApp},
				App:         &appRequest,
				Holder: compozyupdate.Holder{
					PID: os.Getpid(), PIDStartTime: mustProcessStartedAt(t), Surface: compozyupdate.ActorShell,
					ExecutorGeneration: "shell-generation", LeaseExpiresAt: time.Now().UTC().Add(time.Minute),
				},
				Deadline: time.Now().UTC().Add(time.Minute),
			}
			current, transitionErr := store.Acquire(t.Context(), request)
			for _, phase := range []compozyupdate.OperationPhase{
				compozyupdate.PhaseApplying,
				compozyupdate.PhaseInstallerHandoff,
				compozyupdate.PhaseRestarted,
				compozyupdate.PhaseVerified,
			} {
				if transitionErr != nil {
					break
				}
				current, transitionErr = store.Transition(
					t.Context(), current.ID, current.Holder.ExecutorGeneration, current.Revision,
					compozyupdate.Transition{
						Kind: compozyupdate.TransitionPhase, Actor: compozyupdate.ActorShell,
						Target: compozyupdate.TargetApp, Phase: phase, Percent: 100,
					},
				)
			}
			done <- transitionErr
		}()

		deps := newTestDeps(t, &stubClient{})
		deps.pollInterval = time.Millisecond
		deps.now = func() time.Time { return time.Now().UTC() }
		archived, err := waitForAppUpdateCompletion(
			t.Context(), deps, store, operation.ID, time.Now().UTC().Add(time.Second),
		)
		if err != nil {
			t.Fatalf("waitForAppUpdateCompletion() error = %v", err)
		}
		if transitionErr := <-done; transitionErr != nil {
			t.Fatalf("shell simulator error = %v", transitionErr)
		}
		record := applyArchivedAppOutcome(updateRecord{
			Runtime: compozyupdate.RuntimeTrackState{Status: compozyupdate.StatusUpdated},
			App: &compozyupdate.AppTrackState{
				Status: compozyupdate.StatusStaged, CurrentVersion: "v1.0.0", LatestVersion: "v1.1.0",
			},
		}, archived)
		if record.Status != compozyupdate.StatusUpdated || record.App == nil ||
			record.App.Status != compozyupdate.StatusUpdated || record.App.CurrentVersion != "v1.1.0" ||
			record.App.Message != "CompozyOS app is restarting on v1.1.0." {
			t.Fatalf("verified app record = %#v, want frozen updated outcome", record)
		}
	})
}

func TestSettingsRestartTimeout(t *testing.T) {
	t.Run("Should enforce the restart timeout floor", func(t *testing.T) {
		t.Parallel()

		timeout := settingsRestartTimeout(commandDeps{startTimeout: 15 * time.Second, stopTimeout: 15 * time.Second})
		if timeout != defaultSettingsRestartTimeout {
			t.Fatalf("settingsRestartTimeout() = %s, want %s", timeout, defaultSettingsRestartTimeout)
		}
	})

	t.Run("Should extend the timeout above the floor", func(t *testing.T) {
		t.Parallel()

		timeout := settingsRestartTimeout(commandDeps{startTimeout: 25 * time.Second, stopTimeout: 25 * time.Second})
		if timeout != 55*time.Second {
			t.Fatalf("settingsRestartTimeout() = %s, want 55s", timeout)
		}
	})
}

func newCLIUpdateOperationStore(t *testing.T) *compozyupdate.OperationStore {
	t.Helper()
	store, err := compozyupdate.NewOperationStore(compozyconfig.HomePaths{HomeDir: t.TempDir()}, nil)
	if err != nil {
		t.Fatalf("NewOperationStore() error = %v", err)
	}
	return store
}

func acquireCLIUpdateOperation(
	t *testing.T,
	store *compozyupdate.OperationStore,
	targets []compozyupdate.Target,
) *compozyupdate.Operation {
	t.Helper()
	now := time.Now().UTC()
	request := compozyupdate.OperationRequest{
		RequestedBy: compozyupdate.ActorCLI,
		Targets:     targets,
		Holder: compozyupdate.Holder{
			PID: os.Getpid(), PIDStartTime: mustProcessStartedAt(t), Surface: compozyupdate.ActorCLI,
			ExecutorGeneration: "generation-1", LeaseExpiresAt: now.Add(time.Hour),
		},
		Deadline: now.Add(time.Hour),
	}
	for _, target := range targets {
		switch target {
		case compozyupdate.TargetRuntime:
			request.Runtime = &compozyupdate.RuntimeOperationState{
				ArtifactIdentity: compozyupdate.ArtifactIdentity{
					FromVersion: "v1.0.0", ToVersion: "v1.1.0", ReleaseTag: "v1.1.0",
					Asset: "runtime.tar.gz", Digest: "sha256:runtime",
				},
				InstallMethod: compozyupdate.InstallMethodDirectBinary, Phase: compozyupdate.PhasePending,
			}
		case compozyupdate.TargetApp:
			request.App = &compozyupdate.AppOperationState{
				ArtifactIdentity: compozyupdate.ArtifactIdentity{
					FromVersion: "v1.0.0", ToVersion: "v1.1.0", ReleaseTag: "v1.1.0",
					Asset: "app.zip", Digest: "sha256:app",
				},
				AttemptID: "attempt-1", Phase: compozyupdate.PhasePending,
			}
		}
	}
	operation, err := store.Acquire(t.Context(), request)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	return operation
}

func mustProcessStartedAt(t *testing.T) time.Time {
	t.Helper()
	startedAt, err := procutil.StartedAt(os.Getpid())
	if err != nil {
		t.Fatalf("procutil.StartedAt() error = %v", err)
	}
	return startedAt
}

func assertOrderedSubstrings(t *testing.T, output string, values []string) {
	t.Helper()
	position := 0
	for _, value := range values {
		index := strings.Index(output[position:], value)
		if index < 0 {
			t.Fatalf("output %q does not contain %q after byte %d", output, value, position)
		}
		position += index + len(value)
	}
}
