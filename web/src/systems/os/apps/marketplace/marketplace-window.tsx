import { useDesktop } from "../../hooks/use-desktop";
import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import { MarketplaceDetailLocation } from "./marketplace-detail-location";
import { validateMarketplaceDetailSearch } from "./marketplace-detail-search";
import {
  isRetiredMarketplaceDetailKind,
  isRetiredMarketplaceKindSegment,
  MarketplaceInstalledPage,
  MarketplacePage,
  validateMarketplaceSearch,
} from "@/systems/marketplace";

const DEFAULT_MARKETPLACE_ROUTE = { pathname: "/marketplace", search: {} } as const;

function decodePathSegment(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

/**
 * Marketplace app controller driven exclusively by the logical window's WM location:
 * `/marketplace` (Browse) · `/marketplace/installed` · `/marketplace/$entryId` (detail).
 * Retired kind locations still held by persisted layouts resolve to the same three views for
 * one release, mirroring the route-layer redirects.
 */
export function MarketplaceWindow({ windowId }: { windowId: string }) {
  const location = useDesktop(state => state.windows[windowId]?.route ?? DEFAULT_MARKETPLACE_ROUTE);
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const segments = location.pathname.split("/").filter(Boolean);
  const [, second, third] = segments;

  if (second === "installed") {
    return (
      <MarketplaceInstalledPage
        liveDataEnabled={liveDataEnabled}
        search={validateMarketplaceSearch(location.search)}
      />
    );
  }

  if (third && isRetiredMarketplaceDetailKind(second)) {
    return (
      <MarketplaceDetailLocation
        entryId={decodePathSegment(third)}
        liveDataEnabled={liveDataEnabled}
        search={validateMarketplaceDetailSearch(location.search)}
      />
    );
  }

  if (second && !isRetiredMarketplaceKindSegment(second)) {
    return (
      <MarketplaceDetailLocation
        entryId={decodePathSegment(second)}
        liveDataEnabled={liveDataEnabled}
        search={validateMarketplaceDetailSearch(location.search)}
      />
    );
  }

  return (
    <MarketplacePage
      liveDataEnabled={liveDataEnabled}
      search={validateMarketplaceSearch(location.search)}
    />
  );
}
