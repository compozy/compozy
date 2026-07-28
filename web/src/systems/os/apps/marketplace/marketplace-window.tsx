import { BundleActivationDetail } from "@/systems/extensions";
import {
  isMarketplaceKind,
  isMarketplaceRouteKind,
  marketplaceApiKindFor,
  MarketplaceKindPage,
  validateMarketplaceKindSearch,
} from "@/systems/marketplace";
import { useDesktop } from "../../hooks/use-desktop";
import { MarketplaceDetailLocation } from "./marketplace-detail-location";
import { validateMarketplaceDetailSearch } from "./marketplace-detail-search";

const DEFAULT_MARKETPLACE_ROUTE = { pathname: "/marketplace/skills", search: {} } as const;

function decodePathSegment(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

/** Marketplace app controller driven exclusively by the logical window's WM location. */
export function MarketplaceWindow({ windowId }: { windowId: string }) {
  const location = useDesktop(state => state.windows[windowId]?.route ?? DEFAULT_MARKETPLACE_ROUTE);
  const segments = location.pathname.split("/").filter(Boolean);

  if (segments[1] === "bundles" && segments[2] === "activations" && segments[3]) {
    return <BundleActivationDetail id={decodePathSegment(segments[3])} />;
  }

  if (isMarketplaceKind(segments[1]) && segments[2]) {
    return (
      <MarketplaceDetailLocation
        entryId={decodePathSegment(segments[2])}
        kind={segments[1]}
        search={validateMarketplaceDetailSearch(location.search)}
      />
    );
  }

  const routeKind = isMarketplaceRouteKind(segments[1]) ? segments[1] : "skills";
  return (
    <MarketplaceKindPage
      kind={marketplaceApiKindFor(routeKind)}
      search={validateMarketplaceKindSearch(location.search)}
    />
  );
}
