package loop_test

import (
	"errors"
	"testing"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/resources"
)

func TestLoopCatalogShouldPageStableEffectiveRows(t *testing.T) {
	t.Parallel()

	t.Run("Should keep read-only rows before workspace rows and allow limit changes", func(t *testing.T) {
		t.Parallel()

		records := []resources.Record[loop.ResourceSpec]{
			catalogRecord("workspace-beta", "beta", loop.SourceWorkspace, "delivery", "Deploy beta"),
			catalogRecord("read-zeta", "zeta", loop.SourceMarketplace, "delivery", "Deploy zeta"),
			catalogRecord("workspace-gamma", "gamma", loop.SourceWorkspace, "watch", "Watch gamma"),
			catalogRecord("read-alpha", "alpha", loop.SourceUser, "delivery", "Deploy alpha"),
		}
		summaries := map[string]loop.CatalogRunSummary{
			"alpha": {LoopName: "alpha", LastRun: catalogLastRun(loop.StatusDone)},
			"zeta":  {LoopName: "zeta", LastRun: catalogLastRun(loop.StatusRunning)},
			"beta":  {LoopName: "beta", LastRun: catalogLastRun(loop.StatusFailed)},
		}

		first, err := loop.BuildCatalogPage(records, summaries, loop.CatalogQuery{
			WorkspaceID: "ws-one",
			Limit:       2,
		})
		if err != nil {
			t.Fatalf("BuildCatalogPage(first) error = %v", err)
		}
		assertCatalogNames(t, first.Items, "alpha", "zeta")
		if first.Total != 4 || !first.HasMore || first.NextCursor == "" || first.Limit != 2 {
			t.Fatalf("first page = %#v, want total 4 and continuation", first)
		}

		second, err := loop.BuildCatalogPage(records, summaries, loop.CatalogQuery{
			WorkspaceID: "ws-one",
			Cursor:      first.NextCursor,
			Limit:       1,
		})
		if err != nil {
			t.Fatalf("BuildCatalogPage(second) error = %v", err)
		}
		assertCatalogNames(t, second.Items, "beta")
		if !second.HasMore || second.NextCursor == "" || second.Limit != 1 {
			t.Fatalf("second page = %#v, want one row and continuation", second)
		}

		mutated := append([]resources.Record[loop.ResourceSpec]{
			catalogRecord("read-aardvark", "aardvark", loop.SourceUser, "delivery", "Deploy first"),
		}, records...)
		mutated = append(mutated,
			catalogRecord("workspace-epsilon", "epsilon", loop.SourceWorkspace, "delivery", "Deploy later"),
		)
		remaining, err := loop.BuildCatalogPage(mutated, summaries, loop.CatalogQuery{
			WorkspaceID: "ws-one",
			Cursor:      first.NextCursor,
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("BuildCatalogPage(mutated) error = %v", err)
		}
		assertCatalogNames(t, remaining.Items, "beta", "epsilon", "gamma")
	})
}

func TestLoopCatalogShouldApplySelfFilteredFacets(t *testing.T) {
	t.Parallel()

	t.Run("Should apply search and every non-owning facet before exact counts", func(t *testing.T) {
		t.Parallel()

		records := []resources.Record[loop.ResourceSpec]{
			catalogRecord("read-delivery", "read-delivery", loop.SourceUser, "delivery", "Deploy safely"),
			catalogRecord(
				"workspace-delivery",
				"workspace-delivery",
				loop.SourceWorkspace,
				"delivery",
				"Deploy safely",
			),
			catalogRecord("workspace-watch", "workspace-watch", loop.SourceWorkspace, "watch", "Deploy safely"),
			catalogRecord("workspace-failed", "workspace-failed", loop.SourceWorkspace, "delivery", "Deploy safely"),
			catalogRecord("workspace-never", "workspace-never", loop.SourceWorkspace, "delivery", "Deploy safely"),
			catalogRecord("workspace-other", "workspace-other", loop.SourceWorkspace, "delivery", "Inspect only"),
		}
		summaries := map[string]loop.CatalogRunSummary{
			"read-delivery":      {LoopName: "read-delivery", LastRun: catalogLastRun(loop.StatusRunning)},
			"workspace-delivery": {LoopName: "workspace-delivery", LastRun: catalogLastRun(loop.StatusRunning)},
			"workspace-watch":    {LoopName: "workspace-watch", LastRun: catalogLastRun(loop.StatusRunning)},
			"workspace-failed":   {LoopName: "workspace-failed", LastRun: catalogLastRun(loop.StatusFailed)},
			"workspace-other":    {LoopName: "workspace-other", LastRun: catalogLastRun(loop.StatusRunning)},
		}

		page, err := loop.BuildCatalogPage(records, summaries, loop.CatalogQuery{
			WorkspaceID: "ws-one",
			Search:      " deploy ",
			Kind:        loop.CatalogKindWorkspace,
			Category:    "delivery",
			Status:      loop.StatusRunning,
		})
		if err != nil {
			t.Fatalf("BuildCatalogPage() error = %v", err)
		}
		assertCatalogNames(t, page.Items, "workspace-delivery")
		if page.Total != 1 {
			t.Fatalf("page.Total = %d, want 1", page.Total)
		}
		if got := page.Facets.Kinds; got[loop.CatalogKindReadOnly] != 1 || got[loop.CatalogKindWorkspace] != 1 {
			t.Fatalf("kind facets = %#v, want read_only=1 workspace=1", got)
		}
		if got := page.Facets.Categories; got["delivery"] != 1 || got["watch"] != 1 {
			t.Fatalf("category facets = %#v, want delivery=1 watch=1", got)
		}
		if got := page.Facets.Statuses; got[loop.StatusRunning] != 1 || got[loop.StatusFailed] != 1 {
			t.Fatalf("status facets = %#v, want running=1 failed=1", got)
		}
	})
}

func TestLoopCatalogShouldRejectInvalidQueriesAndCursors(t *testing.T) {
	t.Parallel()

	t.Run("Should bind cursors to workspace and normalized filters", func(t *testing.T) {
		t.Parallel()

		records := []resources.Record[loop.ResourceSpec]{
			catalogRecord("read-alpha", "alpha", loop.SourceUser, "delivery", "Deploy alpha"),
			catalogRecord("read-beta", "beta", loop.SourceUser, "delivery", "Deploy beta"),
		}
		page, err := loop.BuildCatalogPage(records, nil, loop.CatalogQuery{WorkspaceID: "ws-one", Limit: 1})
		if err != nil {
			t.Fatalf("BuildCatalogPage(first) error = %v", err)
		}
		for _, query := range []loop.CatalogQuery{
			{WorkspaceID: "ws-two", Cursor: page.NextCursor},
			{WorkspaceID: "ws-one", Search: "different", Cursor: page.NextCursor},
			{WorkspaceID: "ws-one", Cursor: page.NextCursor[:len(page.NextCursor)-1]},
		} {
			if _, err := loop.BuildCatalogPage(records, nil, query); !errors.Is(err, loop.ErrCatalogCursorInvalid) {
				t.Fatalf("BuildCatalogPage(%#v) error = %v, want ErrCatalogCursorInvalid", query, err)
			}
		}
	})

	t.Run("Should reject invalid kind status sort and limits", func(t *testing.T) {
		t.Parallel()

		queries := []loop.CatalogQuery{
			{WorkspaceID: "ws-one", Kind: "global"},
			{WorkspaceID: "ws-one", Status: "unknown"},
			{WorkspaceID: "ws-one", Sort: "recent"},
			{WorkspaceID: "ws-one", Limit: -1},
			{WorkspaceID: "ws-one", Limit: loop.MaxCatalogLimit + 1},
		}
		for _, query := range queries {
			if _, err := loop.BuildCatalogPage(nil, nil, query); !errors.Is(err, loop.ErrCatalogQueryInvalid) {
				t.Fatalf("BuildCatalogPage(%#v) error = %v, want ErrCatalogQueryInvalid", query, err)
			}
		}
	})
}

func catalogRecord(
	id string,
	name string,
	source loop.Source,
	category string,
	goal string,
) resources.Record[loop.ResourceSpec] {
	return resources.Record[loop.ResourceSpec]{
		ID: id,
		Spec: loop.ResourceSpec{
			Name:         name,
			Source:       source,
			Catalog:      loop.CatalogResourceSpec{Category: category},
			ContractGoal: goal,
		},
	}
}

func catalogLastRun(status loop.Status) *loop.CatalogRunHead {
	return &loop.CatalogRunHead{Status: status}
}

func assertCatalogNames(t *testing.T, items []loop.CatalogItem, want ...string) {
	t.Helper()
	if len(items) != len(want) {
		t.Fatalf("catalog item count = %d, want %d: %#v", len(items), len(want), items)
	}
	for index := range want {
		if got := items[index].Record.Spec.Name; got != want[index] {
			t.Fatalf("catalog item %d name = %q, want %q", index, got, want[index])
		}
	}
}
