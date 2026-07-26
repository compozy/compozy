import { useState } from "react";

import { usePutVaultSecret, useVaultSecrets } from "@/systems/vault";

import type { MarketplaceEntryResponse, MCPInstallRequest, MCPInstallResponse } from "../types";
import {
  bindingValuePresent,
  buildMCPInstallRequest,
  createInitialMCPBindings,
  type MCPEnvField,
  type MCPFieldBinding,
} from "./mcp-install-model";
import { marketplaceErrorMessage } from "./marketplace-ui";

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
  const [scope, setScope] = useState<"global" | "workspace">(defaultScope);
  const [bindings, setBindings] = useState<Record<string, MCPFieldBinding>>(() =>
    createInitialMCPBindings(fields)
  );
  const [installError, setInstallError] = useState<string | null>(null);
  const [installing, setInstalling] = useState(false);
  const vaultQuery = useVaultSecrets({ namespace: "mcp" });
  const putVault = usePutVaultSecret();

  const requiredComplete =
    remote ||
    fields.every(field => {
      const binding = bindings[field.name];
      return !field.required || (binding ? bindingValuePresent(binding) : false);
    });

  const updateBinding = (name: string, binding: MCPFieldBinding) => {
    setBindings(current => ({ ...current, [name]: binding }));
    setInstallError(null);
  };

  const canonicalRef = (field: MCPEnvField) => {
    const server = entry.install_slug?.trim() || entry.entry_id;
    return scope === "workspace" && workspaceId
      ? `vault:mcp/ws/${workspaceId}/${server}/env/${field.name}`
      : `vault:mcp/global/${server}/env/${field.name}`;
  };

  const createSecret = async (field: MCPEnvField) => {
    const binding = bindings[field.name];
    if (!binding) return;
    const ref = canonicalRef(field);
    try {
      await putVault.mutateAsync({
        kind: "mcp_env",
        ref,
        secret_value: binding.createValue,
      });
      updateBinding(field.name, {
        ...binding,
        createError: undefined,
        createOpen: false,
        createValue: "",
        mode: "vault",
        touched: true,
        vaultRef: ref,
      });
    } catch (error) {
      updateBinding(field.name, {
        ...binding,
        createError: marketplaceErrorMessage(error, "Failed to create the Vault secret"),
      });
    }
  };

  const install = async () => {
    if (!requiredComplete) {
      setBindings(current =>
        Object.fromEntries(
          Object.entries(current).map(([name, binding]) => [name, { ...binding, touched: true }])
        )
      );
      return;
    }
    setInstalling(true);
    setInstallError(null);
    await onInstall(buildMCPInstallRequest(data, scope, workspaceId, bindings, remote))
      .then(() => {
        onOpenChange(false);
      })
      .catch((error: unknown) => {
        setInstallError(marketplaceErrorMessage(error, "Failed to install the MCP server"));
      })
      .finally(() => {
        setInstalling(false);
      });
  };

  return {
    bindings,
    canonicalRef,
    createSecret,
    fields,
    install,
    installError,
    installing,
    putVault,
    remote,
    requiredComplete,
    scope,
    setScope,
    updateBinding,
    vaultQuery,
  };
}

export type { UseMCPInstallDialogOptions };
