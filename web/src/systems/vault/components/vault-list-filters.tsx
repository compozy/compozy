import { ListFilter } from "lucide-react";

import { Button } from "@agh/ui";
import { FiltersWithSearch, type Filter } from "@agh/ui";

import type { VaultNamespaceFilter } from "../hooks/use-vault-page";
import {
  applyVaultFilterChips,
  buildVaultFilterFields,
  vaultFiltersToChips,
} from "../lib/vault-list-filters";

export interface VaultListFiltersProps {
  namespace: VaultNamespaceFilter;
  onNamespaceChange: (next: VaultNamespaceFilter) => void;
}

/**
 * Vault namespace filter chip bar for composition inside ListingToolbar.Filters.
 * Drives the server-side `namespace` query param (one chip, AND-combined with
 * the prefix search).
 */
export function VaultListFilters({ namespace, onNamespaceChange }: VaultListFiltersProps) {
  const fields = buildVaultFilterFields();
  const chips = vaultFiltersToChips({ namespace });

  const handleFiltersChange = (next: Filter<string>[]) => {
    applyVaultFilterChips(next, { onNamespaceChange });
  };

  return (
    <FiltersWithSearch<string>
      allowMultiple={false}
      data-testid="vault-list-filters"
      fields={fields}
      filters={chips}
      onChange={handleFiltersChange}
      size="sm"
      trigger={
        <Button
          aria-label="Add filter"
          data-testid="vault-list-filters-add"
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
