package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/procutil"
)

func TestOperationAcquisitionAndFencing(t *testing.T) {
	t.Run("Should publish one complete operation and block a competing acquisition", func(t *testing.T) {
		t.Parallel()

		store, paths, events := newOperationTestStore(t)
		request := operationTestRequest(testOperationNow)
		operation, err := store.Acquire(context.Background(), request)
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}
		if operation.Revision != 1 || operation.Runtime == nil || operation.Runtime.Digest != "sha256:runtime" {
			t.Fatalf("Acquire() operation = %#v, want complete revision 1 runtime identity", operation)
		}

		_, err = store.Acquire(context.Background(), request)
		var blocked *BlockedError
		if !errors.As(err, &blocked) || blocked.Operation.ID != operation.ID {
			t.Fatalf("second Acquire() error = %v, want BlockedError for %q", err, operation.ID)
		}

		data, err := os.ReadFile(paths.UpdateOperationFile)
		if err != nil {
			t.Fatalf("ReadFile(operation) error = %v", err)
		}
		var persisted Operation
		if err := json.Unmarshal(data, &persisted); err != nil {
			t.Fatalf("Unmarshal(operation) error = %v", err)
		}
		if persisted.ID != operation.ID || len(events.snapshot()) != 2 {
			t.Fatalf("persisted/event contract = %#v/%#v, want acquired then blocked", persisted, events.snapshot())
		}
	})

	t.Run("Should reject stale revisions and expired executor generations", func(t *testing.T) {
		t.Parallel()

		store, _, _ := newOperationTestStore(t)
		operation, err := store.Acquire(context.Background(), operationTestRequest(testOperationNow))
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}
		transition := Transition{
			Kind:    TransitionPhase,
			Actor:   ActorCLI,
			Target:  TargetRuntime,
			Phase:   PhaseDownloading,
			Percent: 0,
		}
		_, err = store.Transition(context.Background(), operation.ID, "generation-1", operation.Revision+1, transition)
		if !errors.Is(err, ErrOperationConflict) {
			t.Fatalf("Transition(stale revision) error = %v, want ErrOperationConflict", err)
		}
		_, err = store.Transition(context.Background(), operation.ID, "generation-2", operation.Revision, transition)
		if !errors.Is(err, ErrExecutorFenced) {
			t.Fatalf("Transition(stale generation) error = %v, want ErrExecutorFenced", err)
		}
	})
}

