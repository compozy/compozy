import type { MarketplaceEntryResponse, MCPInstallRequest } from "../types";
import type { MCPConfigScope } from "../hooks/marketplace-mcp-scope";

export type MCPInputField = NonNullable<
  NonNullable<MarketplaceEntryResponse["mcp"]>["inputs"]
>[number];

export interface MCPFieldBinding {
  mode: "typed" | "vault";
  typedValue: string;
  vaultRef: string;
  touched: boolean;
  createOpen: boolean;
  createValue: string;
  createError?: string;
}

function inputDefault(field: MCPInputField): string {
  if (typeof field.default === "boolean") return String(field.default);
  return typeof field.default === "string" ? field.default : "";
}

export function createInitialMCPBindings(
  fields: readonly MCPInputField[]
): Record<string, MCPFieldBinding> {
  return Object.fromEntries(
    fields.map(field => [
      field.id,
      {
        mode: "typed",
        typedValue: field.type === "secret" ? "" : inputDefault(field),
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
  scope: MCPConfigScope,
  workspaceId: string | null | undefined,
  profileName: string | null | undefined,
  bindings: Record<string, MCPFieldBinding>
): MCPInstallRequest {
  const inputs = Object.fromEntries(
    (data.mcp?.inputs ?? []).flatMap(field => {
      const binding = bindings[field.id];
      if (!binding || !bindingValuePresent(binding)) return [];
      return [
        [
          field.id,
          binding.mode === "vault"
            ? { vault_ref: binding.vaultRef }
            : { value: binding.typedValue },
        ],
      ];
    })
  );
  return {
    entry_id: data.entry.entry_id,
    scope,
    values: Object.keys(inputs).length === 0 ? null : { inputs },
    workspace_id: scope === "user" ? undefined : (workspaceId ?? undefined),
    profile: scope === "profile" ? (profileName ?? undefined) : undefined,
  };
}
