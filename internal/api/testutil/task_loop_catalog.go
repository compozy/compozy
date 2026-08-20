package testutil

import (
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	taskpkg "github.com/compozy/compozy/internal/task"
)

const (
	TaskLoopParityRunID         = "looprun-transport-parity"
	TaskLoopParityName          = "transport-parity"
	TaskLoopParityCoordinatorID = "task-loop-transport-coordinator"
	TaskLoopParityCellID        = "task-loop-transport-cell"
	TaskLoopParityGeneration    = 2
	TaskLoopParityNodeID        = "review"
	TaskLoopParityItemIndex     = 0
)

// AssertLoopRunTaskCatalog checks the shared task-list result used by every public transport.
func AssertLoopRunTaskCatalog(t testing.TB, response contract.TasksResponse) {
	t.Helper()

	if len(response.Tasks) != 2 {
		t.Fatalf("Loop run tasks = %#v, want coordinator and one logical cell", response.Tasks)
	}
	byID := make(map[string]contract.TaskCatalogItemPayload, len(response.Tasks))
	for _, item := range response.Tasks {
		if _, duplicate := byID[item.ID]; duplicate {
			t.Fatalf("Loop run tasks contain duplicate id %q: %#v", item.ID, response.Tasks)
		}
		byID[item.ID] = item
	}
	AssertLoopTaskProvenance(t, byID[TaskLoopParityCoordinatorID], contract.LoopProvenanceRoleCoordinator)
	AssertLoopTaskProvenance(t, byID[TaskLoopParityCellID], contract.LoopProvenanceRoleCell)
	if response.Page.Total != 2 || response.Page.HasMore || response.Page.NextCursor != "" {
		t.Fatalf("Loop run page = %#v, want two gap-free logical tasks", response.Page)
	}
	if len(response.Facets.Statuses) != 1 ||
		response.Facets.Statuses[0].Status != taskpkg.TaskStatusCompleted ||
		response.Facets.Statuses[0].Count != 2 {
		t.Fatalf("Loop run status facets = %#v, want completed=2", response.Facets.Statuses)
	}
}

// AssertLoopTaskProvenance checks the complete shared provenance projection for the canonical fixture.
func AssertLoopTaskProvenance(
	t testing.TB,
	item contract.TaskCatalogItemPayload,
	wantRole contract.LoopProvenanceRole,
) {
	t.Helper()

	if item.Loop == nil || item.Loop.RunID != TaskLoopParityRunID ||
		item.Loop.LoopName != TaskLoopParityName || item.Loop.Role != wantRole {
		t.Fatalf("Loop task %q provenance = %#v, want run/name/role", item.ID, item.Loop)
	}
	if wantRole == contract.LoopProvenanceRoleCoordinator {
		if item.ID != TaskLoopParityCoordinatorID || item.Loop.Generation != nil ||
			item.Loop.NodeID != "" || item.Loop.ItemIndex != nil {
			t.Fatalf("Loop coordinator = %#v, want coordinator-only provenance", item)
		}
		return
	}
	if item.ID != TaskLoopParityCellID || item.Loop.Generation == nil ||
		*item.Loop.Generation != TaskLoopParityGeneration || item.Loop.NodeID != TaskLoopParityNodeID ||
		item.Loop.ItemIndex == nil || *item.Loop.ItemIndex != TaskLoopParityItemIndex {
		t.Fatalf("Loop cell = %#v, want complete cell provenance", item)
	}
}
