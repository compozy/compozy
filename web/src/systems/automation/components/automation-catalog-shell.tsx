import { AlertCircle, Zap } from "lucide-react";
import type { ReactNode } from "react";

import { CatalogEmptyState } from "@/components/catalog-empty-state";
import { Button, Empty, type ListingViewMode } from "@compozy/ui";

import { emptyForScope, type ProfileListingScope } from "@/systems/profiles";

import { AUTOMATION_CARDS_CLASS, AUTOMATION_ROWS_CLASS } from "./automation-catalog-classes";
import { AutomationCatalogSkeleton } from "./automation-catalog-skeleton";

/** Load-more state for the catalog shell, grouped to avoid boolean-prop sprawl. */
export interface AutomationCatalogPagination {
  hasNextPage: boolean;
  isFetchingNextPage: boolean;
  isPaused?: boolean;
  onLoadMore?: () => void;
}

export interface AutomationCatalogLoadError {
  message: string;
  onRetry: () => void;
}

export interface AutomationCatalogShellProps {
  /** Names the selected listing scope in the first-run state. */
  profileScope: ProfileListingScope;
  view: ListingViewMode;
  itemCount: number;
  isLoading: boolean;
  /** Both lists failed (or the only requested one): the load error state. */
  loadError: AutomationCatalogLoadError | null;
  /** Any search, facet or Start view: zero rows read as filtered empty. */
  hasActiveFilters: boolean;
  pagination: AutomationCatalogPagination;
  onClearFilters: () => void;
  /** First-run starts ("On a schedule" · "When something happens"). */
  firstRunActions: ReactNode;
  /** First-run extra, such as live suggestions. Filtered empty never receives it. */
  unfilteredEmptyPanel?: ReactNode;
  children: ReactNode;
}

/**
 * State envelope for the Automations listing: loading, load error, first-run
 * vs filtered empty, rows/cards wrapper and load-more.
 */
export function AutomationCatalogShell({
  view,
  itemCount,
  isLoading,
  loadError,
  hasActiveFilters,
  pagination,
  onClearFilters,
  firstRunActions,
  profileScope,
  unfilteredEmptyPanel,
  children,
}: AutomationCatalogShellProps) {
  const isEmpty = itemCount === 0;

  if (isLoading && isEmpty) {
    return <AutomationCatalogSkeleton view={view} />;
  }

  if (loadError && isEmpty) {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center p-4"
        data-testid="automations-list-error"
      >
        <Empty
          action={
            <Button
              data-testid="automations-list-retry"
              onClick={loadError.onRetry}
              size="sm"
              type="button"
              variant="secondary"
            >
              Try again
            </Button>
          }
          className="max-w-sm"
          description={loadError.message}
          icon={AlertCircle}
          title="Unable to load automations"
        />
      </div>
    );
  }

  if (isEmpty && hasActiveFilters) {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center p-4"
        data-testid="automations-list-filtered-empty"
      >
        <Empty
          action={
            <Button
              data-testid="automations-list-clear-filters"
              onClick={onClearFilters}
              size="sm"
              type="button"
              variant="ghost"
            >
              Clear filters
            </Button>
          }
          className="max-w-sm"
          description="Try clearing search or filters."
          icon={Zap}
          title="No automations match"
        />
      </div>
    );
  }

  if (isEmpty) {
    return (
      <CatalogEmptyState
        action={firstRunActions}
        data-testid="automations-list-empty"
        icon={Zap}
        panel={unfilteredEmptyPanel}
        support="An automation runs an agent, a Loop or a task on a schedule, or when something happens."
        title={emptyForScope("automations", profileScope.scopeLabel)}
      />
    );
  }

  return (
    <div className="flex flex-col gap-3">
      {view === "cards" ? (
        <div className={AUTOMATION_CARDS_CLASS} data-testid="automations-list-card-grid">
          {children}
        </div>
      ) : (
        <div className={AUTOMATION_ROWS_CLASS} data-testid="automations-list-rows">
          {children}
        </div>
      )}
      <AutomationCatalogLoadMore pagination={pagination} />
    </div>
  );
}

function AutomationCatalogLoadMore({ pagination }: { pagination: AutomationCatalogPagination }) {
  const { hasNextPage, isFetchingNextPage, isPaused, onLoadMore } = pagination;
  if (!hasNextPage || !onLoadMore) return null;
  return (
    <div className="flex justify-center">
      <Button
        aria-busy={isFetchingNextPage}
        data-testid="automations-list-load-more"
        disabled={isFetchingNextPage || isPaused}
        onClick={onLoadMore}
        size="sm"
        type="button"
        variant="ghost"
      >
        {isPaused
          ? "Waiting for connection…"
          : isFetchingNextPage
            ? "Loading more automations…"
            : "Load more automations"}
      </Button>
    </div>
  );
}
