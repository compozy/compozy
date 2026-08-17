package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	compozyupdate "github.com/compozy/compozy/internal/update"
)

// Invariant: update commands preserve the frozen multi-format and mutation contract.
// Owning layer: CLI update command boundary.
// Canonical suite: TestUpdateCommandContract.
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
		assertOrderedSubstrings(t, human, []string{
			"Status", "available", "Runtime", "0.5.0", "0.5.1", "desktop-app", "App", "running", "Release",
		})
		assertOrderedSubstrings(t, toon, []string{
			"update{status,runtime,app,release}", "available", "0.5.0", "0.5.1", "desktop-app", "running",
		})
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
		assertErrorContains(t, err, "cannot be combined")
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
		if !errors.Is(err, compozyupdate.ErrOperationNotCancelable) {
			t.Fatalf("executeRootCommand() error = %v, want ErrOperationNotCancelable", err)
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

// Invariant: terminal update records report the actual affected track and outcome.
// Owning layer: CLI update projection.
// Canonical suite: TestUpdateTerminalProjection.
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

	t.Run("Should derive planning failures and live blocks from the actual app target", func(t *testing.T) {
		t.Parallel()

		base := updateRecord{
			Runtime: compozyupdate.RuntimeTrackState{Status: compozyupdate.StatusAvailable},
			App:     &compozyupdate.AppTrackState{Status: compozyupdate.StatusAvailable},
		}
		failed := failedUpdateRecord(
			base,
			&compozyupdate.TargetError{Target: compozyupdate.TargetApp, Err: errors.New("app asset is unavailable")},
			compozyupdate.TargetRuntime,
			compozyupdate.TargetApp,
		)
		if failed.Runtime.Status != compozyupdate.StatusAvailable || failed.App == nil ||
			failed.App.Status != compozyupdate.StatusFailed {
			t.Fatalf("targeted failure = %#v, want app-only failure", failed)
		}

		blocked := blockedUpdateRecord(base, &compozyupdate.Operation{
			ActiveTarget: compozyupdate.TargetApp,
			Holder:       &compozyupdate.Holder{PID: 4242},
		}, compozyupdate.TargetRuntime, compozyupdate.TargetApp)
		if blocked.Runtime.Status != compozyupdate.StatusAvailable || blocked.App == nil ||
			blocked.App.Status != compozyupdate.StatusBlocked {
			t.Fatalf("targeted block = %#v, want app-only block", blocked)
		}
	})

	t.Run("Should emit a terminal app failure when completion times out without an archive", func(t *testing.T) {
		t.Parallel()

		base := updateRecord{
			Runtime: compozyupdate.RuntimeTrackState{Status: compozyupdate.StatusUpdated},
			App: &compozyupdate.AppTrackState{
				Status: compozyupdate.StatusAvailable, Running: true,
				CurrentVersion: "v1.0.0", LatestVersion: "v1.1.0",
			},
		}
		record, err := finalizeUpdateRecord(
			base,
			compozyupdate.OperationRequest{App: &compozyupdate.AppOperationState{
				ArtifactIdentity: compozyupdate.ArtifactIdentity{ToVersion: "v1.1.0"}, AttemptID: "attempt-1",
			}},
			nil,
			nil,
			errors.New("desktop app update did not complete before its deadline"),
		)
		assertErrorContains(t, err, "did not complete before its deadline")
		if record.Status != compozyupdate.StatusFailed || record.App == nil ||
			record.App.Status != compozyupdate.StatusFailed || record.Operation != nil {
			t.Fatalf("timeout record = %#v, want terminal app failure", record)
		}
	})

	t.Run("Should wait for a running app and report its archived verified outcome", func(t *testing.T) {
		t.Parallel()

		artifact := []byte("verified app installer")
		digest := sha256.Sum256(artifact)
		store := newCLIUpdateOperationStore(t)
		operation := acquireCLIUpdateOperationWithAppDigest(
			t,
			store,
			[]compozyupdate.Target{compozyupdate.TargetApp},
			"sha256:"+hex.EncodeToString(digest[:]),
		)
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
		if transitionErr != nil {
			t.Fatalf("shell simulator error = %v", transitionErr)
		}
		artifactPath := filepath.Join(t.TempDir(), "app.zip")
		if err := os.WriteFile(artifactPath, artifact, 0o600); err != nil {
			t.Fatalf("WriteFile(app artifact) error = %v", err)
		}
		if err := store.VerifyAppArtifact(
			t.Context(), current.ID, current.Holder.ExecutorGeneration, current.Revision, "attempt-1", artifactPath,
		); err != nil {
			t.Fatalf("VerifyAppArtifact() error = %v", err)
		}
		current, transitionErr = store.Read(t.Context())
		if transitionErr == nil {
			_, transitionErr = store.Transition(
				t.Context(), current.ID, current.Holder.ExecutorGeneration, current.Revision,
				compozyupdate.Transition{
					Kind: compozyupdate.TransitionPhase, Actor: compozyupdate.ActorShell,
					Target: compozyupdate.TargetApp, Phase: compozyupdate.PhaseVerified, Percent: 100,
				},
			)
		}
		if transitionErr != nil {
			t.Fatalf("Transition(verified) error = %v", transitionErr)
		}

		deps := newTestDeps(t, &stubClient{})
		deps.pollInterval = time.Millisecond
		deps.now = func() time.Time { return time.Now().UTC() }
		archived, err := waitForAppUpdateCompletion(
			t.Context(), deps, store, operation.ID, time.Now().UTC().Add(time.Second),
		)
		if err != nil {
			t.Fatalf("waitForAppUpdateCompletion() error = %v", err)
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
