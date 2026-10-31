import { AlertCircle, Activity, Repeat2 } from "@compozy/ui/icons";
import { Link } from "@tanstack/react-router";

import {
  Button,
  Empty,
  ListingPage,
  ListingToolbar,
  buttonVariants,
  useTopbarSlot,
} from "@compozy/ui";
import { type LoopsRouteSearch, useLoopsCatalog } from "./use-loops-catalog";

import {
  LoopCatalog,
  LoopCatalogFilters,
  LoopCatalogLede,
  LoopCatalogSkeleton,
} from "@/systems/loops";

export function LoopsCatalogLocation({ search }: { search: LoopsRouteSearch }) {
  const page = useLoopsCatalog(search);
  const { catalogQuery } = page;

  const toolbar = !page.ready ? undefined : (
    <ListingToolbar>
      <ListingToolbar.Leading>
        <ListingToolbar.Search
          aria-label="Search loops"
          data-testid="loop-search-input"
          onChange={page.setSearchQuery}
          placeholder="Search loops"
          value={page.searchQuery}
        />
        <ListingToolbar.Filters>
          <LoopCatalogFilters
            categoryOptions={page.categoryOptions}
            filter={page.filter}
            onFiltersChange={page.setFilters}
            projectFilter={page.projectFilter}
          />
        </ListingToolbar.Filters>
      </ListingToolbar.Leading>
      <ListingToolbar.Trailing>
        {page.definitionsNote ? (
          <span className="text-caption text-muted" data-testid="loops-definitions-note">
            {page.definitionsNote}
          </span>
        ) : null}
        <ListingToolbar.ViewToggle onChange={page.setView} value={page.view} />
      </ListingToolbar.Trailing>
    </ListingToolbar>
  );

  useTopbarSlot({
    glyph: <Repeat2 />,
    actions: !page.ready ? undefined : (
      <div className="flex items-center gap-2" data-testid="loops-topbar-actions">
        <Link
          className={buttonVariants({ variant: "secondary", size: "sm" })}
          data-testid="loops-runs-link"
          to="/loop-runs"
        >
          <Activity aria-hidden="true" />
          Runs
        </Link>
      </div>
    ),
    toolbar,
  });

  // The first load stays blank until the window's data scope settles.
  if (!page.ready) {
    return <div className="min-h-0 flex-1" data-testid="loops-no-workspace" aria-busy="true" />;
  }
  // No rows yet and no failure: the catalog's own shape, inside the page it will fill.
  if (catalogQuery.isPending) {
    return (
      <ListingPage data-testid="loops-loading">
        <LoopCatalogSkeleton view={page.view} workspaceLabel={page.workspaceLabel} />
      </ListingPage>
    );
  }
  // One read: a failure with nothing to keep is one error with Retry.
  if (catalogQuery.error && page.entries.length === 0) {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center py-10"
        data-testid="loops-error"
      >
        <Empty
          action={
            <Button onClick={catalogQuery.retry} size="sm" type="button" variant="secondary">
              Retry
            </Button>
          }
          className="max-w-md"
          description={catalogQuery.error.message ?? "Failed to load loops"}
          icon={AlertCircle}
          title="Unable to load loops"
        />
      </div>
    );
  }

  return (
    <ListingPage data-testid="loops-catalog">
      <LoopCatalogLede total={page.total} workspaceLabel={page.workspaceLabel} />
      <LoopCatalog
        aggregate={page.aggregate}
        emptyDescription={
          page.allProjects ? "No project has Loops yet." : "This project has no Loops yet."
        }
        entries={page.entries}
        errorMessage={catalogQuery.error?.message}
        facets={page.facets}
        hasActiveFilters={page.hasActiveFilters}
        hasNextPage={catalogQuery.hasNextPage}
        isFetchingNextPage={catalogQuery.isFetchingNextPage}
        onClearFilters={page.clearFilters}
        onLoadMore={catalogQuery.fetchNextPage}
        onRun={page.handleRun}
        view={page.view}
      />
    </ListingPage>
  );
}
