import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { extensionInstallationScope } from "../lib/extension-installation-scope";
import { extensionTrustFacts } from "../lib/extension-trust-facts";
import {
  useToggleExtension,
  useUpdateExtension,
  type ToggleExtensionVariables,
  type UpdateExtensionVariables,
} from "./use-extension-actions";
import { useExtensionLogs, type ExtensionLogEventSource } from "./use-extension-logs";
import { useExtensionDetail, useExtensionKitInventory } from "./use-extensions";

export type ExtensionDetailDialog = "provenance" | "remove" | "setup" | "update" | null;

type ExtensionDetailDialogState =
  | { type: "closed" }
  | { type: "dialog"; dialog: Exclude<ExtensionDetailDialog, null> };

const CLOSED_DIALOG_STATE: ExtensionDetailDialogState = { type: "closed" };

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
  const toggle = useToggleExtension();
  const update = useUpdateExtension();
  const navigate = useNavigate();
  const [dialogState, setDialogState] = useState<ExtensionDetailDialogState>(CLOSED_DIALOG_STATE);
  const activeDialog = dialogState.type === "dialog" ? dialogState.dialog : null;
  const extension = detail.data?.extension ?? null;
  const instanceWorkspaceId = extension?.workspace_id?.trim() || null;
  const inventory = useExtensionKitInventory(
    name,
    { workspaceId: instanceWorkspaceId, profileName: extension?.profile ?? detail.profileName },
    extension !== null
  );
  const logs = useExtensionLogs({
    enabled: extension !== null,
    eventSourceFactory: options.logEventSourceFactory,
    name,
    workspaceId: instanceWorkspaceId,
    profileName: extension?.profile ?? detail.profileName,
  });

  const updateVariables = (allowUnverified: boolean): UpdateExtensionVariables | null => {
    if (!extension) return null;
    return {
      ...extensionInstallationScope(extension),
      scope: extension.workspace_id ? "workspace" : "global",
      allowUnverified,
      name: extension.name,
      version: options.updateVersion?.trim() || extension.remote_version?.trim() || undefined,
    };
  };

  const runToggle = async (variables: ToggleExtensionVariables) => {
    await toggle.mutateAsync(variables);
  };

  const runUpdate = async (variables: UpdateExtensionVariables) => {
    try {
      await update.mutateAsync(variables);
      setDialogState(CLOSED_DIALOG_STATE);
    } catch {
      // The mutation reports the error and the dialog stays open for retry.
    }
  };

  return {
    activeDialog,
    detail,
    dismissDialog: () =>
      setDialogState(current => (current.type === "dialog" ? CLOSED_DIALOG_STATE : current)),
    inventory,
    logs,
    navigate,
    requestProvenance: () => setDialogState({ type: "dialog", dialog: "provenance" }),
    requestRemoval: () => setDialogState({ type: "dialog", dialog: "remove" }),
    requestSetup: () => setDialogState({ type: "dialog", dialog: "setup" }),
    requestToggle: async (enabled: boolean) => {
      if (!extension) return;
      await runToggle({
        enabled,
        name: extension.name,
        profileName: extension.profile,
        workspaceId: instanceWorkspaceId,
      });
    },
    /**
     * An unverified installation still needs an explicit per-update decision, so the consent
     * dialog opens instead of silently sending `allow_unverified`.
     */
    requestUpdate: async () => {
      if (!extension) return;
      if (extensionTrustFacts(extension).checksumVerified) {
        const variables = updateVariables(false);
        if (variables) await runUpdate(variables);
        return;
      }
      setDialogState({ type: "dialog", dialog: "update" });
    },
    submitUpdate: async () => {
      const variables = updateVariables(true);
      if (variables) await runUpdate(variables);
    },
    toggle,
    update,
    workspaceId: instanceWorkspaceId,
  };
}
