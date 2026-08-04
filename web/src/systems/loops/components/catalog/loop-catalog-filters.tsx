import { NativeSelect, NativeSelectOption } from "@compozy/ui";

import type { LoopStatusFilter } from "../../lib/loop-catalog";
import { loopStatusFilterOptions } from "../../lib/loop-list-filters";
import { isLoopRunStatus } from "../../lib/loop-formatters";

const ALL_STATUSES = "";

export interface LoopCatalogFiltersProps {
  statusFilter: LoopStatusFilter | null;
  onStatusFilterChange: (next: LoopStatusFilter | null) => void;
}

/**
 * Catalog filter control: one select over the latest-run status of each loop.
 *
 * The option list is the daemon's full status vocabulary rather than the statuses
 * present on the loaded page, so `canceled` stays selectable on a roster that has
 * none — the answer is the truthful empty state, not a missing option.
 */
function LoopCatalogFilters({ statusFilter, onStatusFilterChange }: LoopCatalogFiltersProps) {
  return (
    <NativeSelect
      aria-label="Filter by latest run status"
      data-testid="loop-catalog-status-filter"
      onChange={event => {
        const next = event.target.value;
        onStatusFilterChange(isLoopRunStatus(next) ? next : null);
      }}
      size="sm"
      value={statusFilter ?? ALL_STATUSES}
    >
      <NativeSelectOption value={ALL_STATUSES}>All statuses</NativeSelectOption>
      {loopStatusFilterOptions().map(option => (
        <NativeSelectOption key={option.value} value={option.value}>
          {option.label}
        </NativeSelectOption>
      ))}
    </NativeSelect>
  );
}

export { LoopCatalogFilters };
