import { Pill } from "@compozy/ui";

import { cn } from "@/lib/utils";
import { SessionArchivedToggle } from "@/systems/session";

import {
  PALETTE_SESSION_FILTER_LIST,
  type PaletteSessionFilterCounts,
  type PaletteSessionFilterId,
} from "../lib/palette-session-filters";
import { paletteViewGutterClass } from "../lib/palette-view-inset";

export interface OsPaletteSessionChipsProps {
  filterId: PaletteSessionFilterId;
  counts: PaletteSessionFilterCounts | undefined;
  archived: boolean;
  onFilterChange: (filterId: PaletteSessionFilterId) => void;
  onArchivedChange: (archived: boolean) => void;
}

/**
 * State filters and the archive switch for the Sessions view.
 *
 * Each filter is its own pill — a segmented track collapses five labels plus
 * counts into one run-on string. Counts sit beside the label so a chip reading
 * `Finished 0` still renders: it tells the operator the filter is empty before
 * they press it.
 *
 * Breadth is not here: the view follows the topbar globe like every session
 * list (ADR-004).
 */
export function OsPaletteSessionChips({
  filterId,
  counts,
  archived,
  onFilterChange,
  onArchivedChange,
}: OsPaletteSessionChipsProps) {
  return (
    <div
      className={cn("flex items-center gap-2 border-b border-line py-2", paletteViewGutterClass)}
      data-slot="os-palette-chips"
    >
      <div
        role="group"
        aria-label="Session state"
        className="flex min-w-0 flex-1 flex-wrap items-center gap-2.5"
      >
        {PALETTE_SESSION_FILTER_LIST.map(filter => {
          const count = counts?.[filter.id];
          const active = filter.id === filterId;
          return (
            <Pill
              key={filter.id}
              size="md"
              tone="neutral"
              active={active}
              data-testid={`os-palette-session-filter-${filter.id}`}
              render={<button type="button" />}
              aria-label={count === undefined ? filter.label : `${filter.label} ${count}`}
              onClick={() => {
                if (active) return;
                onFilterChange(filter.id);
              }}
            >
              {filter.label}
              <span className="tabular-nums text-faint">{count}</span>
            </Pill>
          );
        })}
      </div>
      <div className="flex shrink-0 items-center gap-1">
        <SessionArchivedToggle
          archived={archived}
          onArchivedChange={onArchivedChange}
          testId="os-palette-session-archived"
        />
      </div>
    </div>
  );
}
