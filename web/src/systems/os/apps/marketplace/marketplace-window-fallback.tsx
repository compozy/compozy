import {
  ToolbarSearchSkeleton,
  ToolbarSlotSkeleton,
} from "../../components/os-window-toolbar-skeleton";
import type { OsWindowRoute } from "../../lib/os-types";
import { parseMarketplaceWindowLocation } from "./marketplace-window-location";
import {
  MarketplaceBrowseSkeleton,
  MarketplaceDetailSkeleton,
  MarketplaceInstalledSkeleton,
} from "@/systems/marketplace";

/**
 * The marketplace window's body while its code loads. An entry shows its detail frame; Browse and
 * Installed publish the search-only strip their pages do, over their loading cards. A route
 * that names no surface shows Browse, which is where the window's not-found view leads.
 */
export function MarketplaceWindowFallback({ route }: { route: OsWindowRoute }) {
  const location = parseMarketplaceWindowLocation(route);
  if (location.kind === "detail") return <MarketplaceDetailSkeleton />;
  return (
    <>
      <ToolbarSlotSkeleton leading={<ToolbarSearchSkeleton className="w-full max-w-md" />} />
      {location.kind === "installed" ? (
        <MarketplaceInstalledSkeleton />
      ) : (
        <MarketplaceBrowseSkeleton />
      )}
    </>
  );
}
