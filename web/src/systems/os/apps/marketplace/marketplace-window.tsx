import { useNavigate } from "@tanstack/react-router";

import { useDesktop } from "../../hooks/use-desktop";
import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import { MarketplaceDetailLocation } from "./marketplace-detail-location";
import { parseMarketplaceWindowLocation } from "./marketplace-window-location";
import {
  MarketplaceDetailNotFound,
  MarketplaceInstalledPage,
  MarketplacePage,
} from "@/systems/marketplace";

const DEFAULT_MARKETPLACE_ROUTE = { pathname: "/marketplace", search: {} } as const;

/**
 * Marketplace app controller driven exclusively by the logical window's WM location:
 * `/marketplace` (Browse) · `/marketplace/installed` · `/marketplace/$entryId` (detail).
 */
export function MarketplaceWindow({ windowId }: { windowId: string }) {
  const navigate = useNavigate();
  const location = useDesktop(state => state.windows[windowId]?.route ?? DEFAULT_MARKETPLACE_ROUTE);
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const parsed = parseMarketplaceWindowLocation(location);

  switch (parsed.kind) {
    case "not-found":
      return (
        <MarketplaceDetailNotFound
          onBack={() => void navigate({ to: "/marketplace", search: parsed.search })}
        />
      );
    case "installed":
      return <MarketplaceInstalledPage liveDataEnabled={liveDataEnabled} search={parsed.search} />;
    case "detail":
      return (
        <MarketplaceDetailLocation
          entryId={parsed.entryId}
          liveDataEnabled={liveDataEnabled}
          search={parsed.search}
        />
      );
    case "browse":
      return <MarketplacePage liveDataEnabled={liveDataEnabled} search={parsed.search} />;
  }
}
