import { Button } from "@compozy/ui";

export function TerminalJournalFooter({
  loadedCount,
  filterCount,
  emptyMatch,
  hasMore,
  isLoadingMore,
  onLoadMore,
}: {
  /** Rows loaded, or examined on a miss. Omit until the host has counted them. */
  loadedCount?: number;
  filterCount: number;
  /** True when filters are on and this page has no rows. */
  emptyMatch: boolean;
  hasMore: boolean;
  isLoadingMore: boolean;
  onLoadMore: () => void;
}) {
  const filters = filterCount === 1 ? "filter" : "filters";
  const summary =
    loadedCount === undefined
      ? `${filterCount} ${filters}`
      : emptyMatch
        ? `${loadedCount} ${loadedCount === 1 ? "row" : "rows"} loaded · ${filterCount} ${filters}`
        : `${loadedCount} ${loadedCount === 1 ? "row" : "rows"} loaded · newest first`;

  return (
    <div className="flex min-h-11.5 flex-none items-center gap-3 border-line border-t px-3.5 py-2.25">
      <span className="mr-auto text-badge text-subtle" data-testid="terminal-journal-loaded">
        {summary}
      </span>
      {hasMore && !emptyMatch ? (
        <Button
          data-testid="terminal-journal-load-more"
          disabled={isLoadingMore}
          onClick={onLoadMore}
          size="sm"
          type="button"
          variant="ghost"
        >
          Load older rows
        </Button>
      ) : null}
    </div>
  );
}
