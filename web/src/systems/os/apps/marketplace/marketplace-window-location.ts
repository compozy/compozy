import type { OsWindowRoute } from "../../lib/os-types";
import {
  validateMarketplaceDetailSearch,
  type MarketplaceDetailSearch,
} from "./marketplace-detail-search";
import { validateMarketplaceSearch, type MarketplaceSearch } from "@/systems/marketplace";

/** The surface a Marketplace window route names; the window and its loading fallback read it alike. */
export type MarketplaceWindowLocation =
  | { kind: "not-found"; search: MarketplaceSearch }
  | { kind: "installed"; search: MarketplaceSearch }
  | { kind: "detail"; entryId: string; search: MarketplaceDetailSearch }
  | { kind: "browse"; search: MarketplaceSearch };

function decodePathSegment(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

/** `/marketplace` (Browse) · `/marketplace/installed` · `/marketplace/$entryId` (detail). */
export function parseMarketplaceWindowLocation(location: OsWindowRoute): MarketplaceWindowLocation {
  const segments = location.pathname.split("/").filter(Boolean);
  const [, second] = segments;

  if (segments[0] !== "marketplace" || segments.length > 2) {
    return { kind: "not-found", search: validateMarketplaceSearch(location.search) };
  }
  if (second === "installed") {
    return { kind: "installed", search: validateMarketplaceSearch(location.search) };
  }
  if (second) {
    return {
      kind: "detail",
      entryId: decodePathSegment(second),
      search: validateMarketplaceDetailSearch(location.search),
    };
  }
  return { kind: "browse", search: validateMarketplaceSearch(location.search) };
}
