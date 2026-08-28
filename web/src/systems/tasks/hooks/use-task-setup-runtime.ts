import { type RuntimeCatalogProvider, useRuntimeModelCatalog } from "@/systems/model-catalog";
import { settingsProviderToOption, useSettingsProviders } from "@/systems/settings";
import { useWorkspace, workspaceProviderToOption } from "@/systems/workspace";

/** Runtime-selector data scoped to the task's workspace. */
export function useTaskSetupRuntime(
  workspaceId?: string | null,
  options: { enabled?: boolean } = {}
) {
  const enabled = options.enabled ?? true;
  const workspace = useWorkspace(workspaceId ?? "", {
    enabled: enabled && Boolean(workspaceId),
  });
  const settings = useSettingsProviders({ enabled: enabled && !workspaceId });
  const providers = workspaceId
    ? (workspace.data?.providers ?? []).map(workspaceProviderToOption)
    : (settings.data?.providers ?? []).map(settingsProviderToOption);
  const catalogProviders: RuntimeCatalogProvider[] = providers.map(provider => ({
    id: provider.id,
    needsAuth: provider.needs_auth,
  }));
  const catalog = useRuntimeModelCatalog(catalogProviders, {
    enabled: enabled && providers.length > 0,
  });

  return {
    catalogError: catalog.error,
    catalogLoading: catalog.loading,
    catalogRefreshing: catalog.refreshing,
    models: catalog.models,
    onRefreshCatalog: catalog.refresh,
    providers,
    providersError: workspaceId ? workspace.error?.message : settings.error?.message,
    providersLoading: workspaceId ? workspace.isLoading : settings.isLoading,
  };
}
