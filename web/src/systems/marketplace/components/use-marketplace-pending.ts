import { useState } from "react";

import type { InstalledExtensionView } from "@/systems/extensions";

import { installedExtensionKey, marketplaceOriginKey } from "../lib/marketplace-installed-view";
import type { MarketplaceCatalogListing } from "../types";

/**
 * Pending and just-landed rows keyed by origin (catalog) or local installed name (Installed view).
 * Names never join the two: a row is pending only for the identity the mutation addressed.
 */
function useMarketplacePending() {
  const [pendingKeys, setPendingKeys] = useState<ReadonlyMap<string, number>>(() => new Map());
  const [flashKeys, setFlashKeys] = useState<ReadonlySet<string>>(() => new Set());

  const track = async <T>(keys: readonly string[], action: () => Promise<T>): Promise<T> => {
    setPendingKeys(current => {
      const next = new Map(current);
      for (const key of keys) next.set(key, (next.get(key) ?? 0) + 1);
      return next;
    });
    return Promise.resolve()
      .then(action)
      .finally(() => {
        setPendingKeys(current => {
          const next = new Map(current);
          for (const key of keys) {
            const remaining = (next.get(key) ?? 1) - 1;
            if (remaining > 0) next.set(key, remaining);
            else next.delete(key);
          }
          return next;
        });
      });
  };

  const flash = (key: string) => setFlashKeys(current => new Set(current).add(key));
  const endFlash = (key: string) =>
    setFlashKeys(current => {
      const next = new Set(current);
      next.delete(key);
      return next;
    });

  return {
    flashEntry: (entry: MarketplaceCatalogListing) => flash(marketplaceOriginKey(entry)),
    flashItem: (item: InstalledExtensionView) => flash(installedExtensionKey(item)),
    endEntryFlash: (entry: MarketplaceCatalogListing) => endFlash(marketplaceOriginKey(entry)),
    endItemFlash: (item: InstalledExtensionView) => endFlash(installedExtensionKey(item)),
    isEntryFlashing: (entry: MarketplaceCatalogListing) =>
      flashKeys.has(marketplaceOriginKey(entry)),
    isItemFlashing: (item: InstalledExtensionView) => flashKeys.has(installedExtensionKey(item)),
    isEntryPending: (entry: MarketplaceCatalogListing) =>
      (pendingKeys.get(marketplaceOriginKey(entry)) ?? 0) > 0,
    isItemPending: (item: InstalledExtensionView) =>
      (pendingKeys.get(installedExtensionKey(item)) ?? 0) > 0,
    trackEntry: <T>(entry: MarketplaceCatalogListing, action: () => Promise<T>) =>
      track([marketplaceOriginKey(entry)], action),
    trackItem: <T>(item: InstalledExtensionView, action: () => Promise<T>) =>
      track([installedExtensionKey(item)], action),
    trackItems: <T>(items: readonly InstalledExtensionView[], action: () => Promise<T>) =>
      track(items.map(installedExtensionKey), action),
  };
}

export { useMarketplacePending };
