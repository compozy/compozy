import { useExtensionInventory } from "@/systems/extensions";

import { MarketplaceApiError } from "../adapters/marketplace-api-error";
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
  const unknownCategory =
    reader.error instanceof MarketplaceApiError &&
    reader.error.status === 400 &&
    reader.error.diagnosticCode === "marketplace_category_unknown";
  return {
    items,
    label,
    total: first?.total ?? 0,
    stale: pages.some(page => page.stale),
    unknownCategory,
    error: unknownCategory ? null : reader.error,
    // A placeholder or a refreshing catalog with no rows is not an answer yet.
    isLoading:
      reader.isPending ||
      (items.length === 0 && (reader.isPlaceholderData || pages.some(page => page.refreshing))),
    hasMore: Boolean(reader.hasNextPage),
    isLoadingMore: reader.isFetchingNextPage,
    loadMore: reader.loadMore,
    retry: () => void reader.refetch(),
    /** Retry under the list: the next page when that failed, otherwise the whole read again. */
    retryMore: () => void (reader.isFetchNextPageError ? reader.loadMore() : reader.refetch()),
    inventory: { items: inventory.data, pending: inventory.isPending },
  };
}
