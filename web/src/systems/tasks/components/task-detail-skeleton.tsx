import type { TaskDetailTab } from "../lib/task-detail-search";
import { TaskActivityFeedSkeleton } from "./task-activity-panel";
import { TaskOverviewSkeleton } from "./task-loading-skeletons";
import { TaskRunsTableSkeleton } from "./task-runs-panel";

/**
 * The task detail's main column while its record loads: the placeholder of whichever tab the
 * location asks for, so a deep link to Runs or Activity arrives into the table or feed it shows.
 * The call site owns the `SkeletonRegion` around it.
 */
export function TaskDetailPanelsSkeleton({ tab }: { tab: TaskDetailTab }) {
  if (tab === "runs") return <TaskRunsTableSkeleton />;
  if (tab === "activity") return <TaskActivityFeedSkeleton />;
  return <TaskOverviewSkeleton />;
}
