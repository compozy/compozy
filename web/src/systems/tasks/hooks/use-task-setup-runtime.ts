import {
  providerNeedsAuth,
  type RuntimeCatalogProvider,
  useRuntimeModelCatalog,
} from "@/systems/model-catalog";
import type { RuntimeProviderOption } from "@/systems/runtime";
import { type SettingsProviderEntry, useSettingsProviders } from "@/systems/settings";
import { type SessionProviderOption, useWorkspace } from "@/systems/workspace";

function settingsProviderOption(provider: SettingsProviderEntry): RuntimeProviderOption {
  return {
    id: provider.name,
    name: provider.settings.display_name?.trim() || provider.name,
    harness: provider.settings.harness?.trim() || undefined,
    runtime_provider: provider.settings.runtime_provider?.trim() || undefined,
    needs_auth: providerNeedsAuth(provider.auth_status?.state),
  };
}

function workspaceProviderOption(provider: SessionProviderOption): RuntimeProviderOption {
  return {
    id: provider.name,
    name: provider.display_name?.trim() || provider.name,
    harness: provider.harness?.trim() || undefined,
    runtime_provider: provider.runtime_provider?.trim() || provider.name,
  };
}

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
    ? (workspace.data?.providers ?? []).map(workspaceProviderOption)
    : (settings.data?.providers ?? []).map(settingsProviderOption);
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
