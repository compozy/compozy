import type { MarketplaceDetailSearch } from "../marketplace-detail-search";
import { PERMANENT_PROFILE, useProfileReadScope } from "@/systems/profiles";
import { useActiveWorkspace } from "@/systems/workspace";

export function useMarketplaceDetailScope(search: MarketplaceDetailSearch) {
  const { activeWorkspaceId } = useActiveWorkspace();
  const { destination } = useProfileReadScope();
  const requestedProfile =
    search.scope === "profile" ? search.profile?.trim() || null : destination;
  const profileName = requestedProfile === PERMANENT_PROFILE ? null : requestedProfile;
  const workspaceId = search.scope === "user" ? null : (search.workspace_id ?? activeWorkspaceId);
  return {
    managementScope: search.scope ?? (profileName ? "profile" : workspaceId ? "workspace" : "user"),
    profileName,
    workspaceId,
  } as const;
}
