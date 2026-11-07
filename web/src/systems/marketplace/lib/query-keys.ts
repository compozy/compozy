import type {
  MarketplaceCatalogOptions,
  MarketplaceCatalogEntryOptions,
  MarketplaceSectionsOptions,
} from "../types";

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
  sources: () => [...marketplaceKeys.all, "sources"] as const,
  sourcePreview: (ref: string, name?: string) =>
    [...marketplaceKeys.all, "source-preview", ref.trim(), name?.trim() ?? ""] as const,
  catalog: (options: MarketplaceCatalogOptions = {}) =>
    [
      ...marketplaceKeys.all,
      "catalog",
      ...scopeIdentity(options.workspaceId),
      normalizeText(options.profileName) ?? "default",
      normalizeText(options.q),
      options.limit ?? 100,
      normalizeText(options.category),
    ] as const,
  sections: (options: MarketplaceSectionsOptions = {}) =>
    [
      ...marketplaceKeys.all,
      "sections",
      ...scopeIdentity(options.workspaceId),
      normalizeText(options.profileName) ?? "default",
      normalizeText(options.q),
      options.perSection ?? 6,
    ] as const,
  /** One category page, paged by cursor (`pageParam`); `q` is scoped to the category. */
  category: (options: MarketplaceCatalogOptions) =>
    [
      ...marketplaceKeys.all,
      "category",
      normalizeText(options.category),
      ...scopeIdentity(options.workspaceId),
      normalizeText(options.profileName) ?? "default",
      normalizeText(options.q),
      options.limit ?? 50,
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
