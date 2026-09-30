import { AlertCircle, Clock3, Zap, type LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { CatalogEmptyState } from "@/components/catalog-empty-state";
import { Button, Empty, Skeleton, SkeletonRows, type ListingViewMode } from "@compozy/ui";

import type { AutomationKind } from "../types";
import { emptyForScope, type ProfileListingScope } from "@/systems/profiles";

/** Load-more state for the catalog shell, grouped to avoid boolean-prop sprawl. */
export interface AutomationCatalogPagination {
  hasNextPage: boolean;
  isFetchingNextPage: boolean;
  onLoadMore?: () => void;
}

export interface AutomationCatalogShellProps {
  /** Names the selected listing scope in the zero-inventory state (US-009.EC-3). */
  profileScope: ProfileListingScope;
  kind: AutomationKind;
  view: ListingViewMode;
  itemCount: number;
  isLoading: boolean;
  errorMessage: string | null;
  hasActiveFilters: boolean;
  pagination: AutomationCatalogPagination;
  onClearFilters: () => void;
  onCreate: () => void;
  /** Zero-inventory extra, such as live job suggestions. Filtered empty never receives it. */
  unfilteredEmptyPanel?: ReactNode;
  children: ReactNode;
}

/**
 * Shared state envelope for the Jobs/Triggers catalogs: loading, error, empty
 * (zero vs filtered), rows/cards wrapper, page-level error, and load-more.
 */
export function AutomationCatalogShell({
  kind,
  view,
  itemCount,
  isLoading,
  errorMessage,
  hasActiveFilters,
  pagination,
  onClearFilters,
  onCreate,
  profileScope,
  unfilteredEmptyPanel,
  children,
}: AutomationCatalogShellProps) {
  const noun = kind === "jobs" ? "jobs" : "triggers";
  const EmptyIcon = kind === "jobs" ? Clock3 : Zap;
  const isEmpty = itemCount === 0;

  if (isLoading && isEmpty) {
    return <AutomationCatalogSkeleton noun={noun} view={view} />;
  }

  if (errorMessage && isEmpty) {
    return <AutomationCatalogError errorMessage={errorMessage} kind={kind} noun={noun} />;
  }

  if (isEmpty && hasActiveFilters) {
    return (
      <AutomationCatalogFilteredEmpty
        icon={EmptyIcon}
        noun={noun}
        onClearFilters={onClearFilters}
      />
    );
  }

  if (isEmpty) {
    return (
      <CatalogEmptyState
        action={
          <Button
            data-testid={`${noun}-list-create`}
            onClick={onCreate}
            size="sm"
            type="button"
            variant="neutral"
          >
            Create from scratch
          </Button>
        }
        data-testid={`${noun}-list-empty`}
        icon={EmptyIcon}
        panel={unfilteredEmptyPanel}
        support={
          kind === "jobs"
            ? "A job runs an agent or a loop on a schedule."
            : "A trigger runs something when an event happens."
        }
        title={emptyForScope(noun, profileScope.scopeLabel)}
      />
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <AutomationCatalogItems noun={noun} view={view}>
        {children}
      </AutomationCatalogItems>

      {errorMessage ? (
        <p
          className="px-1 text-caption text-danger"
          data-testid={`${noun}-list-page-error`}
          role="alert"
        >
          {errorMessage}
        </p>
      ) : null}

      <AutomationCatalogLoadMore noun={noun} pagination={pagination} />
    </div>
  );
}

function AutomationCatalogError({
  errorMessage,
  kind,
  noun,
}: {
  errorMessage: string;
  kind: AutomationKind;
  noun: string;
}) {
  return (
    <div
      className="flex min-h-0 flex-1 items-center justify-center p-4"
      data-testid={`${noun}-list-error`}
    >
      <Empty
        className="max-w-sm"
        description={errorMessage}
        icon={AlertCircle}
        title={kind === "jobs" ? "Unable to load jobs" : "Unable to load triggers"}
      />
    </div>
  );
}

function AutomationCatalogFilteredEmpty({
  icon,
  noun,
  onClearFilters,
}: {
  icon: LucideIcon;
  noun: string;
  onClearFilters: () => void;
}) {
  return (
    <div
      className="flex min-h-0 flex-1 items-center justify-center p-4"
      data-testid={`${noun}-list-empty`}
    >
      <Empty
        action={
          <Button
            data-testid={`${noun}-list-clear-filters`}
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
        icon={icon}
        title={`No ${noun} match`}
      />
    </div>
  );
}

function AutomationCatalogItems({
  children,
  noun,
  view,
}: {
  children: ReactNode;
  noun: string;
  view: ListingViewMode;
}) {
  if (view === "cards") {
    return (
      <div
        className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3"
        data-testid={`${noun}-list-card-grid`}
      >
        {children}
      </div>
    );
  }
  return (
    <div
      className="overflow-hidden rounded-lg bg-canvas shadow-card"
      data-testid={`${noun}-list-rows`}
    >
      {children}
    </div>
  );
}

function AutomationCatalogLoadMore({
  noun,
  pagination,
}: {
  noun: string;
  pagination: AutomationCatalogPagination;
}) {
  const { hasNextPage, isFetchingNextPage, onLoadMore } = pagination;
  if (!hasNextPage || !onLoadMore) return null;
  return (
    <div className="flex justify-center">
      <Button
        aria-busy={isFetchingNextPage}
        data-testid={`${noun}-list-load-more`}
        disabled={isFetchingNextPage}
        onClick={onLoadMore}
        size="sm"
        type="button"
        variant="ghost"
      >
        {isFetchingNextPage ? `Loading more ${noun}…` : `Load more ${noun}`}
      </Button>
    </div>
  );
}

function AutomationCatalogSkeleton({ noun, view }: { noun: string; view: ListingViewMode }) {
  return (
    <div
      aria-busy="true"
      aria-label={`Loading ${noun} as ${view}`}
      data-testid={`${noun}-list-loading`}
      role="status"
    >
      {view === "cards" ? (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {[0, 1, 2, 3, 4, 5].map(index => (
            <Skeleton className="h-36 w-full rounded-lg" key={index} />
          ))}
        </div>
      ) : (
        <SkeletonRows
          className="overflow-hidden rounded-lg border border-line"
          count={5}
          rowClassName="border-b border-line-soft p-3 last:border-b-0"
        >
          <div className="flex items-center gap-3">
            <Skeleton className="size-8 shrink-0" />
            <div className="flex min-w-0 flex-1 flex-col gap-2">
              <Skeleton className="h-3.5 w-2/5" />
              <Skeleton className="h-3 w-3/5" />
            </div>
            <Skeleton className="h-5 w-20" />
          </div>
        </SkeletonRows>
      )}
    </div>
  );
}
