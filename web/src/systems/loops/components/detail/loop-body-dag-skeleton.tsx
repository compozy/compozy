import { ArrowRight } from "@compozy/ui/icons";

import { SkeletonText } from "@compozy/ui";

import { placeholderSlots } from "../../lib/placeholder-slots";
import { DAG_ARROW_CLASS, DAG_CARD_CLASS, DAG_LANE_CLASS } from "./loop-body-dag-classes";

/**
 * The steps graph while the Loop is being read: a spine of `count` step cards in the graph's own
 * recessed lane. Presentational, so the caller wraps the area in a `SkeletonRegion`.
 */
export function LoopBodyDagSkeleton({ count = 4 }: { count?: number }) {
  const slots = placeholderSlots("dag-step", count);
  return (
    <div aria-hidden="true" className="rounded-lg bg-sunken" data-testid="loop-dag-loading">
      <div className={DAG_LANE_CLASS}>
        {slots.map(slot => (
          <div className="flex items-stretch" key={slot.id}>
            <div className={DAG_CARD_CLASS}>
              <SkeletonText className="w-20 text-small-body" />
              <SkeletonText className="w-16 text-form-hint" />
            </div>
            {slot.position < slots.length - 1 ? (
              <span aria-hidden="true" className={DAG_ARROW_CLASS}>
                <ArrowRight className="size-4" />
              </span>
            ) : null}
          </div>
        ))}
      </div>
    </div>
  );
}
