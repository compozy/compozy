import { ListFilter, X } from "lucide-react";

import { Button, Eyebrow, ListingToolbar } from "@compozy/ui";

import type { TerminalJournalFilters } from "../types";

export interface TerminalJournalFilterChip {
  key: keyof TerminalJournalFilters;
  label: string;
  value: string;
}

export function TerminalJournalToolbar({
  filters,
  onClearFilter,
  onOpenFilters,
}: {
  filters: readonly TerminalJournalFilterChip[];
  onClearFilter: (key: keyof TerminalJournalFilters) => void;
  onOpenFilters?: () => void;
}) {
  return (
    <div className="flex min-h-11.5 flex-none items-center gap-2.5 border-line border-b px-3.5 py-2.25">
      <ListingToolbar className="min-w-0 flex-1">
        <div className="flex min-w-0 flex-wrap items-center gap-1.5">
          {filters.map(filter => (
            <span
              className="inline-flex items-center gap-1.5 rounded-md bg-badge-fill px-2 py-1 text-eyebrow text-muted"
              data-testid={`terminal-journal-filter-${filter.key}`}
              key={filter.key}
            >
              <span className="text-subtle">{filter.label}</span>
              <span className="text-faint">is</span>
              <span className="text-fg">{filter.value}</span>
              <button
                aria-label={`Clear ${filter.label} filter`}
                onClick={() => onClearFilter(filter.key)}
                type="button"
              >
                <X aria-hidden="true" className="size-2.5" />
              </button>
            </span>
          ))}
          {onOpenFilters ? (
            <Button onClick={onOpenFilters} size="sm" type="button" variant="ghost">
              <ListFilter aria-hidden="true" className="size-3" />
              Filter
            </Button>
          ) : null}
        </div>
        <Eyebrow className="ml-auto text-subtle">Newest first</Eyebrow>
      </ListingToolbar>
    </div>
  );
}
