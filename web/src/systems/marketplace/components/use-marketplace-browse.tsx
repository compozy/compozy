import { useMarketplacePage } from "../hooks/use-marketplace-page";
import { marketplaceKind, type MarketplaceSearch } from "../lib/marketplace-search";
import { useAddMarketplaceDialog } from "./use-add-marketplace-dialog";
import { useExtensionInstallDialog } from "./use-extension-install-dialog";
import { useMarketplaceActionController } from "./use-marketplace-action-controller";
import { useMarketplaceSearchStrip } from "./use-marketplace-search-strip";
import { useMCPServerManagement } from "@/systems/settings";

/**
 * Browse behavior: route kind + query → catalog page (Plugins) or the managed MCP server list
 * (MCPs), row actions, every Add ▾ flow, the strip. Only the shown kind reads live data.
 */
export function useMarketplaceBrowse(search: MarketplaceSearch, liveDataEnabled: boolean) {
  const query = search.q ?? "";
  const kind = marketplaceKind(search);
  const strip = useMarketplaceSearchStrip({
    kind,
    label: kind === "mcps" ? "Search MCP servers" : "Search extensions",
    placeholder: kind === "mcps" ? "Search MCP servers…" : "Search extensions…",
    query,
    testId: "marketplace-search",
    to: "/marketplace",
  });
  const page = useMarketplacePage(query, liveDataEnabled && kind === "plugins");
  const mcp = useMCPServerManagement({
    enabled: liveDataEnabled && kind === "mcps",
    search: {
      query,
      setQuery: value =>
        void strip.navigate({
          replace: true,
          search: { kind: "mcps", q: value.trim() || undefined },
          to: "/marketplace",
        }),
    },
  });
  const actions = useMarketplaceActionController();
  const install = useExtensionInstallDialog({
    onInstalled: () => void strip.navigate({ search: {}, to: "/marketplace/installed" }),
  });
  const addMarketplace = useAddMarketplaceDialog();
  /** Add ▾ › Add MCP server… lands on the MCPs view with the editor open over it. */
  const addMCPServer = () => {
    if (kind !== "mcps") strip.switchKind("mcps");
    mcp.openCreate();
  };
  return { actions, addMarketplace, addMCPServer, install, kind, mcp, page, query, strip };
}
