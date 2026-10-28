package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	compozydaemon "github.com/compozy/compozy/internal/daemon"
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
			record.Runtime.CurrentVersion != "v1.0.0" || record.Runtime.LatestVersion != "v1.1.0" ||
			record.Runtime.Managed || record.Runtime.InstallMethod != "direct-binary" {
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
	t.Run("Should refresh the app-owned runtime from the local daemon after verified app install", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name            string
			runtimeVersion  string
			staged          bool
			starting        bool
			unavailable     bool
			stalled         bool
			incompatible    bool
			restarted       bool
			reusedPID       bool
			absent          bool
			corruptBaseline bool
			snapshotTimeout bool
			wantErr         string
		}{
			{name: "Should observe the bundled runtime and a replaced daemon", runtimeVersion: "v1.1.0", restarted: true},
			{name: "Should recognize a new daemon start even when the PID is reused", runtimeVersion: "v1.1.0", restarted: true, reusedPID: true},
			{name: "Should preserve a newer compatible runtime", runtimeVersion: "v1.2.0"},
			{name: "Should not claim a restart when no daemon was initially running", runtimeVersion: "v1.1.0", absent: true},
			{name: "Should observe the new runtime when previous daemon discovery is corrupt", runtimeVersion: "v1.1.0", corruptBaseline: true},
			{name: "Should wait for an incompatible stable runtime to be replaced for a beta app", runtimeVersion: "v0.3.0", incompatible: true, wantErr: "cli: bundled runtime did not become ready before the update deadline: context deadline exceeded"},
			{name: "Should wait for the new daemon to report running", runtimeVersion: "v1.1.0", starting: true},
			{name: "Should keep a closed app staged without claiming runtime completion", staged: true},
			{name: "Should report a runtime observation failure after app success", unavailable: true, wantErr: "daemon snapshot failed"},
			{name: "Should report the update deadline when the runtime snapshot request is interrupted", snapshotTimeout: true, wantErr: "cli: bundled runtime did not become ready before the update deadline: context deadline exceeded"},
			{name: "Should not report completion while the old runtime remains active", stalled: true, wantErr: "cli: bundled runtime did not become ready before the update deadline: context deadline exceeded"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				targetVersion, fromVersion := "v1.1.0", "v1.0.0"
				if tc.incompatible {
					targetVersion, fromVersion = "v0.3.0-beta.38", "v0.3.0-beta.37"
				}
				deps := newTestDeps(t, &stubClient{}).withDefaults()
				paths, err := deps.resolveHome()
				if err != nil {
					t.Fatal(err)
				}
				info := compozydaemon.Info{
					PID: os.Getpid(), Port: 2123, StartedAt: mustProcessStartedAt(t),
				}
				previousInfo := info
				if tc.restarted {
					if tc.reusedPID {
						previousInfo.StartedAt = info.StartedAt.Add(-time.Hour)
					} else {
						previousInfo.PID++
					}
				}
				// Process liveness is the unit I/O boundary; discovery and HTTP status remain real.
				deps.processAlive = func(int) bool { return true }
				deps.processMatchesStartTime = func(int, time.Time) bool { return true }
				var requests, statusRequests atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					if request.URL.Path == "/api/status" {
						status := daemonRunningStatus
						version := tc.runtimeVersion
						if version == "" {
							version = targetVersion
						}
						minAppVersion := targetVersion
						if tc.incompatible {
							minAppVersion = "v0.3.0"
						}
						if statusRequests.Add(1) < 3 && tc.starting {
							status = "starting"
						}
						if err := jsonv2.MarshalWrite(writer, StatusRecord{Daemon: DaemonStatus{
							Status: status, Version: version, PID: info.PID, StartedAt: info.StartedAt,
							Socket: paths.DaemonSocket, HTTPPort: info.Port, MinAppVersion: minAppVersion,
						}}); err != nil {
							t.Errorf("write daemon status: %v", err)
						}
						return
					}
					if request.Method != http.MethodGet || request.URL.Path != "/api/settings/update" {
						t.Errorf("unexpected daemon request: %s %s", request.Method, request.URL.Path)
						http.NotFound(writer, request)
						return
					}
					attempt := requests.Add(1)
					if tc.snapshotTimeout {
						<-request.Context().Done()
						return
					}
					if tc.unavailable {
						http.Error(writer, "daemon snapshot failed", http.StatusInternalServerError)
						return
					}
					currentVersion := tc.runtimeVersion
					status := contract.SettingsUpdateStatusKind(compozyupdate.StatusUpToDate)
					if attempt == 1 || tc.stalled {
						currentVersion = fromVersion
						status = contract.SettingsUpdateStatusKind(compozyupdate.StatusAvailable)
					}
					if err := jsonv2.MarshalWrite(
						writer,
						SettingsUpdateRecord{Runtime: contract.SettingsUpdateRuntimePayload{
							Status: status, InstallMethod: contract.SettingsUpdateInstallDesktopApp,
							CurrentVersion: currentVersion, LatestVersion: targetVersion,
							Message: "Current daemon runtime.",
						}},
					); err != nil {
						t.Errorf("write daemon snapshot: %v", err)
					}
				}))
				t.Cleanup(server.Close)
				client := &daemonClient{
					target: ClientTarget{
						kind:    clientTargetSSHForward,
						baseURL: server.URL,
					},
					httpClient: server.Client(),
				}
				deps.newClient = func(target ClientTarget) (DaemonClient, error) {
					if target.kind != clientTargetLocal || target.socketPath != paths.DaemonSocket {
						t.Fatalf("runtime observation target = %#v, want this home's local daemon", target)
					}
					return client, nil
				}
				deps.now = func() time.Time { return time.Now().UTC() }
				deps.pollInterval = time.Millisecond
				updateRuntime := newCLIUpdateRuntime(deps, paths)
				base := updateRecord{
					Runtime: compozyupdate.RuntimeTrackState{
						Status:         compozyupdate.StatusAvailable,
						InstallMethod:  string(compozyupdate.InstallMethodDesktopApp),
						CurrentVersion: fromVersion,
						LatestVersion:  targetVersion,
						Message:        "Old runtime snapshot.",
					},
					App: &compozyupdate.AppTrackState{
						Status: compozyupdate.StatusAvailable, Running: !tc.staged,
						CurrentVersion: fromVersion, LatestVersion: targetVersion,
					},
				}
				request := compozyupdate.OperationRequest{
					Targets: []compozyupdate.Target{compozyupdate.TargetApp}, Deadline: time.Now().Add(time.Second),
					App: &compozyupdate.AppOperationState{ToVersion: targetVersion, AttemptID: "attempt-1"},
				}
				if !tc.absent {
					if err := compozydaemon.WriteInfo(paths.DaemonInfo, previousInfo); err != nil {
						t.Fatal(err)
					}
				}
				if tc.corruptBaseline {
					if err := os.WriteFile(paths.DaemonInfo, []byte("{"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				updateRuntime.captureDaemonBeforeAppUpdate(request, base.Runtime)
				if err := compozydaemon.WriteInfo(paths.DaemonInfo, info); err != nil {
					t.Fatal(err)
				}
				archived := &compozyupdate.Operation{App: &compozyupdate.AppOperationState{
					ToVersion: targetVersion, AttemptID: "attempt-1", Phase: compozyupdate.PhaseVerified,
				}}
				if tc.staged {
					archived = nil
				}
				timeout := time.Second
				if tc.stalled || tc.incompatible || tc.snapshotTimeout {
					timeout = 200 * time.Millisecond
				}
				ctx, cancel := context.WithTimeout(t.Context(), timeout)
				defer cancel()
				record, err := updateRuntime.finalizeUpdateRecord(ctx, base, request, nil, archived, nil)
				if tc.staged {
					if err != nil || record.Status != compozyupdate.StatusStaged || record.Runtime != base.Runtime ||
						requests.Load() != 0 {
						t.Fatalf("staged record = %#v, %v, requests = %d", record, err, requests.Load())
					}
					return
				}
				if tc.unavailable || tc.stalled || tc.incompatible || tc.snapshotTimeout {
					assertErrorContains(t, err, tc.wantErr)
					if record.Status != compozyupdate.StatusFailed ||
						record.Runtime.Status != compozyupdate.StatusFailed || record.Runtime.CurrentVersion != fromVersion ||
						record.App.Status != compozyupdate.StatusUpdated {
						t.Fatalf(
							"unconfirmed runtime record = %#v, %v, want explicit runtime failure and verified app",
							record,
							err,
						)
					}
					return
				}
				if err != nil || record.Status != compozyupdate.StatusUpdated ||
					record.Runtime.Status != compozyupdate.StatusUpToDate || record.Runtime.CurrentVersion != tc.runtimeVersion ||
					record.Runtime.LatestVersion != targetVersion || record.Runtime.DaemonRestarted != tc.restarted ||
					record.Runtime.Message != "Current daemon runtime." || record.App.Status != compozyupdate.StatusUpdated ||
					requests.Load() < 2 {
					t.Fatalf(
						"completed record = %#v, %v, requests = %d, want observed runtime %s",
						record,
						err,
						requests.Load(),
						tc.runtimeVersion,
					)
				}
				if tc.starting && statusRequests.Load() < 3 {
					t.Fatalf("daemon status requests = %d, want readiness after starting", statusRequests.Load())
				}
			})
		}
	})

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
				ToVersion: "v1.1.0",
			},
			App: &compozyupdate.AppOperationState{
				ToVersion: "v1.1.0", AttemptID: "attempt-1",
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
		record, err := (&cliUpdateRuntime{}).finalizeUpdateRecord(
			t.Context(),
			base,
			compozyupdate.OperationRequest{App: &compozyupdate.AppOperationState{
				ToVersion: "v1.1.0", AttemptID: "attempt-1",
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

// TestLocalRestartStatusDuringBoot verifies offline journal status and the missing-record remote fallback.
func TestLocalRestartStatusDuringBoot(t *testing.T) {
	t.Parallel()
	t.Run("Should use remote status when the local record is missing", func(t *testing.T) {
		t.Parallel()
		want := SettingsRestartStatusRecord{
			OperationID: "remote-restart",
			Status:      "ready",
			NewPID:      42,
		}
		called := false
		client := localRestartStatusClient{homePaths: mustTestHomePaths(t), remote: &stubClient{
			getSettingsRestartStatusFn: func(_ context.Context, operationID string) (SettingsRestartStatusRecord, error) {
				called = true
				if operationID != want.OperationID {
					t.Fatalf("remote operation ID = %q, want %q", operationID, want.OperationID)
				}
				return want, nil
			},
		}}
		got, err := client.GetSettingsRestartStatus(t.Context(), want.OperationID)
		if err != nil || !called || got != want {
			t.Fatalf("remote restart status = %#v, %v, called = %v", got, err, called)
		}
	})
	for _, phase := range []compozydaemon.RestartStatus{compozydaemon.RestartStatusStarting, compozydaemon.RestartStatusFailed, compozydaemon.RestartStatusReady} {
		t.Run("Should read durable "+string(phase)+" while the API is unavailable", func(t *testing.T) {
			t.Parallel()
			paths := mustTestHomePaths(t)
			if err := os.MkdirAll(paths.RestartsDir, 0o700); err != nil {
				t.Fatal(err)
			}
			record := compozydaemon.RestartOperation{
				OperationID:   "offline-restart",
				Status:        phase,
				OldPID:        1,
				OldStartedAt:  fixedTestNow,
				OldSocketPath: paths.DaemonSocket,
				StartedAt:     fixedTestNow,
				UpdatedAt:     fixedTestNow,
			}
			if phase == compozydaemon.RestartStatusFailed {
				record.FailureReason = "replacement exited"
				record.CompletedAt = new(fixedTestNow)
			}
			if phase == compozydaemon.RestartStatusReady {
				record.NewPID = 2
				record.CompletedAt = new(fixedTestNow)
			}
			data, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(
				filepath.Join(paths.RestartsDir, record.OperationID+".json"),
				data,
				0o600,
			); err != nil {
				t.Fatal(err)
			}
			client := localRestartStatusClient{homePaths: paths, remote: &stubClient{}}
			got, err := client.GetSettingsRestartStatus(t.Context(), record.OperationID)
			if err != nil || string(got.Status) != string(phase) || got.FailureReason != record.FailureReason ||
				got.NewPID != record.NewPID {
				t.Fatalf("local restart status = %#v, %v", got, err)
			}
		})
	}
}

func TestRecoveredUpdateOutcome(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve staged app details from the recovering executor", func(t *testing.T) {
		t.Parallel()
		operation := &compozyupdate.Operation{
			App: &compozyupdate.AppOperationState{
				Phase:     compozyupdate.PhaseStaged,
				ToVersion: "v1.1.0",
				AttemptID: "recovered-attempt",
			},
		}
		base := updateRecord{
			Runtime: compozyupdate.RuntimeTrackState{Status: compozyupdate.StatusUpToDate},
			App: &compozyupdate.AppTrackState{
				CurrentVersion: "v1.0.0",
				Status:         compozyupdate.StatusAvailable,
				Message:        "available",
				LastError:      "old error",
			},
		}
		record, err := recoveredUpdateRecord(base, operation)
		if err != nil || record.Status != compozyupdate.StatusStaged || record.App.LatestVersion != "v1.1.0" ||
			record.App.AttemptID != "recovered-attempt" ||
			record.App.LastError != "" ||
			!strings.Contains(record.App.Message, "next launch") {
			t.Fatalf("staged recovery = %#v, %v", record, err)
		}
	})
	for _, tc := range []struct {
		name    string
		runtime *compozyupdate.RuntimeOperationState
		app     *compozyupdate.AppOperationState
	}{
		{name: "Should report a canceled runtime without recommending restart", runtime: &compozyupdate.RuntimeOperationState{Phase: compozyupdate.PhaseDownloading}},
		{name: "Should report a canceled staged app without claiming staged success", app: &compozyupdate.AppOperationState{Phase: compozyupdate.PhaseStaged}},
		{name: "Should preserve a finalized runtime when the pending app is canceled", runtime: &compozyupdate.RuntimeOperationState{Phase: compozyupdate.PhaseFinalized, ToVersion: "v1.1.0", DaemonRestarted: true}, app: &compozyupdate.AppOperationState{Phase: compozyupdate.PhaseStaged}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			record, err := recoveredUpdateRecord(
				updateRecord{App: &compozyupdate.AppTrackState{}},
				&compozyupdate.Operation{Outcome: "canceled", Runtime: tc.runtime, App: tc.app},
			)
			if err != nil || record.Status != compozyupdate.StatusCanceled ||
				strings.Contains(record.Runtime.Message, "compozy daemon stop") {
				t.Fatalf("canceled recovery = %#v, %v", record, err)
			}
			if tc.runtime != nil && tc.runtime.Phase == compozyupdate.PhaseFinalized &&
				(record.Runtime.Status != compozyupdate.StatusUpdated || record.Runtime.CurrentVersion != "v1.1.0") {
				t.Fatalf("lost completed runtime: %#v", record.Runtime)
			}
			if tc.app != nil && record.App.Status != compozyupdate.StatusCanceled {
				t.Fatalf("app status = %s", record.App.Status)
			}
		})
	}

	for _, tc := range []struct {
		name        string
		phase       compozyupdate.OperationPhase
		restarted   bool
		wantStatus  compozyupdate.Status
		wantMessage string
		wantErr     bool
	}{
		{name: "Should report the successful recovering executor", phase: compozyupdate.PhaseFinalized, restarted: true, wantStatus: compozyupdate.StatusUpdated, wantMessage: "no manual daemon restart"},
		{name: "Should report recovery failure and manual restart", phase: compozyupdate.PhaseFailed, wantStatus: compozyupdate.StatusFailed, wantMessage: "compozy daemon stop && compozy daemon start", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			operation := &compozyupdate.Operation{
				Runtime: &compozyupdate.RuntimeOperationState{
					Phase:           tc.phase,
					ToVersion:       "v0.4.5",
					DaemonRestarted: tc.restarted,
				},
				LastError: "replacement failed readiness",
			}
			record, err := recoveredUpdateRecord(updateRecord{}, operation)
			if (err != nil) != tc.wantErr || record.Status != tc.wantStatus ||
				!strings.Contains(record.Runtime.Message, tc.wantMessage) {
				t.Fatalf("recovered outcome = %#v, %v", record, err)
			}
			if tc.wantErr && !strings.Contains(err.Error(), operation.LastError) {
				t.Fatalf("lost recovery cause: %v", err)
			}
		})
	}
}