func TestOperationDormancyAndArchive(t *testing.T) {
	t.Run("Should treat dead and reused process identities as expired holders", func(t *testing.T) {
		t.Parallel()

		paths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		store, err := NewOperationStore(paths, nil)
		if err != nil {
			t.Fatalf("NewOperationStore() error = %v", err)
		}
		startedAt, err := procutil.StartedAt(os.Getpid())
		if err != nil {
			t.Fatalf("procutil.StartedAt() error = %v", err)
		}
		live := Holder{PID: os.Getpid(), PIDStartTime: startedAt, LeaseExpiresAt: time.Now().UTC().Add(time.Minute)}
		if !store.HolderLive(&live) {
			t.Fatal("HolderLive(current process) = false, want true")
		}
		dead := live
		dead.PID = 99999999
		if store.HolderLive(&dead) {
			t.Fatal("HolderLive(dead process) = true, want false")
		}
		reused := live
		reused.PIDStartTime = startedAt.Add(-time.Hour)
		if store.HolderLive(&reused) {
			t.Fatal("HolderLive(reused process identity) = true, want false")
		}
	})

	t.Run("Should transfer a live lease to the detached executor without changing its generation", func(t *testing.T) {
		t.Parallel()

		store, _, _ := newOperationTestStore(t)
		operation, err := store.Acquire(context.Background(), operationTestRequest(testOperationNow))
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}
		holder := *operation.Holder
		holder.PID = 7777
		holder.PIDStartTime = testOperationNow.Add(time.Second)
		holder.Surface = ActorDaemon
		holder.LeaseExpiresAt = testOperationNow.Add(2 * time.Hour)
		transferred, err := store.Transition(
			context.Background(), operation.ID, operation.Holder.ExecutorGeneration, operation.Revision,
			Transition{
				Kind: TransitionRenewLease, Actor: ActorDaemon, Target: TargetRuntime,
				Holder: &holder, Percent: operation.Percent,
			},
		)
		if err != nil {
			t.Fatalf("Transition(transfer lease) error = %v", err)
		}
		if transferred.Holder == nil || transferred.Holder.PID != 7777 ||
			transferred.Holder.ExecutorGeneration != operation.Holder.ExecutorGeneration {
			t.Fatalf("transferred holder = %#v, want detached PID with original generation", transferred.Holder)
		}
	})

	t.Run("Should allow exactly one dormant resume and preserve the operation id", func(t *testing.T) {
		t.Parallel()

		store, _, _ := newOperationTestStore(t)
		request := operationTestRequest(testOperationNow)
		request.Targets = []Target{TargetApp}
		request.Runtime = nil
		operation, err := store.Acquire(context.Background(), request)
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}
		staged, err := store.Transition(context.Background(), operation.ID, "generation-1", operation.Revision, Transition{
			Kind: TransitionPhase, Actor: ActorCLI, Target: TargetApp, Phase: PhaseStaged, Percent: 100,
		})
		if err != nil {
			t.Fatalf("Transition(staged) error = %v", err)
		}
		waiting, err := store.Transition(context.Background(), operation.ID, "generation-1", staged.Revision, Transition{
			Kind: TransitionWaitForApp, Actor: ActorCLI, Target: TargetApp, Percent: -1,
		})
		if err != nil {
			t.Fatalf("Transition(waiting) error = %v", err)
		}
		if waiting.Holder != nil || waiting.Waiting != WaitingForApp {
			t.Fatalf("waiting operation = %#v, want dormant waiting-for-app", waiting)
		}

		resumeRequest := request
		resumeRequest.RequestedBy = ActorShell
		resumeRequest.Holder = operationTestHolder("generation-2", testOperationNow.Add(time.Minute))
		resumed, err := store.Acquire(context.Background(), resumeRequest)
		if err != nil {
			t.Fatalf("Acquire(resume) error = %v", err)
		}
		if resumed.ID != operation.ID || resumed.Revision != waiting.Revision+1 || resumed.ActiveTarget != TargetApp {
			t.Fatalf("resumed operation = %#v, want same id and next revision", resumed)
		}
	})

	t.Run("Should cancel only dormant operations and archive the terminal record once", func(t *testing.T) {
		t.Parallel()

		store, paths, _ := newOperationTestStore(t)
		request := operationTestRequest(testOperationNow)
		request.Targets = []Target{TargetApp}
		request.Runtime = nil
		operation, err := store.Acquire(context.Background(), request)
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}
		_, err = store.Transition(context.Background(), operation.ID, "", operation.Revision, Transition{
			Kind: TransitionCancel, Actor: ActorCLI, Target: TargetApp,
		})
		if !errors.Is(err, ErrOperationNotCancelable) {
			t.Fatalf("Transition(cancel live) error = %v, want ErrOperationNotCancelable", err)
		}

		store.now = func() time.Time { return testOperationNow.Add(2 * time.Hour) }
		canceled, err := store.Transition(context.Background(), operation.ID, "", operation.Revision, Transition{
			Kind: TransitionCancel, Actor: ActorCLI, Target: TargetApp,
		})
		if err != nil {
			t.Fatalf("Transition(cancel dormant) error = %v", err)
		}
		if canceled.Outcome != "canceled" {
			t.Fatalf("canceled outcome = %q, want canceled", canceled.Outcome)
		}
		if _, err := os.Stat(paths.UpdateOperationFile); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(live operation) error = %v, want not exist", err)
		}
		history, err := os.ReadFile(paths.UpdateHistoryFile)
		if err != nil {
			t.Fatalf("ReadFile(history) error = %v", err)
		}
		if occurrences := countOperationHistoryRecords(t, history, operation.ID); occurrences != 1 {
			t.Fatalf("history occurrences = %d, want 1", occurrences)
		}
		archived, err := store.ReadArchived(t.Context(), operation.ID)
		if err != nil {
			t.Fatalf("ReadArchived() error = %v", err)
		}
		if archived == nil || archived.ID != operation.ID || archived.Outcome != "canceled" {
			t.Fatalf("ReadArchived() = %#v, want canceled operation", archived)
		}
	})
}

