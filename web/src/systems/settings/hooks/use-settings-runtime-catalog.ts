import { settingsProviderToOption } from "../lib/provider-runtime-option";
import { useSettingsProviders } from "./use-settings-collections";
import { type RuntimeCatalogProvider, useRuntimeModelCatalog } from "@/systems/model-catalog";
import type { RuntimeModelOption, RuntimeProviderOption } from "@/systems/runtime";

export interface SettingsRuntimeCatalog {
  providers: RuntimeProviderOption[];
  models: RuntimeModelOption[];
  /** Catalog read in flight; drives the selector's list loading state. */
  loading: boolean;
  catalogError: string | null;
  catalogStale: boolean;
  refresh: () => void;
  refreshing: boolean;
}

function toCatalogProviders(providers: RuntimeProviderOption[]): RuntimeCatalogProvider[] {
  return providers.map(provider => ({ id: provider.id, needsAuth: provider.needs_auth }));
}

/**
 * The configured providers and their model catalog, read once per settings
 * page and handed to every runtime selector on it through props.
 */
export function useSettingsRuntimeCatalog(): SettingsRuntimeCatalog {
  const providersQuery = useSettingsProviders();
  const providers = (providersQuery.data?.providers ?? []).map(settingsProviderToOption);
  const catalog = useRuntimeModelCatalog(toCatalogProviders(providers));

  return {
    providers,
    models: catalog.models,
    loading: catalog.loading || providersQuery.isLoading,
    catalogError: catalog.error,
    catalogStale: catalog.stale,
    refresh: catalog.refresh,
    refreshing: catalog.refreshing,
  };
}
