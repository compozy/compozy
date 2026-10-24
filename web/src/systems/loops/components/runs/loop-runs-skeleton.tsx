import { SkeletonRegion } from "@compozy/ui";

import { LoopRunsTableSkeleton } from "./loop-runs-roster";

/**
 * The runs roster before its first read: the group table in the roster's own frame, with its real
 * column header. It sits inside the same `ListingPage` the loaded roster does.
 */
export function LoopRunsSkeleton({ withProject }: { withProject: boolean }) {
  return (
    <SkeletonRegion
      className="flex flex-col gap-2.5"
      data-testid="loop-runs-loading"
      label="Loading runs"
    >
      <div className="flex flex-col gap-3">
        <LoopRunsTableSkeleton withProject={withProject} />
      </div>
    </SkeletonRegion>
  );
}
