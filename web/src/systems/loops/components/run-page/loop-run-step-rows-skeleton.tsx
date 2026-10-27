import { SkeletonText } from "@compozy/ui";

import { placeholderSlots } from "../../lib/placeholder-slots";
import { STEP_ROW_CLASS } from "./loop-run-page-classes";

/**
 * The step list before the roster has answered: `count` rows in the list's own frame.
 * Presentational; the caller owns the `SkeletonRegion` (or hides it).
 */
export function LoopRunStepRowsSkeleton({ count = 4 }: { count?: number }) {
  return (
    <ul aria-hidden="true" className="mt-4 flex flex-col">
      {placeholderSlots("step", count).map(slot => (
        <li className={STEP_ROW_CLASS} key={slot.id}>
          <div className="min-w-0 flex-1">
            <SkeletonText className="w-32 text-small-body font-medium" />
          </div>
          <SkeletonText className="w-16 shrink-0 text-small-body" />
        </li>
      ))}
    </ul>
  );
}
