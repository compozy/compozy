import { useDesktop } from "../../hooks/use-desktop";
import { BridgeDetailLocation } from "./bridge-detail-location";
import { BridgesCatalogLocation } from "./bridges-catalog-location";
import { validateBridgesSearch } from "./use-bridges-page";

const DEFAULT_BRIDGES_ROUTE = { pathname: "/bridges", search: {} } as const;

function decodePathSegment(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

/** Bridges app controller driven exclusively by the logical window's WM location. */
export function BridgesWindow({ windowId }: { windowId: string }) {
  const location = useDesktop(state => state.windows[windowId]?.route ?? DEFAULT_BRIDGES_ROUTE);
  const detailMatch = /^\/bridges\/([^/]+)$/.exec(location.pathname);
  if (detailMatch) return <BridgeDetailLocation id={decodePathSegment(detailMatch[1])} />;
  return <BridgesCatalogLocation search={validateBridgesSearch(location.search)} />;
}
