import { useSelector, useStore } from "@xstate/store-react";
import { useEffect, useEffectEvent } from "react";

import type { MarketplaceEntryResponse, MCPInstallRequest, MCPInstallResponse } from "../types";
import {
  bindingValuePresent,
  buildMCPInstallRequest,
  createInitialMCPBindings,
  type MCPInputField,
  type MCPFieldBinding,
} from "./mcp-install-model";
import { mcpInstallDialogLogic } from "./mcp-install-dialog-store";
import { usePutVaultSecret, useVaultSecrets } from "@/systems/vault";
import type { SettingsLayeredScope } from "@/systems/settings";

interface UseMCPInstallDialogOptions {
  data: MarketplaceEntryResponse;
  onInstall: (request: MCPInstallRequest) => Promise<MCPInstallResponse>;
  onOpenChange: (open: boolean) => void;
  workspaceId?: string | null;
  scope?: SettingsLayeredScope;
  profileName?: string | null;
}

export function useMCPInstallDialog({
  data,
  onInstall,
  onOpenChange,
  workspaceId,
  scope: requestedScope,
  profileName,
}: UseMCPInstallDialogOptions) {
  const { entry, mcp } = data;
  const fields = mcp?.inputs ?? [];
  const remote = mcp?.launch?.type === "remote";
  const resolvedScope = requestedScope ?? (workspaceId ? "workspace" : "user");
  const vaultQuery = useVaultSecrets({ namespace: "mcp" });
  const putVault = usePutVaultSecret();
  const store = useStore(mcpInstallDialogLogic, {
    bindings: createInitialMCPBindings(fields),
  });
  const state = useSelector(store, snapshot => snapshot.context);
  const closeAfterInstall = useEffectEvent(() => onOpenChange(false));

  useEffect(() => {
    const installed = store.on("installed", closeAfterInstall);
    return () => installed.unsubscribe();
  }, [store]);

  const requiredComplete = fields.every(field => {
    const binding = state.bindings[field.id];
    return !field.required || (binding ? bindingValuePresent(binding) : false);
  });
  const canonicalRef = (field: MCPInputField) => {
    const server = entry.install_slug?.trim() || entry.entry_id;
    if (resolvedScope === "profile" && profileName) {
      return `vault:mcp/profile/${profileName}/${server}/inputs/${field.id}`;
    }
    return resolvedScope === "workspace" && workspaceId
      ? `vault:mcp/ws/${workspaceId}/${server}/inputs/${field.id}`
      : `vault:mcp/user/${server}/inputs/${field.id}`;
  };

  return {
    bindings: state.bindings,
    canonicalRef,
    createSecret: (field: MCPInputField) => {
      const binding = state.bindings[field.id];
      if (!binding) return;
      store.trigger.secretCreationRequested({
        createSecret: async (ref, value) => {
          await putVault.mutateAsync({ kind: "mcp_env", ref, secret_value: value });
        },
        name: field.id,
        ref: canonicalRef(field),
        value: binding.createValue,
      });
    },
    fields,
    install: () =>
      store.trigger.installationRequested({
        scope: resolvedScope,
        install: (bindings, scope) =>
          onInstall(buildMCPInstallRequest(data, scope, workspaceId, profileName, bindings)).then(
            () => undefined
          ),
      }),
    installError: state.installError,
    installing: state.phase === "installing",
    putVault,
    remote,
    requiredComplete:
      requiredComplete &&
      (resolvedScope === "user" ||
        (resolvedScope === "profile" ? Boolean(profileName) : Boolean(workspaceId))),
    scope: resolvedScope,
    updateBinding: (name: string, binding: MCPFieldBinding) =>
      store.trigger.bindingChanged({ name, binding }),
    vaultQuery,
  };
}

export type { UseMCPInstallDialogOptions };
