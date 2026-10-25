import { PropertyRowSkeleton, Skeleton } from "@compozy/ui";

import { TaskRailSection as RailSection } from "./task-rail-section";
import { TASK_RAIL_CLASS, TASK_RAIL_FOOTER_CLASS } from "./task-surface-classes";

/**
 * The placeholder twin of the properties rail: the same card, the three real section labels, a
 * few rows each and the Inspect footer as a block. The call site owns the `SkeletonRegion`.
 */
export function TaskPropertiesRailSkeleton() {
  return (
    <div aria-hidden="true" className={TASK_RAIL_CLASS}>
      <RailSection label="Properties">
        <PropertyRowSkeleton count={2} />
      </RailSection>
      <RailSection
        action={<Skeleton className="-mr-2.5 h-button-sm w-16 rounded-pill" />}
        label="Execution"
      >
        <PropertyRowSkeleton count={4} />
      </RailSection>
      <RailSection label="Activity">
        <PropertyRowSkeleton count={4} />
      </RailSection>
      <footer className={TASK_RAIL_FOOTER_CLASS}>
        <Skeleton className="h-button-sm w-20 rounded-pill" />
      </footer>
    </div>
  );
}
