import type { LaneTabsItem } from "@compozy/ui";

import type { TaskDetailTab } from "@/systems/tasks";

export const TASK_DETAIL_GRID_CLASS =
  "grid items-start gap-8 @min-task-detail-rail:grid-cols-[minmax(0,1fr)_var(--width-task-properties-rail)]";

export const TASK_DETAIL_RAIL_CLASS =
  "min-w-0 @min-task-detail-rail:sticky @min-task-detail-rail:top-0";

/**
 * `runCount` is `"pending"` while the runs are still being read: the tab holds the count's slot, so
 * it never claims zero and the tabs after it do not move when the number arrives.
 */
export function buildTabItems(
  runCount: number | "pending"
): ReadonlyArray<LaneTabsItem<TaskDetailTab>> {
  return [
    { value: "overview", label: "Overview", testId: "tasks-detail-tab-overview" },
    {
      value: "runs",
      label: "Runs",
      count: runCount,
      // A task rarely has more than nine attempts, so the placeholder is one digit wide.
      pendingCountDigits: 1,
      testId: "tasks-detail-tab-runs",
    },
    { value: "activity", label: "Activity", testId: "tasks-detail-tab-activity" },
  ];
}
