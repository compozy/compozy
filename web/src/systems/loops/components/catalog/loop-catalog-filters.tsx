import { ListFilter } from "lucide-react";

import { Button } from "@agh/ui";
import { FiltersWithSearch, type Filter } from "@agh/ui";

import type { LoopKindFilter, LoopStatusFilter } from "../../lib/loop-catalog";
import {
  applyLoopFilterChips,
  buildLoopFilterFields,
  loopFiltersToChips,
} from "../../lib/loop-list-filters";

export interface LoopCatalogFiltersProps {
  categories: readonly string[];
  statuses: readonly LoopStatusFilter[];
  kindFilter: LoopKindFilter;
  categoryFilter: string | null;
  statusFilter: LoopStatusFilter | null;
  onKindFilterChange: (next: LoopKindFilter) => void;
  onCategoryFilterChange: (next: string | null) => void;
  onStatusFilterChange: (next: LoopStatusFilter | null) => void;
}

/**
 * Loops filter chip bar for composition inside ListingToolbar.Filters.
 * Kind / category / status options are data-driven from the catalog (truthful UI).
 */
function LoopCatalogFilters({
  categories,
  statuses,
  kindFilter,
  categoryFilter,
  statusFilter,
  onKindFilterChange,
  onCategoryFilterChange,
  onStatusFilterChange,
}: LoopCatalogFiltersProps) {
  const fields = buildLoopFilterFields(categories, statuses);
  const chips = loopFiltersToChips({
    kind: kindFilter,
    category: categoryFilter,
    status: statusFilter,
  });

  const handleFiltersChange = (next: Filter<string>[]) => {
    applyLoopFilterChips(next, {
      onKindChange: onKindFilterChange,
      onCategoryChange: onCategoryFilterChange,
      onStatusChange: onStatusFilterChange,
    });
  };

  return (
    <FiltersWithSearch<string>
      allowMultiple={false}
      data-testid="loop-catalog-filters"
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
