import { useState } from "react";
import { ListFilter } from "lucide-react";

import { Button, FiltersWithSearch, type Filter } from "@compozy/ui";

import {
  applyLoopRunFilterChips,
  buildLoopRunFilterFields,
  loopRunFiltersToChips,
  type LoopRunFilterHandlers,
  type LoopRunFilterState,
} from "../../lib/loop-list-filters";

export interface LoopRunsFiltersProps extends LoopRunFilterHandlers {
  origin: LoopRunFilterState["origin"];
  originSession: LoopRunFilterState["originSession"];
  outcome: LoopRunFilterState["outcome"];
}

function sameRunFilterState(a: LoopRunFilterState, b: LoopRunFilterState): boolean {
  return a.origin === b.origin && a.originSession === b.originSession && a.outcome === b.outcome;
}

/**
 * Keeps the chip bar truthful when origin and session-id chips coexist: a
 * session id implies the session origin, while explicitly re-picking an origin
 * is an exit from session filtering — the chip the user just touched wins.
 */
function reconcileRunChips(prev: Filter<string>[], next: Filter<string>[]): Filter<string>[] {
  const session = next.find(chip => chip.field === "origin_session");
  const origin = next.find(chip => chip.field === "origin");
  if (!session || !origin || origin.values[0] === "session") return next;
  const prevOriginValue = prev.find(chip => chip.field === "origin")?.values[0];
  if (prevOriginValue !== origin.values[0]) {
    return next.filter(chip => chip.field !== "origin_session");
  }
  return next.map(chip => (chip.field === "origin" ? { ...chip, values: ["session"] } : chip));
}

/**
 * Loop runs filter chip bar for composition inside ListingToolbar.Filters,
 * mirroring the catalog pattern. Origin and session id drive the server query;
 * outcome drives the client-side Active/Past partition.
 *
 * Chips live in local state so a freshly added session-id chip survives while
 * its text is still empty (route state cannot represent that draft); committed
 * filter state is reconciled during render, so URL-driven changes (back or
 * forward, deep links) still reset the bar.
 */
function LoopRunsFilters({
  origin,
  originSession,
  outcome,
  onOriginFilterChange,
  onOutcomeChange,
}: LoopRunsFiltersProps) {
  const committedFromProps: LoopRunFilterState = { origin, originSession, outcome };
  const [chips, setChips] = useState(() => loopRunFiltersToChips(committedFromProps));
  const [committed, setCommitted] = useState(committedFromProps);
  if (!sameRunFilterState(committed, committedFromProps)) {
    setCommitted(committedFromProps);
    setChips(loopRunFiltersToChips(committedFromProps));
  }

  const handleFiltersChange = (next: Filter<string>[]) => {
    const effective = reconcileRunChips(chips, next);
    setChips(effective);
    setCommitted(applyLoopRunFilterChips(effective, { onOriginFilterChange, onOutcomeChange }));
  };

  return (
    <FiltersWithSearch<string>
      allowMultiple={false}
      fields={buildLoopRunFilterFields()}
      filters={chips}
      onChange={handleFiltersChange}
      size="sm"
      trigger={
        <Button
          aria-label="Add filter"
          data-testid="loop-runs-filters-add"
          size="sm"
          type="button"
          variant="ghost"
        >
          <ListFilter aria-hidden="true" className="size-3" />
          Filter
        </Button>
      }
    />
  );
}

export { LoopRunsFilters };
