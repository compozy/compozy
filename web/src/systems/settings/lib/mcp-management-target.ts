import type { SettingsMCPServerEntry, SettingsMCPServerTarget } from "../types";

type MCPManagementTarget = Exclude<SettingsMCPServerTarget, "auto">;

export type MCPManagementFilter =
  | { scope: "user"; target: MCPManagementTarget }
  | { scope: "profile"; target: MCPManagementTarget; profile: string; workspace_id?: string }
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
      return source.scope === "user" ? { scope: "user", target: "config" } : null;
    case "global-mcp-sidecar":
      return source.scope === "user" ? { scope: "user", target: "sidecar" } : null;
    case "profile-config":
    case "profile-mcp-sidecar": {
      if (source.scope !== "profile") return null;
      const profile = source.profile?.trim();
      if (!profile) return null;
      const workspaceId = source.workspace_id?.trim();
      return {
        scope: "profile",
        target: source.kind === "profile-config" ? "config" : "sidecar",
        profile,
        ...(workspaceId ? { workspace_id: workspaceId } : {}),
      };
    }
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
  if (filter.scope === "workspace") return `workspace · ${filter.workspace_id}`;
  if (filter.scope === "profile") return `profile · ${filter.profile}`;
  return "user";
}
