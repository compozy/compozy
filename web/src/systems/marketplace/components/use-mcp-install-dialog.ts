import { useSelector, useStore } from "@xstate/store-react";
import { useEffect, useEffectEvent } from "react";

import { usePutVaultSecret, useVaultSecrets } from "@/systems/vault";

import type { MarketplaceEntryResponse, MCPInstallRequest, MCPInstallResponse } from "../types";
import {
  bindingValuePresent,
  buildMCPInstallRequest,
  createInitialMCPBindings,
  type MCPEnvField,
  type MCPFieldBinding,
} from "./mcp-install-model";
import { mcpInstallDialogLogic } from "./mcp-install-dialog-store";

interface UseMCPInstallDialogOptions {
  data: MarketplaceEntryResponse;
  onInstall: (request: MCPInstallRequest) => Promise<MCPInstallResponse>;
  onOpenChange: (open: boolean) => void;
  workspaceId?: string | null;
}

export function useMCPInstallDialog({
  data,
  onInstall,
  onOpenChange,
  workspaceId,
}: UseMCPInstallDialogOptions) {
  const { entry, mcp } = data;
  const fields = mcp?.env ?? [];
  const remote = Boolean(mcp?.url) || mcp?.transport !== "stdio";
  const defaultScope = mcp?.default_scope === "global" || !workspaceId ? "global" : "workspace";
  const vaultQuery = useVaultSecrets({ namespace: "mcp" });
  const putVault = usePutVaultSecret();
  const store = useStore(mcpInstallDialogLogic, {
    scope: defaultScope,
    bindings: createInitialMCPBindings(fields),
  });
  const state = useSelector(store, snapshot => snapshot.context);
  const closeAfterInstall = useEffectEvent(() => onOpenChange(false));

  useEffect(() => {
    const installed = store.on("installed", closeAfterInstall);
    return () => installed.unsubscribe();
  }, [store]);

  const requiredComplete =
    remote ||
    fields.every(field => {
      const binding = state.bindings[field.name];
      return !field.required || (binding ? bindingValuePresent(binding) : false);
    });
  const canonicalRef = (field: MCPEnvField) => {
    const server = entry.install_slug?.trim() || entry.entry_id;
    return state.scope === "workspace" && workspaceId
      ? `vault:mcp/ws/${workspaceId}/${server}/env/${field.name}`
      : `vault:mcp/global/${server}/env/${field.name}`;
  };

  return {
    bindings: state.bindings,
    canonicalRef,
    createSecret: (field: MCPEnvField) => {
      const binding = state.bindings[field.name];
      if (!binding) return;
      store.trigger.secretCreationRequested({
        createSecret: async (ref, value) => {
          await putVault.mutateAsync({ kind: "mcp_env", ref, secret_value: value });
        },
        name: field.name,
        ref: canonicalRef(field),
        value: binding.createValue,
      });
    },
    fields,
    install: () =>
      store.trigger.installationRequested({
        install: (bindings, scope) =>
          onInstall(buildMCPInstallRequest(data, scope, workspaceId, bindings, remote)).then(
            () => undefined
          ),
      }),
    installError: state.installError,
    installing: state.phase === "installing",
    putVault,
    remote,
    requiredComplete,
    scope: state.scope,
    setScope: (scope: "global" | "workspace") => store.trigger.scopeSelected({ scope }),
    updateBinding: (name: string, binding: MCPFieldBinding) =>
      store.trigger.bindingChanged({ name, binding }),
    vaultQuery,
  };
}

export type { UseMCPInstallDialogOptions };
