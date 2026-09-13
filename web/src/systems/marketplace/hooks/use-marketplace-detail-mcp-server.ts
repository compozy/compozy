import type { MarketplaceExtensionServer } from "../types";
import {
  SETTINGS_QUERY_INTERVALS,
  useSettingsMCPServer,
  type SettingsMCPServerGetFilter,
} from "@/systems/settings";

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
