import { ListFilter } from "lucide-react";

import { Button, Filters, ListingToolbar, type Filter } from "@compozy/ui";

import { terminalJournalFilterFields } from "../lib/terminal-journal-filter-fields";

export interface TerminalJournalToolbarProps {
  chips: Filter<string>[];
  onChange: (chips: Filter<string>[]) => void;
}

/**
 * The journal's filter row, built on the shared `Filters` primitive.
 *
 * The primitive owns adding, editing and removing chips; this row only names
 * the fields and states the fixed sort. Cursor paging is newest-first by
 * construction, so the sort is a fact, not a control.
 */
export function TerminalJournalToolbar({ chips, onChange }: TerminalJournalToolbarProps) {
  return (
    <div className="flex min-h-11.5 flex-none items-center gap-2.5 border-line border-b px-3.5 py-2.25">
      <ListingToolbar className="min-w-0 flex-1">
        <Filters<string>
          allowMultiple={false}
          fields={terminalJournalFilterFields()}
          filters={chips}
          onChange={onChange}
          size="sm"
          trigger={
            <Button
              aria-label="Add filter"
              data-testid="terminal-journal-filters-add"
              size="sm"
              type="button"
              variant="ghost"
            >
              <ListFilter aria-hidden="true" className="size-3" />
              Filter
            </Button>
          }
        />
        <span className="ml-auto text-badge text-subtle">Newest first</span>
      </ListingToolbar>
    </div>
  );
}
