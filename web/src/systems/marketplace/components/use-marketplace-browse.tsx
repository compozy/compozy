import { useMarketplacePage } from "../hooks/use-marketplace-page";
import type { MarketplaceSearch } from "../lib/marketplace-search";
import { useExtensionInstallDialog } from "./use-extension-install-dialog";
import { useMarketplaceActionController } from "./use-marketplace-action-controller";
import { useMarketplaceSearchStrip } from "./use-marketplace-search-strip";

/** Browse behavior: route query → catalog page, row actions, the Add ▾ install flow, the strip. */
export function useMarketplaceBrowse(search: MarketplaceSearch, liveDataEnabled: boolean) {
  const query = search.q ?? "";
  const strip = useMarketplaceSearchStrip({
    label: "Search extensions",
    placeholder: "Search extensions…",
    query,
    testId: "marketplace-search",
    to: "/marketplace",
  });
  const page = useMarketplacePage(query, liveDataEnabled);
  const actions = useMarketplaceActionController();
  const install = useExtensionInstallDialog({
    onInstalled: () => void strip.navigate({ search: {}, to: "/marketplace/installed" }),
  });
  return { actions, install, page, query, strip };
}
