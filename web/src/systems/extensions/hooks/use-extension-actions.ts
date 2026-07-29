import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { reconcileInstalledExtensionCaches } from "@/integrations/tanstack-query/reconcile-installed-extension";

import {
  deactivateBundle,
  disableExtension,
  enableExtension,
  removeExtension,
  updateBundleActivation,
  updateExtension,
} from "../adapters/extensions-api";
import { extensionKeys } from "../lib/query-keys";
import type { BundleActivationUpdateRequest, ExtensionEntry } from "../types";
import { useExtensionInstanceScope } from "./use-extensions";

export function useToggleExtension() {
  const queryClient = useQueryClient();
  const { workspaceId } = useExtensionInstanceScope();
  const listKey = extensionKeys.list(workspaceId);
  return useMutation<
    ExtensionEntry,
    Error,
    { name: string; enabled: boolean },
    { previous?: ExtensionEntry[] }
  >({
    mutationFn: ({ name, enabled }) => (enabled ? enableExtension(name) : disableExtension(name)),
    onMutate: async ({ name, enabled }) => {
      await queryClient.cancelQueries({ queryKey: listKey });
      const previous = queryClient.getQueryData<ExtensionEntry[]>(listKey);
      queryClient.setQueryData<ExtensionEntry[]>(listKey, current =>
        current?.map(item => (item.name === name ? { ...item, enabled } : item))
      );
      return { previous };
    },
    onError: (error, _variables, context) => {
      if (context?.previous) queryClient.setQueryData(listKey, context.previous);
      toast.error(error.message);
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: extensionKeys.lists() }),
  });
}

export interface UpdateExtensionVariables {
  name: string;
  allowUnverified?: boolean;
  version?: string;
}

export interface RemoveExtensionVariables {
  dev: boolean;
  name: string;
}

/** Updates address the marketplace-managed published installation; they are never workspace-scoped. */
export function useUpdateExtension() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ name, allowUnverified, version }: UpdateExtensionVariables) =>
      updateExtension(name, {
        allow_unverified: allowUnverified === true,
        ...(version ? { version } : {}),
      }),
    onSuccess: (_data, { name, version }) =>
      toast.success(version ? `${name} updated to v${version}` : `${name} updated`),
    onError: (error: Error) => toast.error(error.message),
    onSettled: () => reconcileInstalledExtensionCaches(queryClient),
  });
}

/** Explicitly separates a workspace dev unlink from removal of the published installation. */
export function useRemoveExtension() {
  const queryClient = useQueryClient();
  const scope = useExtensionInstanceScope();
  return useMutation({
    mutationFn: ({ dev, name }: RemoveExtensionVariables) =>
      removeExtension(name, dev ? scope : {}),
    onSuccess: (_data, { dev, name }) =>
      toast.success(dev ? `${name} dev overlay unlinked` : `${name} removed`),
    onError: (error: Error) => toast.error(error.message),
    onSettled: () => reconcileInstalledExtensionCaches(queryClient),
  });
}

export function useUpdateBundleActivation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: BundleActivationUpdateRequest }) =>
      updateBundleActivation(id, body),
    onSuccess: activation => toast.success(`${activation.bundle_name} updated`),
    onError: (error: Error) => toast.error(error.message),
    onSettled: (_data, _error, variables) => {
      void queryClient.invalidateQueries({ queryKey: extensionKeys.bundles() });
      if (variables?.id)
        void queryClient.invalidateQueries({ queryKey: extensionKeys.bundle(variables.id) });
    },
  });
}

export function useDeactivateBundle() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deactivateBundle(id),
    onSuccess: () => toast.success("Bundle deactivated"),
    onError: (error: Error) => toast.error(error.message),
    onSettled: () => queryClient.invalidateQueries({ queryKey: extensionKeys.bundles() }),
  });
}
