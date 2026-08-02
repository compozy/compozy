import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { reconcileInstalledExtensionCaches } from "@/integrations/tanstack-query/reconcile-installed-extension";

import {
  disableExtension,
  enableExtension,
  removeExtension,
  updateExtension,
} from "../adapters/extensions-api";
import { extensionNetworkConfirmation } from "../lib/extension-network-confirmation";
import { extensionKeys } from "../lib/query-keys";
import type { ExtensionEnableResult, ExtensionEntry } from "../types";
import { useExtensionInstanceScope } from "./use-extensions";

export interface ToggleExtensionVariables {
  name: string;
  enabled: boolean;
  confirmNetworkDigest?: string;
}

export interface UpdateExtensionVariables {
  name: string;
  allowUnverified?: boolean;
  version?: string;
  confirmNetworkDigest?: string;
}

export interface RemoveExtensionVariables {
  dev: boolean;
  name: string;
}

/**
 * A refused network confirmation is not a failure the operator has to read twice: the confirm
 * affordance owns it, so the toast stays out of the way.
 */
function toastUnlessNetworkConfirmation(error: Error) {
  if (extensionNetworkConfirmation(error)) return;
  toast.error(error.message);
}

function enabledToast(result: ExtensionEnableResult, name: string) {
  const started = result.automation_started;
  if (started.length === 0) {
    toast.success(`${name} enabled`);
    return;
  }
  toast.success(`${name} enabled · ${started.length} automation started`, {
    description: [...started].sort().join(", "),
  });
}

export function useToggleExtension() {
  const queryClient = useQueryClient();
  const { workspaceId } = useExtensionInstanceScope();
  const listKey = extensionKeys.list(workspaceId);
  return useMutation<
    ExtensionEnableResult | ExtensionEntry,
    Error,
    ToggleExtensionVariables,
    { previous?: ExtensionEntry[] }
  >({
    mutationFn: ({ name, enabled, confirmNetworkDigest }) =>
      enabled ? enableExtension(name, { confirmNetworkDigest }) : disableExtension(name),
    onMutate: async ({ name, enabled }) => {
      await queryClient.cancelQueries({ queryKey: listKey });
      const previous = queryClient.getQueryData<ExtensionEntry[]>(listKey);
      queryClient.setQueryData<ExtensionEntry[]>(listKey, current =>
        current?.map(item => (item.name === name ? { ...item, enabled } : item))
      );
      return { previous };
    },
    onSuccess: (data, { enabled, name }) => {
      if (enabled && "automation_started" in data) enabledToast(data, name);
    },
    onError: (error, _variables, context) => {
      if (context?.previous) queryClient.setQueryData(listKey, context.previous);
      toastUnlessNetworkConfirmation(error);
    },
    /**
     * Enabling and disabling change which kit resources are live and whether confirmation is still
     * required, so the whole extension cache — inventory included — is reconciled, not just the list.
     */
    onSettled: () => reconcileInstalledExtensionCaches(queryClient),
  });
}

/** Updates address the marketplace-managed published installation; they are never workspace-scoped. */
export function useUpdateExtension() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      name,
      allowUnverified,
      version,
      confirmNetworkDigest,
    }: UpdateExtensionVariables) =>
      updateExtension(name, {
        allow_unverified: allowUnverified === true,
        ...(version ? { version } : {}),
        ...(confirmNetworkDigest ? { confirm_network_digest: confirmNetworkDigest } : {}),
      }),
    onSuccess: (_data, { name, version }) =>
      toast.success(version ? `${name} updated to v${version}` : `${name} updated`),
    onError: toastUnlessNetworkConfirmation,
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
