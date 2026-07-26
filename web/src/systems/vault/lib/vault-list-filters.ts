import type { Filter, FilterFieldsConfig } from "@agh/ui";

import type { VaultNamespaceFilter } from "../hooks/use-vault-page";
import { VAULT_NAMESPACES } from "../types";

export interface VaultFilterState {
  namespace: VaultNamespaceFilter;
}

export interface VaultFilterHandlers {
  onNamespaceChange: (next: VaultNamespaceFilter) => void;
}

/**
 * reui `<Filters>` field config for the vault listing toolbar. The daemon
 * filters by namespace server-side, so the chip's value maps straight to the
 * `namespace` query param — no client-side re-filtering.
 */
export function buildVaultFilterFields(): FilterFieldsConfig<string> {
  return [
    {
      key: "namespace",
      label: "Namespace",
      type: "select",
      options: VAULT_NAMESPACES.map(value => ({ value, label: value })),
    },
  ];
}

export function vaultFiltersToChips(state: VaultFilterState): Filter<string>[] {
  if (state.namespace === "all") {
    return [];
  }
  return [
    {
      id: "vault-filter-namespace",
      field: "namespace",
      operator: "is",
      values: [state.namespace],
    },
  ];
}

export function applyVaultFilterChips(
  chips: Filter<string>[],
  handlers: VaultFilterHandlers
): void {
  const value = chips.find(chip => chip.field === "namespace")?.values[0];
  handlers.onNamespaceChange(asVaultNamespace(value));
}

function asVaultNamespace(value: string | undefined): VaultNamespaceFilter {
  if (!value) return "all";
  return (VAULT_NAMESPACES as readonly string[]).includes(value)
    ? (value as VaultNamespaceFilter)
    : "all";
}
