import { ListFilter } from "lucide-react";

import { Button, FiltersWithSearch, type Filter } from "@compozy/ui";

import type { LoopCatalogFilter } from "../../lib/loop-catalog";
import {
  applyLoopFilterChips,
  buildLoopFilterFields,
  loopFiltersToChips,
} from "../../lib/loop-list-filters";

export interface LoopCatalogFiltersProps {
  filter: LoopCatalogFilter;
  categoryOptions: readonly string[];
  onFiltersChange: (next: LoopCatalogFilter) => void;
}

/**
 * Loop catalog filter chip bar for composition inside ListingToolbar.Filters.
 * Drives the server-side kind, category and status params, AND-combined with search.
 *
 * The option list is the daemon's full status vocabulary rather than the statuses
 * present on the loaded page, so `canceled` stays selectable on a roster that has
 * none — the answer is the truthful empty state, not a missing option.
 */
function LoopCatalogFilters({ filter, categoryOptions, onFiltersChange }: LoopCatalogFiltersProps) {
  const fields = buildLoopFilterFields(categoryOptions, filter.category);
  const chips = loopFiltersToChips(filter);

  const handleFiltersChange = (next: Filter<string>[]) => {
    applyLoopFilterChips(next, { onFiltersChange });
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
          <ListFilter aria-hidden="true" />
          Filter
        </Button>
      }
    />
  );
}

export { LoopCatalogFilters };
