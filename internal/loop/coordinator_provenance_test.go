package loop

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/compozy/compozy/internal/task"
)

func TestCoordinatorRunnerProvenanceParentSessionID(t *testing.T) {
	t.Parallel()

	const workspaceID WorkspaceID = "ws-provenance"
	newRunner := func(runs map[RunID]Run) *CoordinatorRunner {
		return &CoordinatorRunner{
			store:  &coordinatorRunnerLoopStore{runs: runs},
			logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		}
	}

	t.Run("Should prefer the current inline origin", func(t *testing.T) {
		t.Parallel()
		run := Run{
			ID: "looprun-inline", WorkspaceID: workspaceID,
			StartedBy: task.ActorIdentity{Kind: task.ActorKindAgentSession, Ref: "sess-starter"},
			Origin:    &RunOrigin{Kind: RunOriginSession, SessionID: "sess-inline"},
		}
		got, err := newRunner(nil).provenanceParentSessionID(context.Background(), run)
		if err != nil || got != "sess-inline" {
			t.Fatalf("provenanceParentSessionID() = %q, %v, want sess-inline", got, err)
		}
	})

	t.Run("Should use the current agent-session starter for a catalog run", func(t *testing.T) {
		t.Parallel()
		run := Run{
			ID: "looprun-catalog", WorkspaceID: workspaceID,
			StartedBy: task.ActorIdentity{Kind: task.ActorKindAgentSession, Ref: "sess-starter"},
			Origin:    &RunOrigin{Kind: RunOriginCatalog},
		}
		got, err := newRunner(nil).provenanceParentSessionID(context.Background(), run)
		if err != nil || got != "sess-starter" {
			t.Fatalf("provenanceParentSessionID() = %q, %v, want sess-starter", got, err)
		}
	})

	t.Run("Should walk daemon-started child ancestry to the nearest session", func(t *testing.T) {
		t.Parallel()
		root := Run{
			ID: "looprun-root", WorkspaceID: workspaceID,
			StartedBy: task.ActorIdentity{Kind: task.ActorKindAgentSession, Ref: "sess-root"},
			Origin:    &RunOrigin{Kind: RunOriginCatalog},
		}
		parent := Run{
			ID: "looprun-parent", WorkspaceID: workspaceID, ParentLoopRunID: root.ID,
			StartedBy: task.ActorIdentity{Kind: task.ActorKindDaemon, Ref: "loop-runner"},
			Origin:    &RunOrigin{Kind: RunOriginCatalog},
		}
		child := Run{
			ID: "looprun-child", WorkspaceID: workspaceID, ParentLoopRunID: parent.ID,
			StartedBy: task.ActorIdentity{Kind: task.ActorKindDaemon, Ref: "loop-runner"},
			Origin:    &RunOrigin{Kind: RunOriginCatalog},
		}
		got, err := newRunner(map[RunID]Run{root.ID: root, parent.ID: parent}).
			provenanceParentSessionID(context.Background(), child)
		if err != nil || got != "sess-root" {
			t.Fatalf("provenanceParentSessionID() = %q, %v, want sess-root", got, err)
		}
	})

	t.Run("Should return empty when ancestry contains no session", func(t *testing.T) {
		t.Parallel()
		run := Run{
			ID: "looprun-operator", WorkspaceID: workspaceID,
			StartedBy: task.ActorIdentity{Kind: task.ActorKindHuman, Ref: "operator"},
			Origin:    &RunOrigin{Kind: RunOriginCatalog},
		}
		got, err := newRunner(nil).provenanceParentSessionID(context.Background(), run)
		if err != nil || got != "" {
			t.Fatalf("provenanceParentSessionID() = %q, %v, want empty", got, err)
		}
	})

	t.Run("Should degrade to empty when an ancestor row is missing", func(t *testing.T) {
		t.Parallel()
		run := Run{
			ID: "looprun-missing-child", WorkspaceID: workspaceID, ParentLoopRunID: "looprun-missing",
			StartedBy: task.ActorIdentity{Kind: task.ActorKindDaemon, Ref: "loop-runner"},
			Origin:    &RunOrigin{Kind: RunOriginCatalog},
		}
		got, err := newRunner(map[RunID]Run{}).provenanceParentSessionID(context.Background(), run)
		if err != nil || got != "" {
			t.Fatalf("provenanceParentSessionID() = %q, %v, want graceful empty", got, err)
		}
	})

	t.Run("Should reject corrupt cycles", func(t *testing.T) {
		t.Parallel()
		first := Run{
			ID: "looprun-cycle-a", WorkspaceID: workspaceID, ParentLoopRunID: "looprun-cycle-b",
			StartedBy: task.ActorIdentity{Kind: task.ActorKindDaemon, Ref: "loop-runner"},
			Origin:    &RunOrigin{Kind: RunOriginCatalog},
		}
		second := Run{
			ID: "looprun-cycle-b", WorkspaceID: workspaceID, ParentLoopRunID: first.ID,
			StartedBy: task.ActorIdentity{Kind: task.ActorKindDaemon, Ref: "loop-runner"},
			Origin:    &RunOrigin{Kind: RunOriginCatalog},
		}
		_, err := newRunner(map[RunID]Run{first.ID: first, second.ID: second}).
			provenanceParentSessionID(context.Background(), first)
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("provenanceParentSessionID(cycle) error = %v, want %v", err, ErrValidation)
		}
	})

	t.Run("Should reject an ancestor from another workspace", func(t *testing.T) {
		t.Parallel()
		parent := Run{
			ID: "looprun-foreign", WorkspaceID: "ws-foreign",
			StartedBy: task.ActorIdentity{Kind: task.ActorKindAgentSession, Ref: "sess-foreign"},
			Origin:    &RunOrigin{Kind: RunOriginCatalog},
		}
		child := Run{
			ID: "looprun-local", WorkspaceID: workspaceID, ParentLoopRunID: parent.ID,
			StartedBy: task.ActorIdentity{Kind: task.ActorKindDaemon, Ref: "loop-runner"},
			Origin:    &RunOrigin{Kind: RunOriginCatalog},
		}
		_, err := newRunner(map[RunID]Run{parent.ID: parent}).
			provenanceParentSessionID(context.Background(), child)
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("provenanceParentSessionID(foreign) error = %v, want %v", err, ErrValidation)
		}
	})

	t.Run("Should reject a returned ancestor with another ID", func(t *testing.T) {
		t.Parallel()
		child := Run{
			ID: "looprun-local", WorkspaceID: workspaceID, ParentLoopRunID: "looprun-parent",
			StartedBy: task.ActorIdentity{Kind: task.ActorKindDaemon, Ref: "loop-runner"},
			Origin:    &RunOrigin{Kind: RunOriginCatalog},
		}
		runner := newRunner(nil)
		runner.store = &coordinatorRunnerLoopStore{getRun: func(RunID) (Run, error) {
			return Run{ID: "looprun-other", WorkspaceID: workspaceID}, nil
		}}
		_, err := runner.provenanceParentSessionID(context.Background(), child)
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("provenanceParentSessionID(mismatched ID) error = %v, want %v", err, ErrValidation)
		}
	})

	t.Run("Should propagate an ancestor store failure", func(t *testing.T) {
		t.Parallel()
		storeErr := errors.New("store unavailable")
		child := Run{
			ID: "looprun-local", WorkspaceID: workspaceID, ParentLoopRunID: "looprun-parent",
			StartedBy: task.ActorIdentity{Kind: task.ActorKindDaemon, Ref: "loop-runner"},
			Origin:    &RunOrigin{Kind: RunOriginCatalog},
		}
		runner := newRunner(nil)
		runner.store = &coordinatorRunnerLoopStore{getRun: func(RunID) (Run, error) {
			return Run{}, storeErr
		}}
		_, err := runner.provenanceParentSessionID(context.Background(), child)
		if !errors.Is(err, storeErr) {
			t.Fatalf("provenanceParentSessionID(store failure) error = %v, want %v", err, storeErr)
		}
	})

	t.Run("Should reject ancestry beyond the canonical depth", func(t *testing.T) {
		t.Parallel()
		runs := make(map[RunID]Run, LoopMaxAncestryDepth+1)
		for index := 0; index <= LoopMaxAncestryDepth; index++ {
			id := RunID(fmt.Sprintf("looprun-depth-%d", index))
			parentID := RunID("")
			if index < LoopMaxAncestryDepth {
				parentID = RunID(fmt.Sprintf("looprun-depth-%d", index+1))
			}
			runs[id] = Run{
				ID: id, WorkspaceID: workspaceID, ParentLoopRunID: parentID,
				StartedBy: task.ActorIdentity{Kind: task.ActorKindDaemon, Ref: "loop-runner"},
				Origin:    &RunOrigin{Kind: RunOriginCatalog},
			}
		}
		_, err := newRunner(runs).provenanceParentSessionID(context.Background(), runs["looprun-depth-0"])
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("provenanceParentSessionID(depth) error = %v, want %v", err, ErrValidation)
		}
	})
}
