import { Skeleton, SkeletonText } from "@compozy/ui";

import { placeholderSlots } from "../../lib/placeholder-slots";
import { BEAT_ROW_CLASS } from "./loop-run-page-classes";

/** The activity beats before the timeline has answered: rows in the story's own card. */
export function LoopRunStoryRowsSkeleton({ count = 4 }: { count?: number }) {
  return (
    <ul aria-hidden="true" className="flex flex-col divide-y divide-line-soft">
      {placeholderSlots("beat", count).map(slot => (
        <li className={BEAT_ROW_CLASS} key={slot.id}>
          <Skeleton className="mt-0.5 size-6 shrink-0 rounded-full" />
          <div className="min-w-0 flex-1">
            <SkeletonText className="w-3/5 text-small-body" />
          </div>
          <SkeletonText className="w-12 shrink-0 text-form-hint" />
        </li>
      ))}
    </ul>
  );
}
