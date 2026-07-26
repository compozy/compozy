import type { ListingViewMode } from "@agh/ui";

import type { AutomationTrigger } from "../types";
import {
  AutomationCatalogShell,
  type AutomationCatalogPagination,
} from "./automation-catalog-shell";
import { AutomationTriggerCard } from "./automation-trigger-card";
import { AutomationTriggerRow } from "./automation-trigger-row";

export interface AutomationTriggersCatalogProps {
  triggers: AutomationTrigger[];
  view: ListingViewMode;
  isLoading: boolean;
  errorMessage: string | null;
  hasActiveFilters: boolean;
  pagination: AutomationCatalogPagination;
  onClearFilters: () => void;
  onCreate: () => void;
}

/** Triggers catalog body: rows or cards with shared empty/loading/error/load-more. */
function AutomationTriggersCatalog({
  triggers,
  view,
  isLoading,
  errorMessage,
  hasActiveFilters,
  pagination,
  onClearFilters,
  onCreate,
}: AutomationTriggersCatalogProps) {
  return (
    <AutomationCatalogShell
      errorMessage={errorMessage}
      hasActiveFilters={hasActiveFilters}
      isLoading={isLoading}
      itemCount={triggers.length}
      kind="triggers"
      onClearFilters={onClearFilters}
      onCreate={onCreate}
      pagination={pagination}
      view={view}
    >
      {triggers.map(trigger =>
        view === "cards" ? (
          <AutomationTriggerCard key={trigger.id} trigger={trigger} />
        ) : (
          <AutomationTriggerRow key={trigger.id} trigger={trigger} />
        )
      )}
    </AutomationCatalogShell>
  );
}

export { AutomationTriggersCatalog };
