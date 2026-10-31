import { Repeat2 } from "@compozy/ui/icons";

import { Button, Empty, ListGroup, ListGroupHeader, type ListingViewMode } from "@compozy/ui";

import {
  itemBindingSearch,
  useWindowBindingSearch,
  type WorkspaceBadgeIdentity,
} from "@/systems/workspace";

import type { LoopCatalogFilter } from "../../lib/loop-catalog";
import { groupLoopCatalog, loopKindFacetCount } from "../../lib/loop-catalog";
import { loopCatalogEntryKey } from "../../lib/loops-list-query";
import type { LoopCatalogEntry, LoopsListResponse } from "../../types";
import { LoopCatalogCard } from "./loop-catalog-card";
import {
  LoopCatalogProjectErrors,
  type LoopCatalogProjectError,
} from "./loop-catalog-project-errors";
import { LoopCatalogRow } from "./loop-catalog-row";

/** All projects mode: rows name their project and open bound to it. */
export interface LoopCatalogAggregate {
  projectOf: (workspaceId: string | undefined) => WorkspaceBadgeIdentity;
  /** Projects whose definitions could not be resolved; healthy rows still render. */
  errors: readonly LoopCatalogProjectError[];
  /** The batch hit its bound; what shows is the first 2,000 definitions. */
  truncated: boolean;
  onRetry: () => void;
}

interface LoopCatalogProps {
  entries: readonly LoopCatalogEntry[];
  view: ListingViewMode;
  hasActiveFilters: boolean;
  /** Server-owned kind counts; group headers fall back to loaded length. */
  facets?: LoopsListResponse["facets"];
  hasNextPage?: boolean;
  isFetchingNextPage?: boolean;
  errorMessage?: string | null;
  onClearFilters: () => void;
  onLoadMore?: () => void;
  onRun: (entry: LoopCatalogEntry) => void;
  aggregate?: LoopCatalogAggregate;
  /** True-empty description; defaults to the single-project sentence. */
  emptyDescription?: string;
}

const GROUP_PASS_THROUGH: LoopCatalogFilter = { kind: "all", category: null, status: null };

export function LoopCatalog({
  entries,
  view,
  hasActiveFilters,
  facets,
  hasNextPage = false,
  isFetchingNextPage = false,
  errorMessage = null,
  onClearFilters,
  onLoadMore,
  onRun,
  aggregate,
  emptyDescription = "This project has no Loops yet.",
}: LoopCatalogProps) {
  const windowSearch = useWindowBindingSearch();
  const groups = groupLoopCatalog(entries, GROUP_PASS_THROUGH);
  const isEmpty = entries.length === 0;
  // Aggregate rows open bound to their own project; elsewhere links keep the window's binding.
  const linkSearch = (entry: LoopCatalogEntry) =>
    aggregate ? itemBindingSearch(entry.workspace_id) : windowSearch;
  const projectOf = aggregate
    ? (entry: LoopCatalogEntry) => aggregate.projectOf(entry.workspace_id)
    : undefined;
  const projectErrors = aggregate ? (
    <LoopCatalogProjectErrors
      errors={aggregate.errors}
      onRetry={aggregate.onRetry}
      truncated={aggregate.truncated}
    />
  ) : null;

  if (isEmpty) {
    return (
      <div className="flex min-h-0 flex-1 flex-col gap-3">
        {projectErrors}
        <div
          className="flex min-h-0 flex-1 items-center justify-center p-4"
          data-testid="loop-catalog-empty"
        >
          <Empty
            action={
              hasActiveFilters ? (
                <Button
                  data-testid="loop-catalog-clear-filters"
                  onClick={onClearFilters}
                  size="sm"
                  type="button"
                  variant="ghost"
                >
                  Clear filters
                </Button>
              ) : undefined
            }
            className="max-w-sm"
            description={hasActiveFilters ? "Try clearing search or filters." : emptyDescription}
            icon={Repeat2}
            title={hasActiveFilters ? "No matching loops" : "No loops yet"}
          />
        </div>
      </div>
    );
  }

  const catalog = (
    <div className="flex flex-col gap-5" data-testid="loop-catalog">
      {groups.map(group => {
        const count = loopKindFacetCount(group.kind, group.entries.length, facets?.kinds);
        return (
          <section key={group.kind} data-testid={`loop-group-${group.kind}`}>
            {view === "cards" ? (
              <>
                <ListGroupHeader
                  className="border-b-0 bg-transparent px-1 pt-0"
                  count={count}
                  label={group.label}
                />
                <div
                  className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3"
                  data-testid="loop-catalog-card-grid"
                >
                  {group.entries.map(entry => (
                    <LoopCatalogCard
                      key={loopCatalogEntryKey(entry)}
                      entry={entry}
                      linkSearch={linkSearch(entry)}
                      onRun={onRun}
                      project={projectOf?.(entry)}
                    />
                  ))}
                </div>
              </>
            ) : (
              <ListGroup
                className="overflow-hidden rounded-lg bg-card shadow-card"
                count={count}
                label={group.label}
              >
                {group.entries.map(entry => (
                  <LoopCatalogRow
                    key={loopCatalogEntryKey(entry)}
                    entry={entry}
                    linkSearch={linkSearch(entry)}
                    onRun={onRun}
                    project={projectOf?.(entry)}
                  />
                ))}
              </ListGroup>
            )}
          </section>
        );
      })}
    </div>
  );

  return (
    <div className="flex flex-col gap-3">
      {projectErrors}
      {catalog}
      {errorMessage ? (
        <p
          className="text-caption text-destructive"
          data-testid="loop-catalog-page-error"
          role="alert"
        >
          {errorMessage}
        </p>
      ) : null}
      {hasNextPage && onLoadMore ? (
        <div className="flex justify-center">
          <Button
            aria-busy={isFetchingNextPage}
            data-testid="loop-catalog-load-more"
            disabled={isFetchingNextPage}
            onClick={onLoadMore}
            size="sm"
            type="button"
            variant="ghost"
          >
            {isFetchingNextPage ? "Loading more loops…" : "Load more loops"}
          </Button>
        </div>
      ) : null}
    </div>
  );
}
