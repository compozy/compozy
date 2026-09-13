import { useMutation, useQueryClient } from "@tanstack/react-query";
import { reconcileInstalledExtensionCaches } from "@/integrations/tanstack-query/reconcile-installed-extension";
import { updateExtension } from "@/systems/extensions/adapters/extensions-api";
import {
  updateMarketplaceExtensions,
  installMarketplaceExtension,
  refreshMarketplaceCatalog,
} from "../adapters/marketplace-actions-api";
import { marketplaceKeys } from "../lib/query-keys";
import type {
  ExtensionBatchUpdateRequest,
  ExtensionInstallRequest,
  ExtensionUpdateRequest,
} from "../types";

/** Replaces in-flight pre-mutation reads before refreshing authoritative marketplace pages. */
async function invalidateMarketplace(queryClient: ReturnType<typeof useQueryClient>) {
  await queryClient.cancelQueries({ queryKey: marketplaceKeys.all });
  return queryClient.invalidateQueries({ queryKey: marketplaceKeys.all });
}

export function useRefreshMarketplaceCatalog() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => refreshMarketplaceCatalog(),
    onSettled: () => invalidateMarketplace(queryClient),
  });
}

export function useInstallMarketplaceExtension() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: ExtensionInstallRequest) => installMarketplaceExtension(body),
    onSettled: () => reconcileInstalledExtensionCaches(queryClient),
  });
}

export function useUpdateMarketplaceExtension() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ name, body }: { name: string; body: ExtensionUpdateRequest }) =>
      updateExtension(name, body),
    onSettled: () => reconcileInstalledExtensionCaches(queryClient),
  });
}

export function useUpdateMarketplaceExtensions() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: ExtensionBatchUpdateRequest) => updateMarketplaceExtensions(body),
    onSettled: () => reconcileInstalledExtensionCaches(queryClient),
  });
}