func TestOperationPhaseMatrix(t *testing.T) {
	t.Run("Should reject an app mutation before the runtime track finalizes", func(t *testing.T) {
		t.Parallel()

		store, _, _ := newOperationTestStore(t)
		operation, err := store.Acquire(context.Background(), operationTestRequest(testOperationNow))
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}
		_, err = store.Transition(context.Background(), operation.ID, "generation-1", operation.Revision, Transition{
			Kind: TransitionPhase, Actor: ActorShell, Target: TargetApp, Phase: PhaseStaged, Percent: 100,
		})
		if err == nil {
			t.Fatal("Transition(app before runtime) error = nil, want active-target rejection")
		}
	})

	t.Run("Should refuse dormant cancellation after an irreversible phase starts", func(t *testing.T) {
		t.Parallel()

		store, _, _ := newOperationTestStore(t)
		request := operationTestRequest(testOperationNow)
		request.Targets = []Target{TargetApp}
		request.Runtime = nil
		operation, err := store.Acquire(t.Context(), request)
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}
		for _, phase := range []OperationPhase{PhaseStaged, PhaseApplying, PhaseInstallerHandoff} {
			operation, err = store.Transition(
				t.Context(), operation.ID, operation.Holder.ExecutorGeneration, operation.Revision,
				Transition{
					Kind: TransitionPhase, Actor: ActorShell, Target: TargetApp,
					Phase: phase, Percent: -1,
				},
			)
			if err != nil {
				t.Fatalf("Transition(%s) error = %v", phase, err)
			}
		}
		store.now = func() time.Time { return testOperationNow.Add(2 * time.Hour) }
		_, err = store.Transition(t.Context(), operation.ID, "", operation.Revision, Transition{
			Kind: TransitionCancel, Actor: ActorCLI, Target: TargetApp, Percent: -1,
		})
		if !errors.Is(err, ErrOperationNotCancelable) {
			t.Fatalf("Transition(cancel installer handoff) error = %v, want ErrOperationNotCancelable", err)
		}
	})

	t.Run("Should emit the canonical event for every allowed journaled phase transition", func(t *testing.T) {
		t.Parallel()

		expectedByTargetAndPhase := map[Target]map[OperationPhase]OperationEventName{
			TargetRuntime: {
				PhaseDownloading:    EventDownloadProgress,
				PhaseVerifying:      EventVerifyCompleted,
				PhaseSwapping:       EventSwapCompleted,
				PhaseRestarting:     EventRestartCompleted,
				PhaseHealthChecking: EventHealthCompleted,
				PhaseFinalized:      EventFinalized,
				PhaseRolledBack:     EventRolledBack,
				PhaseFailed:         EventFinalized,
			},
			TargetApp: {
				PhaseStaged:           EventAppStaged,
				PhaseApplying:         EventAppApplying,
				PhaseInstallerHandoff: EventAppInstallerHandoff,
				PhaseRestarted:        EventRestartCompleted,
				PhaseVerified:         EventAppVerified,
				PhaseFailed:           EventAppFailed,
			},
		}

		for target, currentPhases := range allowedPhaseTransitions {
			for current, nextPhases := range currentPhases {
				for _, next := range nextPhases {
					transition := Transition{
						Kind: TransitionPhase, Actor: ActorCLI, Target: target,
						Phase: next, Percent: -1,
					}
					got, err := eventNameForTransition(transition, nil)
					if err != nil {
						t.Fatalf("eventNameForTransition(%q %q -> %q) error = %v", target, current, next, err)
					}
					if want := expectedByTargetAndPhase[target][next]; got != want {
						t.Fatalf("eventNameForTransition(%q %q -> %q) = %q, want %q", target, current, next, got, want)
					}
				}
			}
		}
	})

	t.Run("Should report lease expiry before recovery with complete correlation", func(t *testing.T) {
		t.Parallel()

		store, _, events := newOperationTestStore(t)
		operation, err := store.Acquire(t.Context(), operationTestRequest(testOperationNow))
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}
		store.now = func() time.Time { return testOperationNow.Add(2 * time.Hour) }
		holder := operationTestHolder("generation-2", testOperationNow.Add(2*time.Hour))
		recovered, err := store.Transition(
			t.Context(), operation.ID, "", operation.Revision,
			Transition{
				Kind: TransitionRecover, Actor: ActorDaemon, Target: TargetRuntime,
				Holder: &holder, Percent: operation.Percent, Outcome: "recovered",
			},
		)
		if err != nil {
			t.Fatalf("Transition(recover) error = %v", err)
		}
		got := events.snapshot()
		if len(got) != 3 || got[1].Name != EventLeaseExpired || got[2].Name != EventOperationRecovered {
			t.Fatalf("events = %#v, want acquired, lease-expired, recovered", got)
		}
		for _, event := range got[1:] {
			if event.OperationID != operation.ID || event.Revision < operation.Revision ||
				event.Target != TargetRuntime || event.InstallMethod == "" || event.FromVersion == "" ||
				event.ToVersion == "" || event.Actor != ActorDaemon || event.Outcome == "" {
				t.Fatalf("event correlation = %#v, want complete safe fields", event)
			}
		}
		if recovered.Holder == nil || recovered.Holder.ExecutorGeneration != "generation-2" {
			t.Fatalf("recovered holder = %#v, want generation-2", recovered.Holder)
		}
	})
}

