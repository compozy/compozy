import type { SettingsProviderEntry } from "../types";
import { deriveProviderStateLabel, type ProviderStateLabel } from "./provider-state";

export interface ProviderFilterState {
  statusFilter: ProviderStateLabel | null;
  nameQuery: string;
}

export const DEFAULT_PROVIDER_FILTERS: ProviderFilterState = {
  statusFilter: null,
  nameQuery: "",
};

export function applyProviderFilters(
  providers: SettingsProviderEntry[],
  state: ProviderFilterState
): SettingsProviderEntry[] {
  const query = state.nameQuery.trim().toLowerCase();
  return providers.filter(provider => {
    if (query.length > 0 && !providerMatchesQuery(provider, query)) return false;
    return !state.statusFilter || deriveProviderStateLabel(provider) === state.statusFilter;
  });
}

function providerMatchesQuery(provider: SettingsProviderEntry, query: string): boolean {
  return (
    provider.name.toLowerCase().includes(query) ||
    (provider.settings.display_name?.toLowerCase() ?? "").includes(query)
  );
}
