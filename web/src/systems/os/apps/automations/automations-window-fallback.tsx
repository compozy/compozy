import { ListingPage } from "@compozy/ui";

import {
  ToolbarFilterSkeleton,
  ToolbarSearchSkeleton,
  ToolbarSegmentsSkeleton,
  ToolbarSlotSkeleton,
  ToolbarViewToggleSkeleton,
} from "../../components/os-window-toolbar-skeleton";
import type { OsWindowRoute } from "../../lib/os-types";
import {
  AutomationCatalogSkeleton,
  AutomationDetailSkeleton,
  parseAutomationDetailPath,
  validateAutomationsSearch,
} from "@/systems/automation";

/**
 * The automations window's body while its code loads. An automation shows its page frame with
 * the bands its kind has; the listing publishes the strip its location will (the Start views,
 * search, filters and the cards/rows toggle stay put while the list loads) over the listing's
 * loading rows or cards.
 */
export function AutomationsWindowFallback({ route }: { route: OsWindowRoute }) {
  const detail = parseAutomationDetailPath(route.pathname);
  if (detail) return <AutomationDetailSkeleton kind={detail.kind === "jobs" ? "job" : "trigger"} />;

  const search = validateAutomationsSearch(route.search);
  return (
    <>
      <ToolbarSlotSkeleton
        leading={
          <>
            <ToolbarSegmentsSkeleton className="w-52" />
            <ToolbarSearchSkeleton />
            <ToolbarFilterSkeleton />
          </>
        }
        trailing={<ToolbarViewToggleSkeleton />}
      />
      <ListingPage data-testid="automations-shell">
        <AutomationCatalogSkeleton view={search.view === "cards" ? "cards" : "rows"} />
      </ListingPage>
    </>
  );
}
