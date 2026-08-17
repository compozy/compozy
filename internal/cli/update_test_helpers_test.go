package cli

import (
	"os"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/procutil"
	compozyupdate "github.com/compozy/compozy/internal/update"
)

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
	return acquireCLIUpdateOperationWithAppDigest(t, store, targets, "sha256:app")
}

func acquireCLIUpdateOperationWithAppDigest(
	t *testing.T,
	store *compozyupdate.OperationStore,
	targets []compozyupdate.Target,
	appDigest string,
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
					Asset: "app.zip", Digest: appDigest,
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
