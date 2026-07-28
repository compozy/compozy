import type { SettingsMCPServerEntry, SettingsMCPServerTarget } from "../types";

type MCPManagementTarget = Exclude<SettingsMCPServerTarget, "auto">;

export type MCPManagementFilter =
  | { scope: "global"; target: MCPManagementTarget }
  | {
      scope: "workspace";
      target: MCPManagementTarget;
      workspace_id: string;
    };

/** Resolves the exact config owner that the daemon selected for an MCP row. */
export function deriveMCPManagementFilter(
  server: SettingsMCPServerEntry
): MCPManagementFilter | null {
  const source = server.source_metadata?.effective_source;
  switch (source?.kind) {
    case "global-config":
      return source.scope === "global" ? { scope: "global", target: "config" } : null;
    case "global-mcp-sidecar":
      return source.scope === "global" ? { scope: "global", target: "sidecar" } : null;
    case "workspace-config":
    case "workspace-mcp-sidecar": {
      if (source.scope !== "workspace") return null;
      const workspaceId = source.workspace_id?.trim();
      if (!workspaceId) return null;
      return {
        scope: "workspace",
        target: source.kind === "workspace-config" ? "config" : "sidecar",
        workspace_id: workspaceId,
      };
    }
    default:
      return null;
  }
}

export function mcpManagementScopeLabel(server: SettingsMCPServerEntry): string | null {
  const filter = deriveMCPManagementFilter(server);
  if (!filter) return null;
  return filter.scope === "workspace" ? `workspace · ${filter.workspace_id}` : "global";
}
