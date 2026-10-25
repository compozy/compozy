import { cn, Skeleton, SkeletonRegion, SkeletonText } from "@compozy/ui";

import { TasksDashboardActiveRunsSkeleton } from "./tasks-dashboard-active-runs-skeleton";
import { TasksDashboardCardsSkeleton } from "./tasks-dashboard-cards";
import {
  BODY_CLASS,
  PANEL_PAIR_CLASS,
  QUEUE_CONTROLS_CLASS,
  TOTALS_CLASS,
} from "./tasks-dashboard-classes";
import { TasksDashboardQueueHealthSkeleton } from "./tasks-dashboard-queue-health";
import { TasksDashboardStatusBreakdownSkeleton } from "./tasks-dashboard-status-breakdown";

/**
 * The dashboard while its read is out: every section in its place with its real heading, and
 * the figures left as blocks. The scheduler fold and the totals line are placeholders too, so
 * nothing interactive sits inside the loading region.
 */
export function TasksDashboardLoading() {
  return (
    <SkeletonRegion
      className={cn(BODY_CLASS, "overflow-hidden")}
      data-testid="tasks-dashboard-loading"
      label="Loading tasks dashboard"
    >
      <TasksDashboardCardsSkeleton />
      <TasksDashboardActiveRunsSkeleton />
      <div className={PANEL_PAIR_CLASS}>
        <TasksDashboardQueueHealthSkeleton />
        <TasksDashboardStatusBreakdownSkeleton />
      </div>
      <div aria-hidden="true" className={QUEUE_CONTROLS_CLASS}>
        <div className="flex items-center gap-2 py-2.5 text-small-body">
          <Skeleton className="size-3.5 shrink-0 rounded-full" />
          <SkeletonText className="w-24" />
        </div>
      </div>
      <div aria-hidden="true" className={TOTALS_CLASS}>
        <SkeletonText className="w-32 eyebrow" />
      </div>
    </SkeletonRegion>
  );
}
