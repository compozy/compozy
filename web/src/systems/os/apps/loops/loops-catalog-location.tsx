import { AlertCircle, Activity, RefreshCw, Repeat2 } from "lucide-react";
import { Link } from "@tanstack/react-router";

import {
  Button,
  Empty,
  ListingPage,
  ListingToolbar,
  SkeletonRows,
  buttonVariants,
  useTopbarSlot,
} from "@agh/ui";
import { type LoopsRouteSearch, useLoopsCatalog } from "./use-loops-catalog";
import { LoopCatalog, LoopCatalogFilters, type LoopStatusFilter } from "@/systems/loops";

export function LoopsCatalogLocation({ search }: { search: LoopsRouteSearch }) {
  const page = useLoopsCatalog(search);
  const loops = page.loopsQuery.loops;
  const loopCount = page.loopsQuery.total;

  const toolbar =
    page.workspaceId === "" || page.loopsQuery.isLoading ? undefined : (
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
              categories={Object.keys(page.loopsQuery.facets?.categories ?? {})}
              categoryFilter={page.filter.category}
              kindFilter={page.filter.kind}
              onCategoryFilterChange={page.setCategoryFilter}
              onKindFilterChange={page.setKindFilter}
              onStatusFilterChange={page.setStatusFilter}
              statusFilter={page.filter.status}
              statuses={Object.keys(page.loopsQuery.facets?.statuses ?? {}) as LoopStatusFilter[]}
            />
          </ListingToolbar.Filters>
        </ListingToolbar.Leading>
        <ListingToolbar.Trailing>
          <ListingToolbar.ViewToggle onChange={page.setView} value={page.view} />
        </ListingToolbar.Trailing>
      </ListingToolbar>
    );

  useTopbarSlot({
    glyph: <Repeat2 />,
    count: page.workspaceId === "" || page.loopsQuery.isLoading ? undefined : loopCount,
    actions:
      page.workspaceId === "" ? undefined : (
        <div className="flex items-center gap-2" data-testid="loops-topbar-actions">
          <Link
            className={buttonVariants({ size: "sm", variant: "ghost" })}
            data-testid="loops-runs-link"
            to="/loop-runs"
          >
            <Activity aria-hidden="true" className="size-3" />
            Runs
          </Link>
          <Button
            data-testid="loops-refresh"
            onClick={page.handleRefresh}
            size="sm"
            type="button"
            variant="ghost"
          >
            <RefreshCw aria-hidden="true" className="size-3" />
            Refresh
          </Button>
        </div>
      ),
    toolbar,
  });

  if (page.workspaceId === "") {
    return (
      <CatalogState
        description="Select a workspace to browse its Loops."
        testId="loops-no-workspace"
        title="No workspace selected"
      />
    );
  }
  if (page.loopsQuery.isLoading) {
    return (
      <div className="min-h-0 flex-1 overflow-hidden p-5" data-testid="loops-loading">
        <SkeletonRows count={6} rowClassName="border-b border-line-soft py-3" />
      </div>
    );
  }
  if (page.loopsQuery.error && loops.length === 0) {
    return (
      <CatalogState
        description={page.loopsQuery.error.message ?? "Failed to load loops"}
        icon={AlertCircle}
        testId="loops-error"
        title="Unable to load loops"
      />
    );
  }

  if (loopCount === 0 && !page.hasActiveFilters) {
    return (
      <CatalogState
        description="No Loop definitions are available in this workspace yet."
        testId="loops-empty"
        title="No loops yet"
      />
    );
  }

  return (
    <ListingPage data-testid="loops-catalog">
      <LoopCatalog
        entries={loops}
        errorMessage={page.loopsQuery.error?.message}
        hasActiveFilters={page.hasActiveFilters}
        hasNextPage={page.loopsQuery.hasNextPage}
        isFetchingNextPage={page.loopsQuery.isFetchingNextPage}
        onClearFilters={page.clearFilters}
        onLoadMore={() => void page.loopsQuery.fetchNextPage()}
        onRun={page.handleRun}
        view={page.view}
      />
    </ListingPage>
  );
}

interface CatalogStateProps {
  title: string;
  description: string;
  testId: string;
  icon?: typeof Repeat2;
}

function CatalogState({ title, description, testId, icon = Repeat2 }: CatalogStateProps) {
  return (
    <div className="flex min-h-0 flex-1 items-center justify-center py-10" data-testid={testId}>
      <Empty className="max-w-md" description={description} icon={icon} title={title} />
    </div>
  );
}
