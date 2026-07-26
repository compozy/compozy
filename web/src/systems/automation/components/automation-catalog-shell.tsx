import { AlertCircle, Clock3, Plus, Zap } from "lucide-react";
import type { ReactNode } from "react";

import { Button, Empty, Skeleton, SkeletonRows, type ListingViewMode } from "@agh/ui";

import type { AutomationKind } from "../types";

/** Load-more state for the catalog shell, grouped to avoid boolean-prop sprawl. */
export interface AutomationCatalogPagination {
  hasNextPage: boolean;
  isFetchingNextPage: boolean;
  onLoadMore?: () => void;
}

export interface AutomationCatalogShellProps {
  kind: AutomationKind;
  view: ListingViewMode;
  itemCount: number;
  isLoading: boolean;
  errorMessage: string | null;
  hasActiveFilters: boolean;
  pagination: AutomationCatalogPagination;
  onClearFilters: () => void;
  onCreate: () => void;
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
  children,
}: AutomationCatalogShellProps) {
  const { hasNextPage, isFetchingNextPage, onLoadMore } = pagination;
  const noun = kind === "jobs" ? "jobs" : "triggers";
  const EmptyIcon = kind === "jobs" ? Clock3 : Zap;
  const isEmpty = itemCount === 0;

  if (isLoading && isEmpty) {
    return <AutomationCatalogSkeleton noun={noun} view={view} />;
  }

  if (errorMessage && isEmpty) {
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

  if (isEmpty) {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center p-4"
        data-testid={`${noun}-list-empty`}
      >
        <Empty
          action={
            hasActiveFilters ? (
              <Button
                data-testid={`${noun}-list-clear-filters`}
                onClick={onClearFilters}
                size="sm"
                type="button"
                variant="ghost"
              >
                Clear filters
              </Button>
            ) : (
              <Button
                data-testid={`${noun}-list-create`}
                onClick={onCreate}
                size="sm"
                type="button"
              >
                <Plus aria-hidden="true" className="size-3" />
                {kind === "jobs" ? "Create job" : "Create trigger"}
              </Button>
            )
          }
          className="max-w-sm"
          description={
            hasActiveFilters
              ? "Try clearing search or filters."
              : kind === "jobs"
                ? "Create your first job to run a configured target on a schedule."
                : "Create your first trigger to react to daemon events and run its target."
          }
          icon={EmptyIcon}
          title={hasActiveFilters ? `No ${noun} match` : `No ${noun} yet`}
        />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      {view === "cards" ? (
        <div
          className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3"
          data-testid={`${noun}-list-card-grid`}
        >
          {children}
        </div>
      ) : (
        <div
          className="overflow-hidden rounded-lg border border-line bg-canvas-soft"
          data-testid={`${noun}-list-rows`}
        >
          {children}
        </div>
      )}

      {errorMessage ? (
        <p
          className="px-1 text-caption text-danger"
          data-testid={`${noun}-list-page-error`}
          role="alert"
        >
          {errorMessage}
        </p>
      ) : null}

      {hasNextPage && onLoadMore ? (
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
      ) : null}
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
