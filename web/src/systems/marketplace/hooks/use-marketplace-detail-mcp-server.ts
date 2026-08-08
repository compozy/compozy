import {
  SETTINGS_QUERY_INTERVALS,
  useSettingsMCPServers,
  type SettingsMCPServerEntry,
} from "@/systems/settings";
import { useActiveWorkspace } from "@/systems/workspace";

import type { MarketplaceEntryResponse } from "../types";

type SettingsMCPServersFilter =
  | { scope: "global" }
  | { scope: "workspace"; workspace_id: string | undefined };

function findInstalledMCPServer(
  entry: MarketplaceEntryResponse["entry"],
  servers: readonly SettingsMCPServerEntry[]
): SettingsMCPServerEntry | undefined {
  const installedName = entry.installed_name?.trim();
  if (installedName) {
    return servers.find(server => server.name === installedName);
  }
  // Installed-only entries expose the server name as entry_id; display names
  // can collide with unrelated servers, so they never participate in matching.
  return servers.find(
    server => server.catalog_entry === entry.entry_id || server.name === entry.entry_id
  );
}

/**
 * Resolves the installed settings row backing a marketplace MCP entry. The
 * collection query is shared (same key) between the detail body and the OS-head
 * authorize action, so both read one cache entry.
 */
function useMarketplaceDetailMCPServer(
  entry: MarketplaceEntryResponse["entry"],
  scope?: "global" | "workspace",
  workspaceId?: string
) {
  const { activeWorkspaceId } = useActiveWorkspace();
  const resolvedWorkspaceId = workspaceId ?? activeWorkspaceId ?? undefined;
  const resolvedScope = scope ?? (resolvedWorkspaceId ? "workspace" : "global");
  const queryFilter: SettingsMCPServersFilter =
    resolvedScope === "workspace"
      ? { scope: "workspace", workspace_id: resolvedWorkspaceId }
      : { scope: "global" };
  const queryEnabled =
    entry.installed && (resolvedScope === "global" || Boolean(resolvedWorkspaceId));
  const query = useSettingsMCPServers(queryFilter, {
    enabled: queryEnabled,
    refetchInterval: SETTINGS_QUERY_INTERVALS.collectionRefetchInterval,
  });
  const server = findInstalledMCPServer(entry, query.data?.mcp_servers ?? []);
  return { query, queryEnabled, queryFilter, server };
}

export { findInstalledMCPServer, useMarketplaceDetailMCPServer };
export type { SettingsMCPServersFilter };
