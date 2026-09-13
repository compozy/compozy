import type { MarketplaceCatalogOptions, MarketplaceCatalogEntryOptions } from "../types";

function normalizeText(value?: string | null): string | null {
  const normalized = value?.trim();
  return normalized ? normalized : null;
}

function scopeIdentity(
  workspaceId?: string | null
): readonly ["global"] | readonly ["workspace", string] {
  const normalizedWorkspaceId = normalizeText(workspaceId);
  return normalizedWorkspaceId
    ? (["workspace", normalizedWorkspaceId] as const)
    : (["global"] as const);
}

export const marketplaceKeys = {
  all: ["marketplace"] as const,
  catalog: (options: MarketplaceCatalogOptions = {}) =>
    [
      ...marketplaceKeys.all,
      "catalog",
      ...scopeIdentity(options.workspaceId),
      normalizeText(options.profileName) ?? "default",
      normalizeText(options.q),
      options.limit ?? 100,
    ] as const,
  catalogEntry: (options: MarketplaceCatalogEntryOptions) =>
    [
      ...marketplaceKeys.all,
      "catalog-entry",
      normalizeText(options.source),
      normalizeText(options.entryId),
      normalizeText(options.installedName),
      ...scopeIdentity(options.workspaceId),
      normalizeText(options.profileName) ?? "default",
    ] as const,
};
