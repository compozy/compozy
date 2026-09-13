import type { MarketplaceEntryResponse, MarketplaceExtensionServer } from "../types";
import {
  SETTINGS_QUERY_INTERVALS,
  type SettingsLayeredScope,
  type SettingsMCPServerListFilter,
  type SettingsMCPServerEntry,
  useSettingsMCPServers,
  useSettingsMCPServer,
  type SettingsMCPServerGetFilter,
} from "@/systems/settings";
import { useActiveWorkspace } from "@/systems/workspace";

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
  scope?: SettingsLayeredScope,
  workspaceId?: string,
  profileName?: string,
  liveDataEnabled = true
) {
  const { activeWorkspaceId } = useActiveWorkspace();
  const resolvedWorkspaceId = workspaceId ?? activeWorkspaceId ?? undefined;
  const resolvedScope = scope ?? (resolvedWorkspaceId ? "workspace" : "user");
  const queryFilter: SettingsMCPServerListFilter =
    resolvedScope === "workspace"
      ? { scope: "workspace", workspace_id: resolvedWorkspaceId }
      : resolvedScope === "profile" && profileName
        ? { scope: "profile", profile: profileName, workspace_id: resolvedWorkspaceId }
        : { scope: "user" };
  const queryEnabled =
    entry.installed &&
    (resolvedScope === "user" ||
      (resolvedScope === "profile" ? Boolean(profileName) : Boolean(resolvedWorkspaceId)));
  const query = useSettingsMCPServers(queryFilter, {
    enabled: queryEnabled && liveDataEnabled,
    refetchInterval: SETTINGS_QUERY_INTERVALS.collectionRefetchInterval,
  });
  const server = findInstalledMCPServer(entry, query.data?.mcp_servers ?? []);
  return { query, queryEnabled, queryFilter, server };
}

export { findInstalledMCPServer, useMarketplaceDetailMCPServer };

/** Uses the published instance identity rather than the active window's scope. */
export function marketplaceExtensionMCPFilter(
  server: MarketplaceExtensionServer | undefined
): SettingsMCPServerGetFilter | null {
  const owner = server?.owner?.trim();
  if (
    !server?.name.trim() ||
    !server.runtime_name?.trim() ||
    !owner?.startsWith("extension:") ||
    owner.slice("extension:".length).trim() === ""
  )
    return null;
  const profile = server.profile?.trim();
  const workspaceId = server.workspace_id?.trim();
  if (server.scope !== "global" && server.scope !== "workspace") return null;
  if (server.scope === "workspace" && !workspaceId) return null;
  if (profile && profile !== "default") {
    return {
      scope: "profile",
      profile,
      owner,
      ...(workspaceId ? { workspace_id: workspaceId } : {}),
    };
  }
  if (server.scope === "workspace") return { scope: "workspace", workspace_id: workspaceId, owner };
  if (server.scope === "global") return { scope: "user", owner };
  return null;
}

export function useMarketplaceExtensionMCPServer(
  published: MarketplaceExtensionServer | undefined,
  liveDataEnabled = true,
  refetchInterval = SETTINGS_QUERY_INTERVALS.collectionRefetchInterval
) {
  const filter = marketplaceExtensionMCPFilter(published);
  const query = useSettingsMCPServer(published?.name ?? "", filter ?? {}, {
    enabled: filter !== null && liveDataEnabled,
    refetchInterval,
  });
  return { query, server: filter === null ? undefined : query.data?.server, filter };
}
