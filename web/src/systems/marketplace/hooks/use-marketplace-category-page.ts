import { useExtensionInventory } from "@/systems/extensions";

import { MarketplaceApiError } from "../adapters/marketplace-api-error";
import { marketplaceCatalogUnreachable } from "../lib/catalog-unreachable";
import { useMarketplaceCategory } from "./use-marketplace";

/**
 * One category page: the daemon pages, sorts and scopes `q` to the category; the title and count
 * come from the response's own category counts, never from a hardcoded taxonomy or the rows shown.
 */
export function useMarketplaceCategoryPage(category: string, query = "", liveDataEnabled = true) {
  const inventory = useExtensionInventory(liveDataEnabled);
  const reader = useMarketplaceCategory(
    {
      category,
      q: query,
      limit: 50,
      workspaceId: inventory.workspaceId,
      profileName: inventory.profileName,
    },
    liveDataEnabled
  );
  const pages = reader.data?.pages ?? [];
  const first = pages[0];
  const items = pages.flatMap(page => page.items);
  const label = first?.categories.find(count => count.id === category)?.label;
  // A blank id never reaches the daemon (the read stays disabled), so it is unknown here.
  const unknownCategory =
    category.trim() === "" ||
    (reader.error instanceof MarketplaceApiError &&
      reader.error.status === 400 &&
      reader.error.diagnosticCode === "marketplace_category_unknown");
  const failed = unknownCategory ? null : reader.error;
  // The daemon answered but no source did; one degraded source leaves an empty result empty.
  const unavailable = failed ? null : marketplaceCatalogUnreachable(first, items.length > 0);
  return {
    items,
    label,
    total: first?.total ?? 0,
    stale: pages.some(page => page.stale),
    unknownCategory,
    error: failed ?? unavailable,
    /** Only the daemon's own source report is "unreachable"; a failed read is a load failure. */
    unreachable: unavailable !== null,
    // A placeholder or a refreshing catalog with no rows is not an answer yet; a failed read is,
    // so its error and Retry replace the skeleton.
    isLoading:
      !unknownCategory &&
      (reader.isPending ||
        (!reader.error &&
          items.length === 0 &&
          (reader.isPlaceholderData || pages.some(page => page.refreshing)))),
    hasMore: Boolean(reader.hasNextPage),
    // A stale-cursor restart holds Load more until the fresh pages land.
    isLoadingMore: reader.isFetchingNextPage || reader.isRestarting,
    loadMore: reader.loadMore,
    retry: () => void reader.refetch(),
    /** Retry under the list: the next page when that failed, otherwise the whole read again. */
    retryMore: () => void (reader.isFetchNextPageError ? reader.loadMore() : reader.refetch()),
    inventory: { items: inventory.data, pending: inventory.isPending },
  };
}
