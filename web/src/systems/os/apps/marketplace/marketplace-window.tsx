import { useDesktop } from "../../hooks/use-desktop";
import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import { MarketplaceDetailLocation } from "./marketplace-detail-location";
import { validateMarketplaceDetailSearch } from "./marketplace-detail-search";
import {
  isMarketplaceKind,
  isMarketplaceRouteKind,
  marketplaceApiKindFor,
  MarketplaceKindPage,
  validateMarketplaceKindSearch,
} from "@/systems/marketplace";

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
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const segments = location.pathname.split("/").filter(Boolean);

  if (isMarketplaceKind(segments[1]) && segments[2]) {
    return (
      <MarketplaceDetailLocation
        entryId={decodePathSegment(segments[2])}
        kind={segments[1]}
        liveDataEnabled={liveDataEnabled}
        search={validateMarketplaceDetailSearch(location.search)}
      />
    );
  }

  const routeKind = isMarketplaceRouteKind(segments[1]) ? segments[1] : "skills";
  return (
    <MarketplaceKindPage
      kind={marketplaceApiKindFor(routeKind)}
      liveDataEnabled={liveDataEnabled}
      search={validateMarketplaceKindSearch(location.search)}
    />
  );
}
