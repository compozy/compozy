import type { MarketplaceInstalledItem } from "./marketplace-installed-items";
import type { MarketplaceListing } from "../types";

function updateIdentity(entry: MarketplaceListing): string {
  return `${entry.kind}\u0000${entry.installed_name?.trim() || entry.entry_id}`;
}

/**
 * Updates surface in two projections: the installed inventory (daemon-owned `update_available`)
 * and the catalog page (a listing already installed on this daemon). Counting the union of
 * identities keeps the landing count non-zero on the market scope without double-counting an item
 * that appears in both.
 */
export function countMarketplaceUpdates(input: {
  installedItems: readonly MarketplaceInstalledItem[];
  marketItems: readonly MarketplaceListing[];
}): number {
  const identities = new Set<string>();
  for (const item of input.installedItems) {
    if (item.entry.update_available) identities.add(updateIdentity(item.entry));
  }
  for (const entry of input.marketItems) {
    if (entry.update_available) identities.add(updateIdentity(entry));
  }
  return identities.size;
}
