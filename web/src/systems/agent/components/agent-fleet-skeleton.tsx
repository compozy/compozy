import {
  CatalogCardSkeleton,
  ListingRowSkeleton,
  SkeletonRegion,
  type ListingViewMode,
} from "@compozy/ui";

import {
  FLEET_CARDS_CLASS,
  FLEET_ROWS_CLASS,
  FLEET_SKELETON_CARDS,
  FLEET_SKELETON_ROWS,
} from "./agent-fleet-classes";

/** The fleet before its first read: loading rows or cards in the containers the loaded fleet fills. */
export function AgentFleetSkeleton({ view }: { view: ListingViewMode }) {
  return (
    <SkeletonRegion
      className="min-h-0 flex-1"
      data-testid="agent-fleet-loading"
      label="Loading agents"
    >
      {view === "cards" ? (
        <div className={FLEET_CARDS_CLASS}>
          <CatalogCardSkeleton count={FLEET_SKELETON_CARDS} descriptionLines={0} facts={false} />
        </div>
      ) : (
        <div className={FLEET_ROWS_CLASS}>
          <ListingRowSkeleton count={FLEET_SKELETON_ROWS} trail="pill" />
        </div>
      )}
    </SkeletonRegion>
  );
}
