package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestDaemonLoopCatalogShouldBatchBeforePageHydration(t *testing.T) {
	t.Parallel()

	t.Run("Should isolate profile-placed definitions across two profiles", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
		workspaceRoot := t.TempDir()
		workspaceResolver := loopCatalogWorkspaceResolverForTest(t, "ws-catalog", workspaceRoot, now)
		marketingSpec := loopCatalogSpecWithFile(t, t.TempDir(), "profile-loop", looppkg.SourceMarketplace)
		marketingSpec.Description = "marketing profile"
		marketingSpec.InstalledFromExtension = "acme/release-automation"
		marketingSpec.ExtensionOwner = "release-automation"
		engineeringSpec := loopCatalogSpecWithFile(t, t.TempDir(), "profile-loop", looppkg.SourceMarketplace)
		engineeringSpec.Description = "engineering profile"
		if err := os.WriteFile(
			marketingSpec.FilePath,
			[]byte(testLoopYAML("profile-loop", "marketing profile")),
			0o600,
		); err != nil {
			t.Fatalf("WriteFile(marketing loop) error = %v", err)
		}
		if err := os.WriteFile(
			engineeringSpec.FilePath,
			[]byte(testLoopYAML("profile-loop", "engineering profile")),
			0o600,
		); err != nil {
			t.Fatalf("WriteFile(engineering loop) error = %v", err)
		}
		catalog := newResourceCatalog(looppkg.CloneResourceSpec)
		catalog.Replace(1, []resources.Record[looppkg.ResourceSpec]{
			{
				ID: "marketing-loop", Scope: resources.ResourceScope{
					Kind: resources.ResourceScopeKindProfile, ID: "profile-marketing",
				}, Spec: marketingSpec,
			},
			{
				ID: "engineering-loop", Scope: resources.ResourceScope{
					Kind: resources.ResourceScopeKindProfile, ID: "profile-engineering",
				}, Spec: engineeringSpec,
			},
		})
		service := &daemonLoopAPIService{
			catalog: catalog, catalogRuns: &loopCatalogRunReaderStub{}, workspaceResolver: workspaceResolver,
			profiles: loopProfileNameResolverStub{
				"profile-marketing": "marketing", "profile-engineering": "engineering",
			},
			now: func() time.Time { return now },
		}

		marketing, err := service.ListLoops(ctx, "ws-catalog", looppkg.CatalogQuery{
			ProfileID: "profile-marketing",
		})
		if err != nil {
			t.Fatalf("ListLoops(marketing) error = %v", err)
		}
		engineering, err := service.ListLoops(ctx, "ws-catalog", looppkg.CatalogQuery{
			ProfileID: "profile-engineering",
		})
		if err != nil {
			t.Fatalf("ListLoops(engineering) error = %v", err)
		}
		if got, want := marketing.Loops[0].Description, "marketing profile"; got != want {
			t.Fatalf("marketing description = %q, want %q", got, want)
		}
		if got, want := engineering.Loops[0].Description, "engineering profile"; got != want {
			t.Fatalf("engineering description = %q, want %q", got, want)
		}

		definitionResolver := &daemonLoopDefinitionResolver{
			catalog:  catalog,
			profiles: service.profiles,
			compilerFactory: func(context.Context) *looppkg.Compiler {
				return looppkg.NewCompiler()
			},
		}
		resolvedMarketing, err := definitionResolver.ResolveLoop(
			ctx, "ws-catalog", "profile-marketing", "profile-loop",
		)
		if err != nil {
			t.Fatalf("ResolveLoop(marketing) error = %v", err)
		}
		resolvedEngineering, err := definitionResolver.ResolveLoop(
			ctx, "ws-catalog", "profile-engineering", "profile-loop",
		)
		if err != nil {
			t.Fatalf("ResolveLoop(engineering) error = %v", err)
		}
		if got, want := resolvedMarketing.Definition.Meta.Description, "marketing profile"; got != want {
			t.Fatalf("resolved marketing description = %q, want %q", got, want)
		}
		if got, want := resolvedMarketing.InstalledFromExtension, "acme/release-automation"; got != want {
			t.Fatalf("resolved marketing extension provenance = %q, want %q", got, want)
		}
		if got, want := resolvedMarketing.ExtensionOwner, "release-automation"; got != want {
			t.Fatalf("resolved marketing extension owner = %q, want %q", got, want)
		}
		if got, want := resolvedEngineering.Definition.Meta.Description, "engineering profile"; got != want {
			t.Fatalf("resolved engineering description = %q, want %q", got, want)
		}
	})

	t.Run("Should overlay resources once and open definitions only for returned rows", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
		workspaceRoot := t.TempDir()
		resolver := loopCatalogWorkspaceResolverForTest(t, "ws-catalog", workspaceRoot, now)
		alpha := loopCatalogSpecWithFile(t, t.TempDir(), "alpha", looppkg.SourceUser)
		shadowUser := alpha
		shadowUser.Name = "shadow"
		shadowUser.Source = looppkg.SourceUser
		shadowUser.FilePath = filepath.Join(t.TempDir(), "missing-user-shadow.yaml")
		shadowUser.ContractGoal = "User shadow"
		shadowWorkspace := shadowUser
		shadowWorkspace.Source = looppkg.SourceWorkspace
		shadowWorkspace.ContractGoal = "Workspace shadow"
		shadowWorkspace.FilePath = filepath.Join(t.TempDir(), "missing-workspace-shadow.yaml")
		zeta := shadowWorkspace
		zeta.Name = "zeta"
		zeta.FilePath = filepath.Join(t.TempDir(), "missing-zeta.yaml")

		catalog := newResourceCatalog(looppkg.CloneResourceSpec)
		catalog.Replace(1, []resources.Record[looppkg.ResourceSpec]{
			{
				ID:    "user-alpha",
				Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
				Spec:  alpha,
			},
			{
				ID:    "user-shadow",
				Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
				Spec:  shadowUser,
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
			ProfileID: store.DefaultProfileID,
			Kind:      looppkg.CatalogKindReadOnly,
			Limit:     1,
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
		_, err := service.ListLoops(testutil.Context(t), "ws-unknown", looppkg.CatalogQuery{
			ProfileID: store.DefaultProfileID,
		})
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
		_, err := service.ListLoops(testutil.Context(t), "ws-missing-root", looppkg.CatalogQuery{
			ProfileID: store.DefaultProfileID,
		})
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

type loopProfileNameResolverStub map[string]string

func (s loopProfileNameResolverStub) ProfileName(_ context.Context, profileID string) (string, error) {
	name, ok := s[profileID]
	if !ok {
		return "", errors.New("profile not found")
	}
	return name, nil
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
