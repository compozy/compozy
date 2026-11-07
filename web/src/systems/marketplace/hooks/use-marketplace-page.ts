import { useRef } from "react";
import { toast } from "@compozy/ui";
import { marketplaceCatalogUnreachable } from "../lib/catalog-unreachable";
import { installedDisplayName } from "../lib/marketplace-installed-view";
import type {
  MarketplaceCatalogListing,
  MarketplaceCatalogResponse,
  MarketplaceSection,
  MarketplaceSectionsResponse,
} from "../types";
import { useExtensionInventory } from "@/systems/extensions";

import { useMarketplaceCatalog, useMarketplaceSections } from "./use-marketplace";
import { useRefreshMarketplaceCatalog } from "./use-marketplace-actions";

type MarketplaceCatalogRead = ReturnType<typeof useMarketplaceCatalog>;
type MarketplaceSectionsRead = ReturnType<typeof useMarketplaceSections>;

interface MarketplaceBrowseView {
  catalogItems: MarketplaceCatalogListing[];
  sections: MarketplaceSection[];
  envelope: MarketplaceCatalogResponse | MarketplaceSectionsResponse | undefined;
  hasItems: boolean;
  total: number;
  stale: boolean;
  reader: MarketplaceCatalogRead | MarketplaceSectionsRead;
}

/** Refresh every source; a source that failed is reported once per five seconds, not per call. */
function useMarketplaceRefresh() {
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
  return { refresh: refreshCatalog, isPending: refresh.isPending };
}

/** The flat search answer. */
function searchView(catalog: MarketplaceCatalogRead): MarketplaceBrowseView {
  const pages = catalog.data?.pages ?? [];
  const first = pages[0];
  const catalogItems = pages.flatMap(page => page.items);
  return {
    catalogItems,
    sections: [],
    envelope: first,
    hasItems: catalogItems.length > 0,
    total: first?.total ?? 0,
    stale: pages.some(page => page.stale),
    reader: catalog,
  };
}

/** The sectioned Browse answer. */
function sectionsView(home: MarketplaceSectionsRead): MarketplaceBrowseView {
  const sections = home.data?.sections ?? [];
  return {
    catalogItems: [],
    sections,
    envelope: home.data,
    hasItems: sections.length > 0,
    // Every entry has exactly one category, so the category totals are the catalog total.
    total: sections.reduce(
      (sum, section) => sum + (section.id === "featured" ? 0 : section.total),
      0
    ),
    stale: Boolean(home.data?.stale),
    reader: home,
  };
}

/** Loading and refreshing flags of the active read. */
function readStatus(
  reader: MarketplaceCatalogRead | MarketplaceSectionsRead,
  hasItems: boolean,
  refreshing: boolean
) {
  return {
    // An edited search keeps the previous results on screen; a placeholder with none is no answer.
    isLoading:
      reader.isPending || (reader.isPlaceholderData && !hasItems) || (refreshing && !hasItems),
    isRefetching: reader.isRefetching && !reader.isPlaceholderData,
  };
}

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
  const refresh = useMarketplaceRefresh();
  const view = searching ? searchView(catalog) : sectionsView(home);
  const { envelope, hasItems, reader } = view;
  const unavailable = marketplaceCatalogUnreachable(envelope, hasItems);
  const status = readStatus(reader, hasItems, Boolean(envelope?.refreshing));
  return {
    catalogItems: view.catalogItems,
    /** Featured first, then categories in taxonomy order; the daemon omits empty ones. */
    sections: view.sections,
    hasItems,
    installedItems: inventory.data,
    installedCount: inventory.data.length,
    updates: inventory.data.filter(item => item.updateAvailable),
    total: view.total,
    sources: envelope?.sources ?? [],
    revision: envelope?.revision,
    stale: view.stale,
    diagnostic: envelope?.error,
    catalogError: reader.error ?? unavailable,
    /** Only the daemon's own source report is "unreachable"; a failed read is a load failure. */
    catalogUnreachable: !reader.error && unavailable !== null,
    installedError: inventory.error,
    isLoading: status.isLoading,
    isInstalledLoading: inventory.isPending,
    isRefreshing: refresh.isPending || status.isRefetching,
    refresh: refresh.refresh,
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
