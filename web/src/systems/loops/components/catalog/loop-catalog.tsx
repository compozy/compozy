import { Repeat2 } from "lucide-react";

import { Button, Empty, Eyebrow, type ListingViewMode } from "@agh/ui";

import type { LoopCatalogFilter } from "../../lib/loop-catalog";
import { groupLoopCatalog } from "../../lib/loop-catalog";
import type { LoopCatalogEntry } from "../../types";
import { LoopCatalogCard } from "./loop-catalog-card";
import { LoopCatalogRow } from "./loop-catalog-row";

interface LoopCatalogProps {
  entries: readonly LoopCatalogEntry[];
  view: ListingViewMode;
  hasActiveFilters: boolean;
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
            hasActiveFilters
              ? "Try clearing search or filters."
              : "No Loop definitions are available in this workspace yet."
          }
          icon={Repeat2}
          title={hasActiveFilters ? "No matching loops" : "No loops yet"}
        />
      </div>
    );
  }

  const catalog =
    view === "cards" ? (
      <div
        className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3"
        data-testid="loop-catalog-card-grid"
      >
        {entries.map(entry => (
          <LoopCatalogCard key={entry.name} entry={entry} onRun={onRun} />
        ))}
      </div>
    ) : (
      <div className="flex flex-col gap-5" data-testid="loop-catalog">
        {groups.map(group => (
          <section key={group.kind} data-testid={`loop-group-${group.kind}`}>
            <div className="flex items-center gap-2 px-1 pb-2">
              <Eyebrow className="text-muted">{group.label}</Eyebrow>
              <span className="font-mono text-mono-id tabular-nums text-faint">
                {group.entries.length}
              </span>
            </div>
            <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft">
              {group.entries.map(entry => (
                <LoopCatalogRow key={entry.name} entry={entry} onRun={onRun} />
              ))}
            </div>
          </section>
        ))}
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
