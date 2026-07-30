import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { extensionTrustFacts } from "../lib/extension-trust-facts";
import { useToggleExtension, useUpdateExtension } from "./use-extension-actions";
import { useExtensionLogs, type ExtensionLogEventSource } from "./use-extension-logs";
import { useBundleActivations, useExtensionDetail } from "./use-extensions";

export type ExtensionDetailDialog = "provenance" | "remove" | "update" | null;

/**
 * Detail-screen UI state is local to the mounted extension route. Queries and
 * mutations keep their existing TanStack Query ownership.
 */
export function useExtensionDetailState(
  name: string,
  options: {
    logEventSourceFactory?: (url: string) => ExtensionLogEventSource;
    updateVersion?: string;
  } = {}
) {
  const detail = useExtensionDetail(name);
  const bundles = useBundleActivations();
  const toggle = useToggleExtension();
  const update = useUpdateExtension();
  const navigate = useNavigate();
  const [activeDialog, setActiveDialog] = useState<ExtensionDetailDialog>(null);
  const extension = detail.data?.extension ?? null;
  const instanceWorkspaceId = extension?.workspace_id ?? null;
  const logs = useExtensionLogs({
    enabled: extension !== null,
    eventSourceFactory: options.logEventSourceFactory,
    name,
    workspaceId: instanceWorkspaceId,
  });

  const updateVariables = (allowUnverified: boolean) => {
    if (!extension) return null;
    return {
      allowUnverified,
      name: extension.name,
      version: options.updateVersion?.trim() || extension.remote_version?.trim() || undefined,
    };
  };
  const runUpdate = async (allowUnverified: boolean) => {
    const variables = updateVariables(allowUnverified);
    if (!variables) return;
    await update.mutateAsync(variables);
    setActiveDialog(null);
  };

  return {
    activeDialog,
    bundles,
    detail,
    dismissDialog: () => setActiveDialog(null),
    logs,
    navigate,
    requestProvenance: () => setActiveDialog("provenance"),
    requestRemoval: () => setActiveDialog("remove"),
    /**
     * An unverified installation still needs an explicit per-update decision, so the consent
     * dialog opens instead of silently sending `allow_unverified`.
     */
    requestUpdate: () => {
      if (!extension) return;
      if (extensionTrustFacts(extension).checksumVerified) {
        const variables = updateVariables(false);
        if (variables) update.mutate(variables);
        return;
      }
      setActiveDialog("update");
    },
    submitUpdate: () => runUpdate(true),
    toggle,
    update,
    workspaceId: instanceWorkspaceId,
  };
}
