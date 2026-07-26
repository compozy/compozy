import type {
  MarketplaceEntryOptions,
  MarketplaceKindOptions,
  MarketplaceSearchOptions,
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
  search: (options: MarketplaceSearchOptions = {}) =>
    [
      ...marketplaceKeys.all,
      "search",
      ...scopeIdentity(options.workspaceId),
      normalizeText(options.q),
      options.limit ?? null,
    ] as const,
  kind: (options: MarketplaceKindOptions) =>
    [
      ...marketplaceKeys.all,
      "kind",
      options.kind,
      ...scopeIdentity(options.workspaceId),
      normalizeText(options.q),
      options.limit ?? null,
    ] as const,
  detail: (options: MarketplaceEntryOptions) =>
    [
      ...marketplaceKeys.all,
      "detail",
      options.kind,
      normalizeText(options.entryId),
      options.kind === "bundle" ? null : normalizeText(options.installedName),
      ...scopeIdentity(options.workspaceId),
    ] as const,
};