func TestRequestAppApply(t *testing.T) {
	t.Run("Should persist the verified asset identity before returning accepted", func(t *testing.T) {
		t.Parallel()

		store, _, _ := newOperationTestStore(t)
		request := operationTestRequest(testOperationNow)
		result, err := store.RequestAppApply(t.Context(), AppApplyRequest{
			RequestedBy: ActorWeb,
			App:         *request.App,
			Holder:      request.Holder,
			Deadline:    request.Deadline,
		})
		if err != nil {
			t.Fatalf("RequestAppApply() error = %v", err)
		}
		if result.Status != ApplyStatusAccepted || result.OperationID == "" {
			t.Fatalf("RequestAppApply() = %#v, want accepted operation id", result)
		}
		operation, err := store.Read(t.Context())
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if operation == nil || operation.App == nil || operation.App.AttemptID != "attempt-1" ||
			operation.App.Asset != "CompozyOS-1.1.0-arm64.dmg" || operation.App.Digest != "sha256:app" {
			t.Fatalf("persisted app request = %#v, want verified identity and attempt", operation)
		}
	})

	t.Run("Should return the current holder when another operation blocks app acquisition", func(t *testing.T) {
		t.Parallel()

		store, _, _ := newOperationTestStore(t)
		request := operationTestRequest(testOperationNow)
		operation, err := store.Acquire(t.Context(), request)
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}
		result, err := store.RequestAppApply(t.Context(), AppApplyRequest{
			RequestedBy: ActorWeb,
			App:         *request.App,
			Holder:      request.Holder,
			Deadline:    request.Deadline,
		})
		if err != nil {
			t.Fatalf("RequestAppApply(blocked) error = %v", err)
		}
		if result.Status != ApplyStatusBlocked || result.OperationID != operation.ID || result.Holder == nil {
			t.Fatalf("RequestAppApply(blocked) = %#v, want operation and holder", result)
		}
	})
}

func TestAppAttemptDigestBinding(t *testing.T) {
	t.Run("Should journal failure before a mismatched app artifact can install", func(t *testing.T) {
		t.Parallel()

		store, paths, _ := newOperationTestStore(t)
		request := operationTestRequest(testOperationNow)
		request.Targets = []Target{TargetApp}
		request.Runtime = nil
		operation, err := store.Acquire(t.Context(), request)
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}
		artifactPath := filepath.Join(t.TempDir(), "app.dmg")
		if err := os.WriteFile(artifactPath, []byte("different artifact"), 0o600); err != nil {
			t.Fatalf("WriteFile(artifact) error = %v", err)
		}
		err = store.VerifyAppArtifact(
			t.Context(), operation.ID, "generation-1", operation.Revision, "attempt-1", artifactPath,
		)
		if err == nil || !strings.Contains(err.Error(), "digest does not match") {
			t.Fatalf("VerifyAppArtifact() error = %v, want digest mismatch", err)
		}
		if _, err := os.Stat(paths.UpdateOperationFile); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(operation) error = %v, want terminal journal archived", err)
		}
	})

	t.Run("Should accept only the digest and attempt recorded by acquisition", func(t *testing.T) {
		t.Parallel()

		store, _, _ := newOperationTestStore(t)
		artifact := []byte("verified app artifact")
		sum := sha256.Sum256(artifact)
		request := operationTestRequest(testOperationNow)
		request.Targets = []Target{TargetApp}
		request.Runtime = nil
		request.App.Digest = "sha256:" + hex.EncodeToString(sum[:])
		operation, err := store.Acquire(t.Context(), request)
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}
		artifactPath := filepath.Join(t.TempDir(), "app.dmg")
		if err := os.WriteFile(artifactPath, artifact, 0o600); err != nil {
			t.Fatalf("WriteFile(artifact) error = %v", err)
		}
		if err := store.VerifyAppArtifact(
			t.Context(), operation.ID, "generation-1", operation.Revision, "attempt-1", artifactPath,
		); err != nil {
			t.Fatalf("VerifyAppArtifact() error = %v", err)
		}
	})
}

