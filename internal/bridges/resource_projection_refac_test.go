package bridges_test

import (
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/testutil"
)

func TestResourceProjectionRollbackPlanRefacs(t *testing.T) {
	t.Parallel()

	t.Run("Should count deletions needed to roll back newly created resources", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 6, 9, 0, 0, 0, time.UTC)
		plan, err := bridgepkg.BuildResourceState(
			testutil.Context(t),
			&projectionStore{},
			[]resources.Record[bridgepkg.BridgeInstanceSpec]{{
				ID:        "brg-created",
				Version:   12,
				Scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
				Spec:      resourceSpec("Created Bridge", true),
				CreatedAt: now,
				UpdatedAt: now,
			}},
			func() time.Time { return now },
		)
		if err != nil {
			t.Fatalf("BuildResourceState() error = %v", err)
		}
		if got, want := plan.OperationCount(), 1; got != want {
			t.Fatalf("plan.OperationCount() = %d, want %d", got, want)
		}

		rollback := plan.RollbackPlan()
		if rollback == nil {
			t.Fatal("plan.RollbackPlan() = nil, want rollback plan")
		}
		if got, want := rollback.OperationCount(), 1; got != want {
			t.Fatalf("rollback.OperationCount() = %d, want %d", got, want)
		}
		if next := rollback.NextInstances(); len(next) != 0 {
			t.Fatalf("rollback.NextInstances() = %#v, want no bridge instances", next)
		}
	})
}
