package situation

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
)

func TestContextForSessionActiveLeaseMetadataContract(t *testing.T) {
	t.Run("Should include active lease timing metadata in situation context", func(t *testing.T) {
		t.Parallel()

		leaseUntil := fixedTime().Add(10 * time.Minute)
		heartbeatAt := fixedTime().Add(5 * time.Minute)
		run := taskpkg.Run{
			ID:             "run-1",
			TaskID:         "task-1",
			Status:         taskpkg.TaskRunStatusRunning,
			SessionID:      "sess-1",
			ClaimedBy:      &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAgentSession, Ref: "sess-1"},
			ClaimTokenHash: "sha256:claim-token",
			LeaseUntil:     leaseUntil,
			HeartbeatAt:    heartbeatAt,
			RunNetworkState: &taskpkg.RunNetworkState{
				NetworkSpec: situationLiveSpec(t, "ws-1", "coord-structured"),
			},
		}
		service := NewService(Deps{
			Now: fixedNow,
			TaskStore: taskStoreStub{
				tasks: map[string]taskpkg.Task{
					"task-1": {
						ID:          "task-1",
						Identifier:  "AUTO-1",
						WorkspaceID: "ws-1",
						Status:      taskpkg.TaskStatusInProgress,
					},
				},
				runs: []taskpkg.Run{run},
			},
		})

		payload, err := service.ContextForSession(context.Background(), &session.Info{
			ID:          "sess-1",
			AgentName:   "coder",
			Provider:    "codex",
			WorkspaceID: "ws-1",
			Workspace:   "/work/agh",
			Type:        session.SessionTypeUser,
			State:       session.StateActive,
			CreatedAt:   fixedTime(),
			UpdatedAt:   fixedTime(),
		})
		if err != nil {
			t.Fatalf("ContextForSession() error = %v", err)
		}
		if payload.Task.Lease == nil {
			t.Fatal("Task.Lease = nil, want active lease summary")
		}
		if payload.Task.Lease.ClaimTokenHash != run.ClaimTokenHash {
			t.Fatalf("Task.Lease.ClaimTokenHash = %q, want %q", payload.Task.Lease.ClaimTokenHash, run.ClaimTokenHash)
		}
		if payload.Task.Lease.LeaseUntil == nil || !payload.Task.Lease.LeaseUntil.Equal(leaseUntil) {
			t.Fatalf("Task.Lease.LeaseUntil = %#v, want %s", payload.Task.Lease.LeaseUntil, leaseUntil)
		}
		if payload.Task.Lease.HeartbeatAt == nil || !payload.Task.Lease.HeartbeatAt.Equal(heartbeatAt) {
			t.Fatalf("Task.Lease.HeartbeatAt = %#v, want %s", payload.Task.Lease.HeartbeatAt, heartbeatAt)
		}
	})
}
