package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/testutil"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestDaemonLoopCatalogShouldBatchBeforePageHydration(t *testing.T) {
	t.Parallel()

	t.Run("Should overlay resources once and open definitions only for returned rows", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
		workspaceRoot := t.TempDir()
		resolver := loopCatalogWorkspaceResolverForTest(t, "ws-catalog", workspaceRoot, now)
		alpha := loopCatalogSpecWithFile(t, t.TempDir(), "alpha", looppkg.SourceUser)
		shadowGlobal := alpha
		shadowGlobal.Name = "shadow"
		shadowGlobal.Source = looppkg.SourceUser
		shadowGlobal.FilePath = filepath.Join(t.TempDir(), "missing-global-shadow.yaml")
		shadowGlobal.ContractGoal = "Global shadow"
		shadowWorkspace := shadowGlobal
		shadowWorkspace.Source = looppkg.SourceWorkspace
		shadowWorkspace.ContractGoal = "Workspace shadow"
		shadowWorkspace.FilePath = filepath.Join(t.TempDir(), "missing-workspace-shadow.yaml")
		zeta := shadowWorkspace
		zeta.Name = "zeta"
		zeta.FilePath = filepath.Join(t.TempDir(), "missing-zeta.yaml")

		catalog := newResourceCatalog(looppkg.CloneResourceSpec)
		catalog.Replace(1, []resources.Record[looppkg.ResourceSpec]{
			{
				ID:    "global-alpha",
				Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
				Spec:  alpha,
			},
			{
				ID:    "global-shadow",
				Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
				Spec:  shadowGlobal,
			},
			{
				ID:    "workspace-shadow",
				Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-catalog"},
				Spec:  shadowWorkspace,
			},
			{
				ID:    "workspace-zeta",
				Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-catalog"},
				Spec:  zeta,
			},
		})
		reader := &loopCatalogRunReaderStub{summaries: map[string]looppkg.CatalogRunSummary{
			"alpha": {
				LoopName: "alpha",
				LastRun: &looppkg.CatalogRunHead{
					ID:        "looprun-alpha-latest",
					LoopName:  "alpha",
					Status:    looppkg.StatusDone,
					CreatedAt: now.Add(-time.Minute),
				},
				Aggregate30d: looppkg.CatalogRunAggregate{Runs: 2, Succeeded: 1, Failed: 1},
			},
		}}
		service := &daemonLoopAPIService{
			catalog:           catalog,
			catalogRuns:       reader,
			workspaceResolver: resolver,
			now:               func() time.Time { return now },
		}

		response, err := service.ListLoops(ctx, "ws-catalog", looppkg.CatalogQuery{
			Kind:  looppkg.CatalogKindReadOnly,
			Limit: 1,
		})
		if err != nil {
			t.Fatalf("ListLoops() error = %v", err)
		}
		if reader.calls != 1 {
			t.Fatalf("catalog reader calls = %d, want 1", reader.calls)
		}
		if got, want := reader.query.LoopNames, []string{"alpha", "shadow", "zeta"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("catalog reader names = %#v, want effective %#v", got, want)
		}
		if len(response.Loops) != 1 || response.Loops[0].Name != "alpha" {
			t.Fatalf("response.Loops = %#v, want only hydrated alpha", response.Loops)
		}
		if response.Page.Total != 1 || response.Page.Limit != 1 || response.Page.HasMore {
			t.Fatalf("response.Page = %#v, want one exact read-only match", response.Page)
		}
		if response.Loops[0].Aggregate30d.Runs != 2 || response.Loops[0].SuccessRate30 != 0.5 {
			t.Fatalf("response aggregate = %#v / %v", response.Loops[0].Aggregate30d, response.Loops[0].SuccessRate30)
		}
		lastRun := response.Loops[0].LastRun
		if lastRun == nil || lastRun.ID != "looprun-alpha-latest" ||
			lastRun.Status != contract.LoopRunStatusDone || !lastRun.CreatedAt.Equal(now.Add(-time.Minute)) {
			t.Fatalf("response last run = %#v, want lean latest-run payload", lastRun)
		}
	})

	t.Run("Should reject an unknown workspace before reading run summaries", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
		resolver := loopCatalogWorkspaceResolverForTest(t, "ws-known", t.TempDir(), now)
		reader := &loopCatalogRunReaderStub{}
		service := &daemonLoopAPIService{
			catalog:           newResourceCatalog(looppkg.CloneResourceSpec),
			catalogRuns:       reader,
			workspaceResolver: resolver,
			now:               func() time.Time { return now },
		}
		_, err := service.ListLoops(testutil.Context(t), "ws-unknown", looppkg.CatalogQuery{})
		if !errors.Is(err, workspacepkg.ErrWorkspaceNotFound) {
			t.Fatalf("ListLoops(unknown) error = %v, want ErrWorkspaceNotFound", err)
		}
		if reader.calls != 0 {
			t.Fatalf("catalog reader calls = %d, want zero before workspace validation", reader.calls)
		}
	})

	t.Run("Should reject a missing workspace root before reading run summaries", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
		missingRoot := filepath.Join(t.TempDir(), "missing")
		resolver := loopCatalogWorkspaceResolverForTest(t, "ws-missing-root", missingRoot, now)
		reader := &loopCatalogRunReaderStub{}
		service := &daemonLoopAPIService{
			catalog:           newResourceCatalog(looppkg.CloneResourceSpec),
			catalogRuns:       reader,
			workspaceResolver: resolver,
			now:               func() time.Time { return now },
		}
		_, err := service.ListLoops(testutil.Context(t), "ws-missing-root", looppkg.CatalogQuery{})
		if !errors.Is(err, workspacepkg.ErrWorkspaceRootMissing) {
			t.Fatalf("ListLoops(missing root) error = %v, want ErrWorkspaceRootMissing", err)
		}
		if reader.calls != 0 {
			t.Fatalf("catalog reader calls = %d, want zero before workspace root validation", reader.calls)
		}
	})
}