func TestFencedUpdateCommand(t *testing.T) {
	t.Parallel()
	t.Run("Should print the persisted recovery failure instead of executor fenced", func(t *testing.T) {
		t.Parallel()
		store := newCLIUpdateOperationStore(t)
		deps := newTestDeps(t, &stubClient{})
		now := time.Now().UTC()
		deps.now = func() time.Time { return now }
		var acquired *compozyupdate.Operation
		deps.newUpdateManager = func(compozyconfig.HomePaths) (updateManager, error) {
			return stubUpdateManager{
				operationStore: store,
				checkAllFn: func(context.Context, compozyupdate.CheckOptions) (compozyupdate.MultiState, *compozyupdate.Release, error) {
					return compozyupdate.MultiState{
						Runtime: compozyupdate.RuntimeTrackState{
							Status:         compozyupdate.StatusAvailable,
							CurrentVersion: "v1.0.0",
							LatestVersion:  "v1.1.0",
						},
					}, nil, nil
				},
				planFn: func(_ context.Context, _ compozyupdate.Actor, targets []compozyupdate.Target, holder compozyupdate.Holder) (compozyupdate.OperationRequest, error) {
					return compozyupdate.OperationRequest{
						RequestedBy: compozyupdate.ActorCLI,
						Targets:     targets,
						Holder:      holder,
						Deadline:    deps.now().Add(time.Hour),
						Runtime: &compozyupdate.RuntimeOperationState{
							FromVersion:   "v1.0.0",
							ToVersion:     "v1.1.0",
							ReleaseTag:    "v1.1.0",
							Asset:         "runtime.tar.gz",
							Digest:        "sha256:runtime",
							InstallMethod: compozyupdate.InstallMethodDirectBinary,
							Phase:         compozyupdate.PhasePending,
						},
					}, nil
				},
				acquireFn: func(ctx context.Context, request compozyupdate.OperationRequest) (*compozyupdate.Operation, error) {
					var err error
					acquired, err = store.Acquire(ctx, request)
					return acquired, err
				},
				resolveFn: func(ctx context.Context, _ string) (*compozyupdate.Release, error) {
					_, err := store.Transition(
						ctx,
						acquired.ID,
						acquired.Holder.ExecutorGeneration,
						acquired.Revision,
						compozyupdate.Transition{
							Kind:      compozyupdate.TransitionPhase,
							Actor:     compozyupdate.ActorDaemon,
							Target:    compozyupdate.TargetRuntime,
							Phase:     compozyupdate.PhaseFailed,
							Percent:   -1,
							LastError: "recovery readiness failed",
						},
					)
					if err != nil {
						return nil, err
					}
					return nil, compozyupdate.ErrExecutorFenced
				},
			}, nil
		}
		stdout, _, err := executeRootCommand(t, deps, "update", "-o", "json")
		if err == nil || !strings.Contains(err.Error(), "recovery readiness failed") ||
			strings.Contains(err.Error(), "executor fenced") {
			t.Fatalf("error = %v, output = %s", err, stdout)
		}
		var record updateRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatal(err)
		}
		if record.Status != compozyupdate.StatusFailed ||
			!strings.Contains(record.Runtime.Message, "compozy daemon stop && compozy daemon start") {
			t.Fatalf("recovered output = %#v", record)
		}
	})
}
