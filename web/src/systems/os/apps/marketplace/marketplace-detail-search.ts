export interface MarketplaceDetailSearch {
  installed_name?: string;
  scope?: "user" | "profile" | "workspace";
  profile?: string;
  tab?: "market";
  workspace_id?: string;
}

export function validateMarketplaceDetailSearch(
  search: Record<string, unknown>
): MarketplaceDetailSearch {
  const scope =
    search.scope === "user" || search.scope === "profile" || search.scope === "workspace"
      ? search.scope
      : undefined;
  const workspaceId =
    scope !== "user" && typeof search.workspace_id === "string"
      ? search.workspace_id.trim() || undefined
      : undefined;
  const installedName =
    typeof search.installed_name === "string"
      ? search.installed_name.trim() || undefined
      : undefined;
  const profile =
    scope === "profile" && typeof search.profile === "string"
      ? search.profile.trim() || undefined
      : undefined;
  return {
    installed_name: installedName,
    scope,
    profile,
    tab: search.tab === "market" ? "market" : undefined,
    workspace_id: workspaceId,
  };
}
