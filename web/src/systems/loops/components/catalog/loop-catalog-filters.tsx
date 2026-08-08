import { ListFilter } from "lucide-react";

import { Button, FiltersWithSearch, type Filter } from "@compozy/ui";

import type { LoopStatusFilter } from "../../lib/loop-catalog";
import {
  applyLoopFilterChips,
  buildLoopFilterFields,
  loopFiltersToChips,
} from "../../lib/loop-list-filters";

export interface LoopCatalogFiltersProps {
  statusFilter: LoopStatusFilter | null;
  onStatusFilterChange: (next: LoopStatusFilter | null) => void;
}

/**
 * Loop catalog filter chip bar for composition inside ListingToolbar.Filters.
 * Drives the server-side `status` query param (one chip, AND-combined with the search).
 *
 * The option list is the daemon's full status vocabulary rather than the statuses
 * present on the loaded page, so `canceled` stays selectable on a roster that has
 * none — the answer is the truthful empty state, not a missing option.
 */
function LoopCatalogFilters({ statusFilter, onStatusFilterChange }: LoopCatalogFiltersProps) {
  const fields = buildLoopFilterFields();
  const chips = loopFiltersToChips({ status: statusFilter });

  const handleFiltersChange = (next: Filter<string>[]) => {
    applyLoopFilterChips(next, { onStatusChange: onStatusFilterChange });
  };

  return (
    <FiltersWithSearch<string>
      allowMultiple={false}
      fields={fields}
      filters={chips}
      onChange={handleFiltersChange}
      size="sm"
      trigger={
        <Button
          aria-label="Add filter"
          data-testid="loop-catalog-filters-add"
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

export { LoopCatalogFilters };
