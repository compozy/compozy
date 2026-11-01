import { ListFilter } from "@compozy/ui/icons";

import { Button, FiltersWithSearch, type Filter, type FilterFieldConfig } from "@compozy/ui";

import {
  applyAutomationFilterChips,
  automationFiltersToChips,
  buildAutomationFilterFields,
  type AutomationFilterHandlers,
  type AutomationFilterState,
} from "../lib/automation-list-filters";

export interface AutomationListFiltersProps {
  state: AutomationFilterState;
  handlers: AutomationFilterHandlers;
  /** The shared Project filter field, offered only under All projects. */
  projectField?: FilterFieldConfig<string> | null;
}

/** Automations filter chip bar for composition inside ListingToolbar.Filters. */
function AutomationListFilters({ state, handlers, projectField }: AutomationListFiltersProps) {
  return (
    <FiltersWithSearch<string>
      allowMultiple={false}
      data-testid="automation-list-filters"
      fields={buildAutomationFilterFields(projectField)}
      filters={automationFiltersToChips(state)}
      onChange={(next: Filter<string>[]) => applyAutomationFilterChips(next, handlers)}
      size="sm"
      trigger={
        <Button
          aria-label="Add filter"
          data-testid="automation-list-filters-add"
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

export { AutomationListFilters };
