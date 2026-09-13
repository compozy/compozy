import { useRef } from "react";
import { toast } from "sonner";
import { installedDisplayName } from "../lib/marketplace-installed-view";
import { useExtensionInventory } from "@/systems/extensions";

import { useMarketplaceCatalog } from "./use-marketplace";
import { useRefreshMarketplaceCatalog } from "./use-marketplace-actions";

/** The API owns catalog filtering/paging; the complete installed inventory owns its shelf and updates. */
export function useMarketplacePage(query = "", liveDataEnabled = true) {
  const inventory = useExtensionInventory(liveDataEnabled);
  const catalog = useMarketplaceCatalog(
    {
      q: query,
      limit: 100,
      workspaceId: inventory.workspaceId,
      profileName: inventory.profileName,
    },
    liveDataEnabled
  );
  const refresh = useRefreshMarketplaceCatalog();
  const lastFailureToast = useRef(Number.NEGATIVE_INFINITY);
  const refreshCatalog = async () => {
    try {
      const response = await refresh.mutateAsync();
      const failed = response.sources.find(result => result.error_class);
      if (failed) throw new Error(`Could not refresh the catalog (${failed.error_class})`);
    } catch (error) {
      const now = Date.now();
      if (now - lastFailureToast.current >= 5_000) {
        lastFailureToast.current = now;
        toast.error(error instanceof Error ? error.message : "Failed to refresh the marketplace");
      }
    }
  };
  const pages = catalog.data?.pages ?? [];
  const first = pages[0];
  const catalogItems = pages.flatMap(page => page.items);
  const unavailable =
    first?.error_class && catalogItems.length === 0
      ? new Error(first.error || `Catalog unavailable (${first.error_class})`)
      : null;
  const updates = inventory.data.filter(item => item.updateAvailable);
  return {
    catalogItems,
    installedItems: inventory.data,
    installedCount: inventory.data.length,
    updates,
    total: first?.total ?? 0,
    sources: first?.sources ?? [],
    revision: first?.revision,
    stale: pages.some(page => page.stale),
    diagnostic: first?.error,
    catalogError: catalog.error ?? unavailable,
    installedError: inventory.error,
    isLoading: catalog.isPending,
    isInstalledLoading: inventory.isPending,
    isRefreshing: refresh.isPending || catalog.isRefetching,
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
      [item.extension.name, item.listing?.name, item.listing?.description].some(value =>
        value?.normalize("NFC").toLocaleLowerCase().includes(needle)
      )
    )
    .sort((left, right) => installedDisplayName(left).localeCompare(installedDisplayName(right)));
  return {
    ...inventory,
    items,
    installedCount: inventory.data.length,
    updates: inventory.data.filter(item => item.updateAvailable),
  };
}
