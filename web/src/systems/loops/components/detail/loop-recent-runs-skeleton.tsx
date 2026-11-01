import { ChevronRight } from "@compozy/ui/icons";

import { SkeletonText } from "@compozy/ui";

import { placeholderSlots } from "../../lib/placeholder-slots";
import { RECENT_RUN_ROW_CLASS, RECENT_RUNS_CLASS } from "./loop-recent-runs-classes";

/**
 * The recent runs while they are being read: `count` rows in the list's own card. Presentational,
 * so the caller wraps it in the `SkeletonRegion` for the area it stands in for.
 */
export function LoopRecentRunsSkeleton({ count = 3 }: { count?: number }) {
  return (
    <div aria-hidden="true" className={RECENT_RUNS_CLASS} data-testid="loop-recent-runs-loading">
      {placeholderSlots("recent-run", count).map(slot => (
        <div className={RECENT_RUN_ROW_CLASS} key={slot.id}>
          <SkeletonText className="w-16 text-small-body" />
          <SkeletonText className="w-24 text-form-hint" />
          <SkeletonText className="w-10 font-mono text-mono-id" />
          <ChevronRight aria-hidden="true" className="size-3.5 text-faint" />
        </div>
      ))}
    </div>
  );
}
