import { type RuntimeCatalogProvider, useRuntimeModelCatalog } from "@/systems/model-catalog";
import type { RuntimeModelOption, RuntimeProviderOption } from "@/systems/runtime";
import { useWorkspace, workspaceProviderToOption } from "@/systems/workspace";

export interface SessionContinueRuntimeOptions {
  providers: RuntimeProviderOption[];
  models: RuntimeModelOption[];
  loading: boolean;
  refreshing: boolean;
  refresh: () => void;
  catalogError: string | null;
}

/**
 * Runtime-selector catalogs for the Continue dialog, scoped to the source's
 * workspace: the child is created there, so only its providers may be chosen.
 */
export function useSessionContinueRuntimeOptions(
  workspaceId: string,
  enabled: boolean
): SessionContinueRuntimeOptions {
  const workspace = useWorkspace(workspaceId, { enabled: enabled && workspaceId !== "" });
  const providers = (workspace.data?.providers ?? []).map(workspaceProviderToOption);
  const catalogProviders: RuntimeCatalogProvider[] = providers.map(provider => ({
    id: provider.id,
    needsAuth: provider.needs_auth,
  }));
  const catalog = useRuntimeModelCatalog(catalogProviders, {
    enabled: enabled && providers.length > 0,
  });
  return {
    providers,
    models: catalog.models,
    loading: workspace.isLoading || catalog.loading,
    refreshing: catalog.refreshing,
    refresh: catalog.refresh,
    catalogError: catalog.error,
  };
}
