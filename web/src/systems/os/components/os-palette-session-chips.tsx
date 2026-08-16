import { Globe } from "lucide-react";

import { Icon, Pill, PillGroup } from "@compozy/ui";

import {
  PALETTE_SESSION_FILTER_LIST,
  type PaletteSessionFilterCounts,
  type PaletteSessionFilterId,
} from "../lib/palette-session-filters";

export interface OsPaletteSessionChipsProps {
  filterId: PaletteSessionFilterId;
  counts: PaletteSessionFilterCounts;
  allWorkspaces: boolean;
  /** True while the daemon has not acknowledged the scope write yet. */
  scopeBusy: boolean;
  onFilterChange: (filterId: PaletteSessionFilterId) => void;
  onAllWorkspacesChange: (allWorkspaces: boolean) => void;
}

/**
 * State filters and breadth for the Sessions view.
 *
 * Counts sit inside the labels rather than in a badge slot so that a chip
 * reading `Finished 0` renders: it tells the operator the filter is empty
 * before they press it.
 *
 * "All workspaces" is the operator's persisted session-list scope — the same
 * setting the sessions sidebar writes and `compozy config get
 * shell.sessions.scope` reports.
 */
export function OsPaletteSessionChips({
  filterId,
  counts,
  allWorkspaces,
  scopeBusy,
  onFilterChange,
  onAllWorkspacesChange,
}: OsPaletteSessionChipsProps) {
  return (
    <div className="flex flex-wrap items-center gap-1.5 px-2 pb-1" data-slot="os-palette-chips">
      <PillGroup
        aria-label="Session state"
        size="sm"
        value={filterId}
        onChange={onFilterChange}
        items={PALETTE_SESSION_FILTER_LIST.map(filter => ({
          value: filter.id,
          testId: `os-palette-session-filter-${filter.id}`,
          label: (
            <>
              {filter.label}
              <span className="tabular-nums text-faint">{counts[filter.id]}</span>
            </>
          ),
        }))}
      />
      <Pill
        size="sm"
        tone="neutral"
        active={allWorkspaces}
        data-testid="os-palette-session-scope"
        render={<button type="button" disabled={scopeBusy} />}
        onClick={() => onAllWorkspacesChange(!allWorkspaces)}
      >
        <Icon as={Globe} size="xs" />
        All workspaces
      </Pill>
    </div>
  );
}