type loopCatalogRunReaderStub struct {
	calls     int
	query     looppkg.CatalogRunQuery
	summaries map[string]looppkg.CatalogRunSummary
}

func (s *loopCatalogRunReaderStub) ListLoopCatalogRunSummaries(
	_ context.Context,
	query looppkg.CatalogRunQuery,
) (map[string]looppkg.CatalogRunSummary, error) {
	s.calls++
	s.query = query
	return s.summaries, nil
}

func loopCatalogWorkspaceResolverForTest(
	t *testing.T,
	workspaceID string,
	root string,
	now time.Time,
) *workspacepkg.Resolver {
	t.Helper()
	db := openDaemonTestGlobalDB(t)
	if err := db.InsertWorkspace(testutil.Context(t), workspacepkg.Workspace{
		ID:        workspaceID,
		Name:      workspaceID,
		RootDir:   root,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}
	resolver, err := workspacepkg.NewResolver(db, workspacepkg.WithLogger(discardLogger()))
	if err != nil {
		t.Fatalf("workspace.NewResolver() error = %v", err)
	}
	return resolver
}

func loopCatalogSpecWithFile(
	t *testing.T,
	dir string,
	name string,
	source looppkg.Source,
) looppkg.ResourceSpec {
	t.Helper()
	path := filepath.Join(dir, name+".yaml")
	if err := os.WriteFile(path, []byte(testLoopYAML(name, "catalog")), 0o600); err != nil {
		t.Fatalf("WriteFile(loop definition) error = %v", err)
	}
	spec, _, err := looppkg.ParseResourceFile(path, looppkg.ResourceParseOptions{Source: source})
	if err != nil {
		t.Fatalf("ParseResourceFile() error = %v", err)
	}
	return spec
}
