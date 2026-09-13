export interface MarketplaceDetailSearch {
  installed_name?: string;
  scope?: "user" | "profile" | "workspace";
  profile?: string;
  /** Catalog source name (`compozy-catalog`); the daemon defaults to the Compozy catalog. */
  source?: string;
  workspace_id?: string;
  /** Where Back returns: Browse by default, Installed when the row came from there. */
  from?: "installed";
  /** Query kept from the referrer so Back lands on the same list. */
  q?: string;
}

function optionalText(value: unknown): string | undefined {
  return typeof value === "string" ? value.trim() || undefined : undefined;
}

export function validateMarketplaceDetailSearch(
  search: Record<string, unknown>
): MarketplaceDetailSearch {
  const scope =
    search.scope === "user" || search.scope === "profile" || search.scope === "workspace"
      ? search.scope
      : undefined;
  const workspaceId = scope !== "user" ? optionalText(search.workspace_id) : undefined;
  const profile = optionalText(search.profile);
  return {
    installed_name: optionalText(search.installed_name),
    scope,
    profile,
    source: optionalText(search.source),
    workspace_id: workspaceId,
    from: search.from === "installed" ? "installed" : undefined,
    q: optionalText(search.q),
  };
}
