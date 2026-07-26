import { useAgents, type AgentPayload } from "@/systems/agent";
import { useRuntimeModelCatalog, type RuntimeCatalogProvider } from "@/systems/model-catalog";
import type { RuntimeModelOption, RuntimeProviderOption } from "@/systems/runtime";

import { settingsProviderToOption } from "../lib/provider-runtime-option";
import { useSettingsProviders } from "./use-settings-collections";

export interface RolesRuntimeOptions {
  providers: RuntimeProviderOption[];
  models: RuntimeModelOption[];
  agents: AgentPayload[];
  /** Catalog read in flight; drives the selector's list loading state. */
  loading: boolean;
  /** Catalog has resolved at least once; gates the favorites purge. */
  catalogLoaded: boolean;
  catalogError: string | null;
  catalogStale: boolean;
  refresh: () => void;
  refreshing: boolean;
}

function toCatalogProviders(providers: RuntimeProviderOption[]): RuntimeCatalogProvider[] {
  return providers.map(provider => ({ id: provider.id, needsAuth: provider.needs_auth }));
}

/**
 * One catalog read for the whole Roles page. Every runtime selector on the page
 * — six role routes plus each fallback entry — is fed from this single query
 * through props; no selector fetches on its own.
 */
export function useRolesRuntimeOptions(): RolesRuntimeOptions {
  const providersQuery = useSettingsProviders();
  const agentsQuery = useAgents();

  const providers = (providersQuery.data?.providers ?? []).map(settingsProviderToOption);
  const catalog = useRuntimeModelCatalog(toCatalogProviders(providers));

  return {
    providers,
    models: catalog.models,
    agents: agentsQuery.data ?? [],
    loading: catalog.loading || providersQuery.isLoading,
    catalogLoaded: catalog.loaded,
    catalogError: catalog.error,
    catalogStale: catalog.stale,
    refresh: catalog.refresh,
    refreshing: catalog.refreshing,
  };
}
