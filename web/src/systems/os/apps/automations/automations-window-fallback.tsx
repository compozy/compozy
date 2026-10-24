import { ListingPage } from "@compozy/ui";

import {
  OsWindowToolbarSkeleton,
  ToolbarFilterSkeleton,
  ToolbarSearchSkeleton,
  ToolbarSegmentsSkeleton,
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
 * the bands its kind has; the listing reserves the strip row its location will publish (the
 * Start views, search, filters and the cards/rows toggle stay put while the list loads) over the
 * listing's loading rows or cards.
 */
export function AutomationsWindowFallback({ route }: { route: OsWindowRoute }) {
  const detail = parseAutomationDetailPath(route.pathname);
  if (detail) return <AutomationDetailSkeleton kind={detail.kind === "jobs" ? "job" : "trigger"} />;

  const search = validateAutomationsSearch(route.search);
  return (
    <>
      <OsWindowToolbarSkeleton
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