var testOperationNow = time.Date(2026, time.August, 16, 12, 0, 0, 0, time.UTC)

type recordingOperationEvents struct {
	mu     sync.Mutex
	events []OperationEvent
}

func (r *recordingOperationEvents) EmitUpdateEvent(_ context.Context, event OperationEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *recordingOperationEvents) snapshot() []OperationEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]OperationEvent(nil), r.events...)
}

func newOperationTestStore(
	t *testing.T,
) (*OperationStore, compozyconfig.HomePaths, *recordingOperationEvents) {
	t.Helper()
	paths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	events := &recordingOperationEvents{}
	store, err := NewOperationStore(paths, events)
	if err != nil {
		t.Fatalf("NewOperationStore() error = %v", err)
	}
	store.now = func() time.Time { return testOperationNow }
	store.holderLive = func(holder *Holder, now time.Time) bool {
		return holder != nil && holder.LeaseExpiresAt.After(now)
	}
	return store, paths, events
}

func operationTestRequest(now time.Time) OperationRequest {
	return OperationRequest{
		RequestedBy: ActorCLI,
		Targets:     []Target{TargetRuntime, TargetApp},
		Runtime: &RuntimeOperationState{
			ArtifactIdentity: ArtifactIdentity{
				FromVersion: "v1.0.0", ToVersion: "v1.1.0", ReleaseTag: "v1.1.0",
				Asset: "compozy_darwin_arm64.tar.gz", Digest: "sha256:runtime",
			},
			InstallMethod: InstallMethodDirectBinary,
			Phase:         PhasePending,
		},
		App: &AppOperationState{
			ArtifactIdentity: ArtifactIdentity{
				FromVersion: "v1.0.0", ToVersion: "v1.1.0", ReleaseTag: "v1.1.0",
				Asset: "CompozyOS-1.1.0-arm64.dmg", Digest: "sha256:app",
			},
			AttemptID: "attempt-1",
			Phase:     PhasePending,
		},
		Holder:   operationTestHolder("generation-1", now),
		Deadline: now.Add(30 * time.Minute),
	}
}

func operationTestHolder(generation string, now time.Time) Holder {
	return Holder{
		PID: 1234, PIDStartTime: now.Add(-time.Minute), Surface: ActorCLI,
		ExecutorGeneration: generation, LeaseExpiresAt: now.Add(time.Hour),
	}
}

func countOperationHistoryRecords(t *testing.T, data []byte, operationID string) int {
	t.Helper()
	count := 0
	for _, line := range splitNonEmptyLines(string(data)) {
		var record Operation
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("Unmarshal(history record) error = %v", err)
		}
		if record.ID == operationID {
			count++
		}
	}
	return count
}

func splitNonEmptyLines(value string) []string {
	var lines []string
	for _, line := range strings.Split(value, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func TestOperationPathsUseCanonicalHomeLayout(t *testing.T) {
	t.Run("Should keep every update datum host-global under one home", func(t *testing.T) {
		t.Parallel()

		paths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if filepath.Dir(paths.UpdateOperationFile) != paths.HomeDir ||
			filepath.Dir(paths.UpdateHistoryFile) != paths.LogsDir ||
			filepath.Dir(paths.DesktopProvenanceFile) != paths.BinDir {
			t.Fatalf("update paths = %#v, want host-global home/logs/bin ownership", paths)
		}
	})
}
