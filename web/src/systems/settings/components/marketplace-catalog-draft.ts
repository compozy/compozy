import type { SettingsMarketplaceSection } from "../types";

export type SettingsMarketplaceCatalogConfig = SettingsMarketplaceSection["config"];

/** A field left blank cannot be saved; the daemon owns every other rule for these values. */
export function marketplaceCatalogDraftInvalid(draft: SettingsMarketplaceCatalogConfig): boolean {
  return [draft.base_url, draft.ttl, draft.timeout].some(value => value.trim() === "");
}

export function sameMarketplaceCatalogConfig(
  left: SettingsMarketplaceCatalogConfig,
  right: SettingsMarketplaceCatalogConfig
): boolean {
  return (
    left.base_url === right.base_url && left.ttl === right.ttl && left.timeout === right.timeout
  );
}
