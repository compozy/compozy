import { useRef } from "react";
import { toast } from "@compozy/ui";
import { installedDisplayName } from "../lib/marketplace-installed-view";
import { useExtensionInventory } from "@/systems/extensions";

import { useMarketplaceCatalog, useMarketplaceSections } from "./use-marketplace";
import { useRefreshMarketplaceCatalog } from "./use-marketplace-actions";

/**
 * Browse reads: with no search, the daemon's sections (Featured, then non-empty categories, with
 * their full counts); with a search, the flat catalog list. The complete installed inventory owns
 * the shelf, updates and each row's setup state.
 */
export function useMarketplacePage(query = "", liveDataEnabled = true) {
  const inventory = useExtensionInventory(liveDataEnabled);
  const searching = query.trim() !== "";
  const scope = { workspaceId: inventory.workspaceId, profileName: inventory.profileName };
  const catalog = useMarketplaceCatalog(
    { ...scope, q: query, limit: 100 },
    liveDataEnabled && searching
  );
  const home = useMarketplaceSections({ ...scope, perSection: 6 }, liveDataEnabled && !searching);
  const refresh = useRefreshMarketplaceCatalog();
  const lastFailureToast = useRef(Number.NEGATIVE_INFINITY);
  const reportRefreshFailure = (error: unknown) => {
    const now = Date.now();
    if (now - lastFailureToast.current >= 5_000) {
      lastFailureToast.current = now;
      toast.error(error instanceof Error ? error.message : "Failed to refresh the marketplace");
    }
  };
  const refreshCatalog = () =>
    refresh.mutateAsync().then(response => {
      const failed = response.sources.filter(result => result.error_class);
      if (failed.length > 0) {
        reportRefreshFailure(
          new Error(
            `Could not refresh ${failed.map(result => `${result.source} (${result.error_class})`).join(", ")}`
          )
        );
      }
    }, reportRefreshFailure);
  const pages = searching ? (catalog.data?.pages ?? []) : [];
  const first = pages[0];
  const catalogItems = pages.flatMap(page => page.items);
  const sections = searching ? [] : (home.data?.sections ?? []);
  const envelope = searching ? first : home.data;
  const hasItems = searching ? catalogItems.length > 0 : sections.length > 0;
  const unavailable =
    envelope?.error_class && !hasItems
      ? new Error(envelope.error || `Catalog unavailable (${envelope.error_class})`)
      : null;
  const reader = searching ? catalog : home;
  const updates = inventory.data.filter(item => item.updateAvailable);
  return {
    catalogItems,
    /** Featured first, then categories in taxonomy order; the daemon omits empty ones. */
    sections,
    hasItems,
    installedItems: inventory.data,
    installedCount: inventory.data.length,
    updates,
    // Every entry has exactly one category, so the category totals are the catalog total.
    total: searching
      ? (first?.total ?? 0)
      : sections.reduce((sum, section) => sum + (section.id === "featured" ? 0 : section.total), 0),
    sources: envelope?.sources ?? [],
    revision: envelope?.revision,
    stale: searching ? pages.some(page => page.stale) : Boolean(home.data?.stale),
    diagnostic: envelope?.error,
    catalogError: reader.error ?? unavailable,
    /** Only the daemon's own source report is "unreachable"; a failed read is a load failure. */
    catalogUnreachable: !reader.error && unavailable !== null,
    installedError: inventory.error,
    // An edited search keeps the previous results on screen; a placeholder with none is no answer.
    isLoading:
      reader.isPending ||
      (reader.isPlaceholderData && !hasItems) ||
      Boolean(envelope?.refreshing && !hasItems),
    isInstalledLoading: inventory.isPending,
    isRefreshing: refresh.isPending || (reader.isRefetching && !reader.isPlaceholderData),
    refresh: refreshCatalog,
    profileName: inventory.profileName,
    workspaceId: inventory.workspaceId,
  };
}

/** Installed search never depends on fetching every catalog page. */
export function useMarketplaceInstalledPage(query = "", liveDataEnabled = true) {
  const inventory = useExtensionInventory(liveDataEnabled);
  const needle = query.trim().normalize("NFC").toLocaleLowerCase();
  const items = inventory.data
    .filter(item =>
      [
        item.extension.name,
        item.extension.description,
        item.listing?.name,
        item.listing?.description,
      ].some(value => value?.normalize("NFC").toLocaleLowerCase().includes(needle))
    )
    .sort((left, right) => installedDisplayName(left).localeCompare(installedDisplayName(right)));
  return {
    ...inventory,
    items,
    installedCount: inventory.data.length,
    updates: inventory.data.filter(item => item.updateAvailable),
  };
}
