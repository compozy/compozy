import type { ComponentProps } from "react";

import { cn, Skeleton, SkeletonText } from "@compozy/ui";

/**
 * The page head while the automation loads: sentence lines where the sentence goes (never a
 * guessed one), the switch well opposite, and the subhead line. A schedule reads as one line, an
 * event as two, which is how those sentences usually set.
 */
export function AutomationDetailHeadSkeleton({
  sentenceLines = 2,
  className,
  ...props
}: Omit<ComponentProps<"div">, "children"> & { sentenceLines?: number }) {
  return (
    <div
      aria-hidden="true"
      className={cn("mb-5.5 border-b border-line pb-4.5", className)}
      {...props}
    >
      <div className="flex items-start justify-between gap-6">
        <div className="min-w-0 flex-1">
          <div className="max-w-[46ch] text-detail-h1 font-medium tracking-detail-h1">
            <SkeletonText lines={sentenceLines} />
          </div>
        </div>
        <Skeleton className="mt-0.5 h-8 w-24 shrink-0" />
      </div>
      <div className="mt-3 text-eyebrow">
        <SkeletonText className="w-80" />
      </div>
    </div>
  );
}
