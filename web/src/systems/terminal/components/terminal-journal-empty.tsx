import { ScrollText, SearchX } from "lucide-react";

import { Button, Empty } from "@compozy/ui";

export function TerminalJournalEmpty({
  filtered,
  examinedCount,
  hasMore,
  onLoadMore,
  onClearFilters,
  onOpenTerminal,
}: {
  filtered: boolean;
  /** Rows this query examined. Omit until the host has counted them. */
  examinedCount?: number;
  hasMore: boolean;
  onLoadMore: () => void;
  onClearFilters: () => void;
  onOpenTerminal?: () => void;
}) {
  if (!filtered) {
    return (
      <Empty
        action={
          onOpenTerminal ? (
            <Button
              data-testid="terminal-journal-empty-open"
              onClick={onOpenTerminal}
              size="sm"
              type="button"
            >
              Open a terminal
            </Button>
          ) : undefined
        }
        className="px-6"
        data-testid="terminal-journal-empty"
        description="Commands from every terminal in this project land here — who ran them, where, and how they ended."
        icon={ScrollText}
        title="Nothing has run here yet"
      />
    );
  }
  const title =
    examinedCount === undefined
      ? "No matches in the rows loaded"
      : `No matches in the ${examinedCount} ${examinedCount === 1 ? "row" : "rows"} loaded`;
  return (
    <Empty
      action={
        <>
          {hasMore ? (
            <Button onClick={onLoadMore} size="sm" type="button" variant="outline">
              Load older rows
            </Button>
          ) : null}
          <Button onClick={onClearFilters} size="sm" type="button" variant="ghost">
            Clear filters
          </Button>
        </>
      }
      className="px-6"
      data-testid="terminal-journal-filtered-empty"
      description="Older rows load on demand, so a match may still be further back."
      icon={SearchX}
      title={title}
    />
  );
}
