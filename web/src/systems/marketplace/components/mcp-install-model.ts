import type { MarketplaceEntryResponse, MCPInstallRequest } from "../types";

export type MCPEnvField = NonNullable<NonNullable<MarketplaceEntryResponse["mcp"]>["env"]>[number];

export interface MCPFieldBinding {
  mode: "typed" | "vault";
  typedValue: string;
  vaultRef: string;
  touched: boolean;
  createOpen: boolean;
  createValue: string;
  createError?: string;
}

export function createInitialMCPBindings(
  fields: readonly MCPEnvField[]
): Record<string, MCPFieldBinding> {
  return Object.fromEntries(
    fields.map(field => [
      field.name,
      {
        mode: "typed",
        typedValue: field.secret ? "" : (field.default ?? ""),
        vaultRef: "",
        touched: false,
        createOpen: false,
        createValue: "",
      } satisfies MCPFieldBinding,
    ])
  );
}

export function bindingValuePresent(binding: MCPFieldBinding): boolean {
  return binding.mode === "typed"
    ? binding.typedValue.trim() !== ""
    : binding.vaultRef.trim() !== "";
}

export function buildMCPInstallRequest(
  data: MarketplaceEntryResponse,
  scope: "global" | "workspace",
  workspaceId: string | null | undefined,
  bindings: Record<string, MCPFieldBinding>,
  remote: boolean
): MCPInstallRequest {
  const env = remote
    ? undefined
    : Object.fromEntries(
        (data.mcp?.env ?? []).flatMap(field => {
          const binding = bindings[field.name];
          if (!binding || !bindingValuePresent(binding)) return [];
          return [
            [
              field.name,
              binding.mode === "vault"
                ? { vault_ref: binding.vaultRef }
                : { value: binding.typedValue },
            ],
          ];
        })
      );
  return {
    entry_id: data.entry.entry_id,
    name: data.entry.name,
    scope,
    values: remote || !env || Object.keys(env).length === 0 ? null : { env },
    workspace_id: scope === "workspace" ? (workspaceId ?? undefined) : undefined,
  };
}
