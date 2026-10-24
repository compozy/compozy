import { Skeleton, SkeletonRegion, SkeletonText, cn } from "@compozy/ui";

import {
  PROVIDER_USAGE_LIST_FRAME,
  PROVIDER_USAGE_ROW_FRAME,
  PROVIDER_USAGE_ROW_TRIGGER_FRAME,
} from "./provider-usage-row-frame";

// How many providers there are is unknown until the envelope arrives; three collapsed rows are
// what a profile with its usual set of providers shows.
const ROW_POSITIONS = [0, 1, 2] as const;
const NAME_WIDTHS = ["w-20", "w-16", "w-24"] as const;
const PLAN_WIDTHS = ["w-10", "w-12", "w-8"] as const;
const CUE_WIDTHS = ["w-16", "w-20", "w-14"] as const;

/**
 * The provider rows before the envelope arrives, in the live row's own frames: the chevron and
 * provider mark, the name, plan and trailing cue on one line. Hidden from assistive technology
 * inside the one status region of the area, so the tray popover and the Usage app hold the same
 * place the rows will take.
 */
export function ProviderUsageRowsSkeleton({ className }: { className?: string }) {
  return (
    <SkeletonRegion
      label="Loading provider usage"
      data-testid="provider-usage-loading"
      className={className}
    >
      <ul aria-hidden="true" className={PROVIDER_USAGE_LIST_FRAME}>
        {ROW_POSITIONS.map(position => (
          <li key={position}>
            <div className={PROVIDER_USAGE_ROW_FRAME}>
              <div className={PROVIDER_USAGE_ROW_TRIGGER_FRAME}>
                <span className="flex shrink-0 items-center gap-1">
                  <Skeleton className="size-3 rounded-sm" />
                  <Skeleton className="size-4 rounded-sm" />
                </span>
                <SkeletonText
                  className={cn("text-small-body font-medium", NAME_WIDTHS[position])}
                />
                <SkeletonText className={cn("text-meta", PLAN_WIDTHS[position])} />
                <span className="flex-1" />
                <SkeletonText className={cn("shrink-0 text-meta", CUE_WIDTHS[position])} />
              </div>
            </div>
          </li>
        ))}
      </ul>
    </SkeletonRegion>
  );
}
