import { useMarketplaceInstalledPage } from "../hooks/use-marketplace-page";
import type { MarketplaceSearch } from "../lib/marketplace-search";
import { useExtensionInstallDialog } from "./use-extension-install-dialog";
import { useMarketplaceActionController } from "./use-marketplace-action-controller";
import { useMarketplaceSearchStrip } from "./use-marketplace-search-strip";

/** Installed behavior: route query → complete inventory filter, row actions, Add ▾, the strip. */
export function useMarketplaceInstalled(search: MarketplaceSearch, liveDataEnabled: boolean) {
  const query = search.q ?? "";
  const strip = useMarketplaceSearchStrip({
    label: "Search installed",
    placeholder: "Search installed…",
    query,
    testId: "marketplace-installed-search",
    to: "/marketplace/installed",
  });
  const page = useMarketplaceInstalledPage(query, liveDataEnabled);
  const actions = useMarketplaceActionController();
  const install = useExtensionInstallDialog();
  const backToBrowse = () =>
    void strip.navigate({ search: { q: query || undefined }, to: "/marketplace" });
  return { actions, backToBrowse, install, page, query, strip };
}
