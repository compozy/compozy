import { ListFilter } from "lucide-react";

import { Button, FiltersWithSearch, type Filter } from "@agh/ui";

import {
  applySandboxFilterChips,
  buildSandboxFilterFields,
  sandboxFiltersToChips,
  type SandboxBackendFilter,
  type SandboxPersistenceFilter,
} from "../lib/sandbox-list-filters";

export interface SandboxListFiltersProps {
  backend: SandboxBackendFilter;
  persistence: SandboxPersistenceFilter;
  onBackendChange: (next: SandboxBackendFilter) => void;
  onPersistenceChange: (next: SandboxPersistenceFilter) => void;
}

export function SandboxListFilters({
  backend,
  persistence,
  onBackendChange,
  onPersistenceChange,
}: SandboxListFiltersProps) {
  const fields = buildSandboxFilterFields();
  const chips = sandboxFiltersToChips({ backend, persistence });

  const handleFiltersChange = (next: Filter<string>[]) => {
    applySandboxFilterChips(next, { onBackendChange, onPersistenceChange });
  };

  return (
    <FiltersWithSearch<string>
      allowMultiple={false}
      data-testid="sandbox-list-filters"
      fields={fields}
      filters={chips}
      onChange={handleFiltersChange}
      size="sm"
      trigger={
        <Button
          aria-label="Add filter"
          data-testid="sandbox-list-filters-add"
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
