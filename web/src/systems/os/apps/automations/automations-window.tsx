import { useDesktop } from "../../hooks/use-desktop";
import { validateAutomationDetailSearch, validateAutomationsSearch } from "@/systems/automation";
import { AutomationDetailLocation } from "./automation-detail-location";
import { AutomationsCatalogLocation } from "./automations-catalog-location";

const DEFAULT_AUTOMATIONS_ROUTE = { pathname: "/automations", search: {} } as const;
const DETAIL_PATH = /^\/automations\/(jobs|triggers)\/([^/]+)$/;

function decodePathSegment(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

/** Automations app controller driven exclusively by the logical window's WM location. */
export function AutomationsWindow({ windowId }: { windowId: string }) {
  const location = useDesktop(state => state.windows[windowId]?.route ?? DEFAULT_AUTOMATIONS_ROUTE);
  const detail = DETAIL_PATH.exec(location.pathname);
  if (detail) {
    return (
      <AutomationDetailLocation
        id={decodePathSegment(detail[2])}
        kind={detail[1] === "jobs" ? "job" : "trigger"}
        search={validateAutomationDetailSearch(location.search)}
      />
    );
  }
  return <AutomationsCatalogLocation search={validateAutomationsSearch(location.search)} />;
}
