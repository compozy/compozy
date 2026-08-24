import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { extensionNetworkConfirmation } from "../lib/extension-network-confirmation";
import { extensionTrustFacts } from "../lib/extension-trust-facts";
import {
  useToggleExtension,
  useUpdateExtension,
  type ToggleExtensionVariables,
  type UpdateExtensionVariables,
} from "./use-extension-actions";
import { useExtensionLogs, type ExtensionLogEventSource } from "./use-extension-logs";
import { useExtensionDetail, useExtensionKitInventory } from "./use-extensions";

export type ExtensionDetailDialog = "provenance" | "remove" | "update" | null;

/**
 * A refused lifecycle mutation is resumed, not rebuilt: the pending record carries the exact
 * variables the daemon refused so the retry ratifies the digest without silently changing what was
 * asked for (an unverified update keeps its `allowUnverified` and resolved version).
 */
export type ExtensionNetworkConfirm = {
  digest: string;
  variables: UpdateExtensionVariables;
};

type ExtensionDetailDialogState =
  | { type: "closed" }
  | { type: "dialog"; dialog: Exclude<ExtensionDetailDialog, null> }
  | { type: "network-confirm"; confirmation: ExtensionNetworkConfirm };

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
  const networkConfirm = dialogState.type === "network-confirm" ? dialogState.confirmation : null;
  const extension = detail.data?.extension ?? null;
  const instanceWorkspaceId = extension?.workspace_id?.trim() || null;
  const inventory = useExtensionKitInventory(
    name,
    extension !== null && instanceWorkspaceId === null
  );
  const logs = useExtensionLogs({
    enabled: extension !== null,
    eventSourceFactory: options.logEventSourceFactory,
    name,
    workspaceId: instanceWorkspaceId,
  });

  const updateVariables = (allowUnverified: boolean): UpdateExtensionVariables | null => {
    if (!extension) return null;
    return {
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
    } catch (error) {
      const confirmation = extensionNetworkConfirmation(error);
      if (!confirmation) return;
      setDialogState({
        type: "network-confirm",
        confirmation: { digest: confirmation.digest, variables },
      });
    }
  };

  return {
    activeDialog,
    detail,
    dismissDialog: () =>
      setDialogState(current => (current.type === "dialog" ? CLOSED_DIALOG_STATE : current)),
    dismissNetworkConfirm: () =>
      setDialogState(current =>
        current.type === "network-confirm" ? CLOSED_DIALOG_STATE : current
      ),
    inventory,
    logs,
    navigate,
    networkConfirm,
    requestProvenance: () => setDialogState({ type: "dialog", dialog: "provenance" }),
    requestRemoval: () => setDialogState({ type: "dialog", dialog: "remove" }),
    requestToggle: async (enabled: boolean) => {
      if (!extension) return;
      await runToggle({ enabled, name: extension.name });
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
    /** Resumes the refused update with its original variables plus the ratified digest. */
    submitNetworkConfirm: async () => {
      if (!networkConfirm) return;
      await runUpdate({
        ...networkConfirm.variables,
        confirmNetworkDigest: networkConfirm.digest,
      });
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
