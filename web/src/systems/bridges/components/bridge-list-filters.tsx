import { ListFilter } from "lucide-react";

import { Button } from "@agh/ui";
import { FiltersWithSearch, type Filter } from "@agh/ui";

import {
  applyBridgeFilterChips,
  buildBridgeFilterFields,
  bridgeFiltersToChips,
  type BridgePlatformFilter,
  type BridgeStatusFilter,
} from "../lib/bridge-list-filters";
import type { BridgeScopeFilter } from "../types";

export interface BridgeListFiltersProps {
  platforms: readonly string[];
  scopeFilter: BridgeScopeFilter;
  platformFilter: BridgePlatformFilter | null;
  statusFilter: BridgeStatusFilter | null;
  statuses?: readonly BridgeStatusFilter[];
  onScopeFilterChange: (next: BridgeScopeFilter) => void;
  onPlatformFilterChange: (next: BridgePlatformFilter | null) => void;
  onStatusFilterChange: (next: BridgeStatusFilter | null) => void;
}

/**
 * Bridge inventory filter chip bar for composition inside ListingToolbar.Filters.
 */
function BridgeListFilters({
  platforms,
  scopeFilter,
  platformFilter,
  statusFilter,
  statuses,
  onScopeFilterChange,
  onPlatformFilterChange,
  onStatusFilterChange,
}: BridgeListFiltersProps) {
  const fields = buildBridgeFilterFields(platforms, statuses);
  const chips = bridgeFiltersToChips({
    platform: platformFilter,
    scope: scopeFilter,
    status: statusFilter,
  });

  const handleFiltersChange = (next: Filter<string>[]) => {
    applyBridgeFilterChips(next, {
      onPlatformChange: onPlatformFilterChange,
      onScopeChange: onScopeFilterChange,
      onStatusChange: onStatusFilterChange,
    });
  };

  return (
    <FiltersWithSearch<string>
      allowMultiple={false}
      data-testid="bridge-list-filters"
      fields={fields}
      filters={chips}
      onChange={handleFiltersChange}
      size="sm"
      trigger={
        <Button
          aria-label="Add filter"
          data-testid="bridge-list-filters-add"
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

export { BridgeListFilters };
