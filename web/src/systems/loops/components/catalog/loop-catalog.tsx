import { Repeat2 } from "lucide-react";

import { Button, Empty, ListGroup, ListGroupHeader, type ListingViewMode } from "@compozy/ui";

import type { LoopCatalogFilter } from "../../lib/loop-catalog";
import { groupLoopCatalog, loopKindFacetCount } from "../../lib/loop-catalog";
import type { LoopCatalogEntry, LoopsListResponse } from "../../types";
import { LoopCatalogCard } from "./loop-catalog-card";
import { LoopCatalogRow } from "./loop-catalog-row";

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
}: LoopCatalogProps) {
  const groups = groupLoopCatalog(entries, GROUP_PASS_THROUGH);
  const isEmpty = entries.length === 0;

  if (isEmpty) {
    return (
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
          description={
            hasActiveFilters ? "Try clearing search or filters." : "This project has no Loops yet."
          }
          icon={Repeat2}
          title={hasActiveFilters ? "No matching loops" : "No loops yet"}
        />
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
                    <LoopCatalogCard key={entry.name} entry={entry} onRun={onRun} />
                  ))}
                </div>
              </>
            ) : (
              <ListGroup
                className="overflow-hidden rounded-lg bg-canvas shadow-card"
                count={count}
                label={group.label}
              >
                {group.entries.map(entry => (
                  <LoopCatalogRow key={entry.name} entry={entry} onRun={onRun} />
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
      {catalog}
      {errorMessage ? (
        <p className="text-caption text-danger" data-testid="loop-catalog-page-error" role="alert">
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
