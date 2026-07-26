import { ListFilter } from "lucide-react";

import { Button, FiltersWithSearch, type Filter } from "@agh/ui";

import {
  applyAutomationFilterChips,
  automationFiltersToChips,
  buildAutomationFilterFields,
  type AutomationFilterState,
} from "../lib/automation-list-filters";
import type { AutomationKind, AutomationScopeFilter, AutomationSource } from "../types";

export interface AutomationListFiltersProps {
  kind: AutomationKind;
  scopeFilter: AutomationScopeFilter;
  sourceFilter: AutomationSource | null;
  enabledFilter: boolean | null;
  eventFilter?: string | null;
  onScopeFilterChange: (next: AutomationScopeFilter | null) => void;
  onSourceFilterChange: (next: AutomationSource | null) => void;
  onEnabledFilterChange: (next: boolean | null) => void;
  onEventFilterChange?: (next: string | null) => void;
}

/**
 * Jobs/Triggers filter chip bar for composition inside ListingToolbar.Filters.
 * Scope, source, and enabled state are shared; triggers additionally filter by event.
 */
function AutomationListFilters({
  kind,
  scopeFilter,
  sourceFilter,
  enabledFilter,
  eventFilter = null,
  onScopeFilterChange,
  onSourceFilterChange,
  onEnabledFilterChange,
  onEventFilterChange,
}: AutomationListFiltersProps) {
  const fields = buildAutomationFilterFields(kind);
  const state: AutomationFilterState = {
    scope: scopeFilter,
    source: sourceFilter,
    enabled: enabledFilter,
    event: eventFilter,
  };
  const chips = automationFiltersToChips(state);

  const handleFiltersChange = (next: Filter<string>[]) => {
    applyAutomationFilterChips(next, {
      onScopeChange: onScopeFilterChange,
      onSourceChange: onSourceFilterChange,
      onEnabledChange: onEnabledFilterChange,
      onEventChange: onEventFilterChange ?? (() => undefined),
    });
  };

  return (
    <FiltersWithSearch<string>
      allowMultiple={false}
      data-testid={`${kind}-list-filters`}
      fields={fields}
      filters={chips}
      onChange={handleFiltersChange}
      size="sm"
      trigger={
        <Button
          aria-label="Add filter"
          data-testid={`${kind}-list-filters-add`}
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

export { AutomationListFilters };
