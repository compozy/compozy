import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { reconcileInstalledExtensionCaches } from "@/integrations/tanstack-query/reconcile-installed-extension";

import {
  removeExtension,
  setExtensionEnablement,
  updateExtension,
} from "../adapters/extensions-api";
import { extensionNetworkConfirmation } from "../lib/extension-network-confirmation";
import { extensionKeys } from "../lib/query-keys";
import type {
  ExtensionEnablement,
  ExtensionEntry,
  ExtensionInstanceScope,
  ExtensionUpdateRequest,
} from "../types";
import { useExtensionInstanceScope } from "./use-extensions";

export interface ToggleExtensionVariables extends Pick<ExtensionInstanceScope, "workspaceId"> {
  profileName?: string;
  name: string;
  enabled: boolean;
}

export interface UpdateExtensionVariables extends ExtensionInstanceScope {
  scope?: ExtensionUpdateRequest["scope"];
  inputs?: ExtensionUpdateRequest["inputs"];
  name: string;
  allowUnverified?: boolean;
  version?: string;
  confirmNetworkDigest?: string;
}

export interface RemoveExtensionVariables extends ExtensionInstanceScope {
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

export function useToggleExtension() {
  const queryClient = useQueryClient();
  const { profileName, workspaceId } = useExtensionInstanceScope();
  return useMutation<
    ExtensionEnablement,
    Error,
    ToggleExtensionVariables,
    { previous?: ExtensionEntry[]; listKey: ReturnType<typeof extensionKeys.list> }
  >({
    mutationFn: ({ name, enabled, profileName: selectedProfile = profileName }) =>
      setExtensionEnablement(name, selectedProfile, enabled),
    onMutate: async ({
      name,
      enabled,
      profileName: selectedProfile = profileName,
      workspaceId: selectedWorkspace = workspaceId,
    }) => {
      const listKey = extensionKeys.list(selectedWorkspace, selectedProfile);
      await queryClient.cancelQueries({ queryKey: listKey });
      const previous = queryClient.getQueryData<ExtensionEntry[]>(listKey);
      queryClient.setQueryData<ExtensionEntry[]>(listKey, current =>
        current?.map(item => (item.name === name ? { ...item, enabled } : item))
      );
      return { previous, listKey };
    },
    onSuccess: (_data, { enabled, name, profileName: selectedProfile = profileName }) =>
      toast.success(`${name} ${enabled ? "enabled" : "disabled"} in ${selectedProfile}`),
    onError: (error, _variables, context) => {
      if (context?.previous) queryClient.setQueryData(context.listKey, context.previous);
      toast.error(error.message);
    },
    /**
     * Enabling and disabling change which kit resources are live, so the whole extension cache —
     * inventory included — is reconciled, not just the list.
     */
    onSettled: () => reconcileInstalledExtensionCaches(queryClient),
  });
}

/** The selected published installation owns update inputs; dev overlays keep their own lifecycle. */
export function useUpdateExtension() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      name,
      allowUnverified,
      version,
      confirmNetworkDigest,
      scope,
      workspaceId,
      profileName,
      inputs,
    }: UpdateExtensionVariables) =>
      updateExtension(name, {
        allow_unverified: allowUnverified === true,
        ...(scope ? { scope } : {}),
        ...(workspaceId ? { workspace_id: workspaceId } : {}),
        ...(profileName ? { profile: profileName } : {}),
        ...(inputs ? { inputs } : {}),
        ...(version ? { version } : {}),
        ...(confirmNetworkDigest ? { confirm_network_digest: confirmNetworkDigest } : {}),
      }),
    onSuccess: (_data, { name, version }) =>
      toast.success(version ? `${name} updated to v${version}` : `${name} updated`),
    onError: toastUnlessNetworkConfirmation,
    onSettled: () => reconcileInstalledExtensionCaches(queryClient),
  });
}

/** Removes the selected attachment or unlinks its dev overlay using the same instance axes. */
export function useRemoveExtension() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ name, workspaceId, profileName }: RemoveExtensionVariables) =>
      removeExtension(name, { workspaceId, profileName }),
    onSuccess: (_data, { dev, name }) =>
      toast.success(dev ? `${name} dev overlay unlinked` : `${name} removed`),
    onError: (error: Error) => toast.error(error.message),
    onSettled: () => reconcileInstalledExtensionCaches(queryClient),
  });
}
