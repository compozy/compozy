import { useMarketplacePage } from "../hooks/use-marketplace-page";
import type { MarketplaceSearch } from "../lib/marketplace-search";
import { useAddMarketplaceDialog } from "./use-add-marketplace-dialog";
import { useExtensionInstallDialog } from "./use-extension-install-dialog";
import { useMarketplaceActionController } from "./use-marketplace-action-controller";
import { useMarketplaceSearchStrip } from "./use-marketplace-search-strip";

/** Browse behavior: route query → catalog page, row actions, both Add ▾ flows, the strip. */
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
  const addMarketplace = useAddMarketplaceDialog();
  return { actions, addMarketplace, install, page, query, strip };
}
