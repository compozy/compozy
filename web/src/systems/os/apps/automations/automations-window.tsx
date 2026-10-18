import { useDesktop } from "../../hooks/use-desktop";
import {
  parseAutomationDetailPath,
  validateAutomationDetailSearch,
  validateAutomationsSearch,
} from "@/systems/automation";
import { AutomationDetailLocation } from "./automation-detail-location";
import { AutomationsCatalogLocation } from "./automations-catalog-location";

const DEFAULT_AUTOMATIONS_ROUTE = { pathname: "/automations", search: {} } as const;

/** Automations app controller driven exclusively by the logical window's WM location. */
export function AutomationsWindow({ windowId }: { windowId: string }) {
  const location = useDesktop(state => state.windows[windowId]?.route ?? DEFAULT_AUTOMATIONS_ROUTE);
  const detail = parseAutomationDetailPath(location.pathname);
  if (detail) {
    return (
      <AutomationDetailLocation
        id={detail.id}
        kind={detail.kind === "jobs" ? "job" : "trigger"}
        search={validateAutomationDetailSearch(location.search)}
      />
    );
  }
  return <AutomationsCatalogLocation search={validateAutomationsSearch(location.search)} />;
}
