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

export function useToggleExtension() {
  const queryClient = useQueryClient();
  return useMutation<
    ExtensionEntry,
    Error,
    { name: string; enabled: boolean },
    { previous?: ExtensionEntry[] }
  >({
    mutationFn: ({ name, enabled }) => (enabled ? enableExtension(name) : disableExtension(name)),
    onMutate: async ({ name, enabled }) => {
      await queryClient.cancelQueries({ queryKey: extensionKeys.list() });
      const previous = queryClient.getQueryData<ExtensionEntry[]>(extensionKeys.list());
      queryClient.setQueryData<ExtensionEntry[]>(extensionKeys.list(), current =>
        current?.map(item => (item.name === name ? { ...item, enabled } : item))
      );
      return { previous };
    },
    onError: (error, _variables, context) => {
      if (context?.previous) queryClient.setQueryData(extensionKeys.list(), context.previous);
      toast.error(error.message);
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: extensionKeys.list() }),
  });
}

export function useUpdateExtension() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => updateExtension(name, {}),
    onSuccess: (_data, name) => toast.success(`${name} updated`),
    onError: (error: Error) => toast.error(error.message),
    onSettled: () => reconcileInstalledExtensionCaches(queryClient),
  });
}

export function useRemoveExtension() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => removeExtension(name),
    onSuccess: (_data, name) => toast.success(`${name} removed`),
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
