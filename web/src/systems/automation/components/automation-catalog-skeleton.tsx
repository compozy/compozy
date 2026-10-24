import { ListingRowSkeleton, SkeletonRegion, type ListingViewMode } from "@compozy/ui";

import { AutomationCardSkeleton } from "./automation-card-skeleton";
import { AUTOMATION_CARDS_CLASS, AUTOMATION_ROWS_CLASS } from "./automation-catalog-classes";

/** The listing before its first read, in the containers the loaded rows or cards fill. */
export function AutomationCatalogSkeleton({ view }: { view: ListingViewMode }) {
  return (
    <SkeletonRegion data-testid="automations-list-loading" label={`Loading automations as ${view}`}>
      {view === "cards" ? (
        <div className={AUTOMATION_CARDS_CLASS}>
          <AutomationCardSkeleton count={6} />
        </div>
      ) : (
        <div className={AUTOMATION_ROWS_CLASS}>
          <ListingRowSkeleton count={6} description />
        </div>
      )}
    </SkeletonRegion>
  );
}
