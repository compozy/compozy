package daemon

import (
	"context"
	"fmt"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/store/globaldb"
)

func BenchmarkRunManagerListWorkspaceRuns(b *testing.B) {
	env := newRunManagerTestEnv(b, runManagerTestDeps{})
	ctx := context.Background()
	startedAt := time.Date(2026, 4, 18, 11, 0, 0, 0, time.UTC)
	workspace, err := env.globalDB.Register(ctx, env.workspaceRoot, "bench-workspace")
	if err != nil {
		b.Fatalf("Register(): %v", err)
	}

	for i := 0; i < 100; i++ {
		workflow, err := env.globalDB.PutWorkflow(ctx, globaldb.Workflow{
			WorkspaceID: workspace.ID,
			Slug:        fmt.Sprintf("workflow-%03d", i),
			CreatedAt:   startedAt,
			UpdatedAt:   startedAt,
		})
		if err != nil {
			b.Fatalf("PutWorkflow(%d): %v", i, err)
		}
		if _, err := env.globalDB.PutRun(ctx, globaldb.Run{
			RunID:            fmt.Sprintf("run-%03d", i),
			WorkspaceID:      workflow.WorkspaceID,
			WorkflowID:       &workflow.ID,
			Mode:             runModeTask,
			Status:           runStatusCompleted,
			PresentationMode: defaultPresentationMode,
			StartedAt:        startedAt.Add(time.Duration(i) * time.Second),
			EndedAt:          ptrTime(startedAt.Add(time.Duration(i+1) * time.Second)),
		}); err != nil {
			b.Fatalf("PutRun(%d): %v", i, err)
		}
	}

	query := apicore.RunListQuery{Workspace: env.workspaceRoot, Limit: 100}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runs, err := env.manager.List(ctx, query)
		if err != nil {
			b.Fatalf("List(): %v", err)
		}
		if len(runs) != 100 {
			b.Fatalf("len(runs) = %d, want 100", len(runs))
		}
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
