export interface MarketplaceDetailSearch {
  installed_name?: string;
  scope?: "global" | "workspace";
  workspace_id?: string;
}

export function validateMarketplaceDetailSearch(
  search: Record<string, unknown>
): MarketplaceDetailSearch {
  const scope =
    search.scope === "global" || search.scope === "workspace" ? search.scope : undefined;
  const workspaceId =
    scope === "workspace" && typeof search.workspace_id === "string"
      ? search.workspace_id.trim() || undefined
      : undefined;
  const installedName =
    typeof search.installed_name === "string"
      ? search.installed_name.trim() || undefined
      : undefined;
  return { installed_name: installedName, scope, workspace_id: workspaceId };
}
