import { AlertCircle, Plus, RefreshCw, Waypoints } from "lucide-react";

import {
  Alert,
  AlertDescription,
  AlertTitle,
  Button,
  Empty,
  ListingPage,
  ListingToolbar,
  useTopbarSlot,
} from "@agh/ui";
import { type BridgesRouteSearch, useBridgesPage } from "./use-bridges-page";
import {
  BridgeCreateDialog,
  BridgeEmptyState,
  BridgeListFilters,
  BridgeListPanel,
} from "@/systems/bridges";

export function BridgesCatalogLocation({ search }: { search: BridgesRouteSearch }) {
  const page = useBridgesPage(search);

  useTopbarSlot({
    glyph: <Waypoints />,
    count: page.totalBridgeCount,
    actions: (
      <div className="flex items-center gap-2" data-testid="bridges-topbar-actions">
        <Button
          data-testid="bridges-refresh"
          onClick={page.handleRefresh}
          size="sm"
          type="button"
          variant="ghost"
        >
          <RefreshCw aria-hidden="true" className="size-3" />
          Refresh
        </Button>
        <Button
          data-testid="create-bridge-btn"
          disabled={!page.canCreateBridge}
          onClick={page.openCreateDialog}
          size="sm"
          type="button"
        >
          <Plus aria-hidden="true" className="size-3" />
          Bridge
        </Button>
      </div>
    ),
    toolbar:
      page.isInitialLoading || page.fatalError ? undefined : (
        <ListingToolbar>
          <ListingToolbar.Leading>
            <ListingToolbar.Search
              aria-label="Search bridges"
              data-testid="bridge-search-input"
              onChange={page.setSearchQuery}
              placeholder="Search bridges"
              value={page.searchQuery}
            />
            <ListingToolbar.Filters>
              <BridgeListFilters
                onPlatformFilterChange={page.setPlatformFilter}
                onScopeFilterChange={page.setScopeFilter}
                onStatusFilterChange={page.setStatusFilter}
                platformFilter={page.platformFilter}
                platforms={page.platforms}
                scopeFilter={page.scopeFilter}
                statusFilter={page.statusFilter}
                statuses={page.statuses}
              />
            </ListingToolbar.Filters>
          </ListingToolbar.Leading>
          <ListingToolbar.Trailing>
            <ListingToolbar.ViewToggle onChange={page.setView} value={page.view} />
          </ListingToolbar.Trailing>
        </ListingToolbar>
      ),
  });

  if (page.fatalError) {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center py-10"
        data-testid="bridges-error"
      >
        <Empty
          className="max-w-md"
          description={page.fatalError.message ?? "Failed to load bridges"}
          icon={AlertCircle}
          title="Unable to load bridges"
        />
      </div>
    );
  }

  if (!page.isInitialLoading && page.totalBridgeCount === 0 && !page.hasActiveFilters) {
    return (
      <>
        <div className="flex min-h-0 flex-1 flex-col overflow-hidden" data-testid="bridges-shell">
          <BridgeEmptyState onCreate={page.openCreateDialog} providers={page.providers} />
        </div>
        <BridgeCreateDialog {...page.createDialogProps} />
      </>
    );
  }

  return (
    <>
      <ListingPage
        banner={
          page.backgroundError ? (
            <div className="border-b border-line px-9 py-3">
              <Alert data-testid="bridges-background-error" variant="warning">
                <AlertCircle aria-hidden="true" className="size-4" />
                <AlertTitle>Showing cached bridges</AlertTitle>
                <AlertDescription>
                  {page.backgroundError.message ??
                    "The latest bridge refresh failed. Existing data remains available."}
                </AlertDescription>
              </Alert>
            </div>
          ) : null
        }
        data-testid="bridges-shell"
      >
        <div data-testid="bridge-list-panel">
          <BridgeListPanel
            bridgeHealth={page.bridgeHealth}
            bridges={page.bridges}
            errorMessage={page.backgroundError?.message}
            emptyState={page.hasActiveFilters ? "filtered" : "default"}
            paginationStatus={
              page.isFetchingNextPage ? "loading" : page.hasNextPage ? "available" : undefined
            }
            onClearFilters={page.clearFilters}
            onLoadMore={() => void page.loadMore()}
            status={page.isInitialLoading ? "loading" : "ready"}
            view={page.view}
          />
        </div>
      </ListingPage>

      <BridgeCreateDialog {...page.createDialogProps} />
    </>
  );
}
