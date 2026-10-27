import type { ReactNode } from "react";

import { cn, LaneTabs, PAGE_CONTENT_GUTTER, SkeletonRegion } from "@compozy/ui";

import {
  buildTabItems,
  TASK_DETAIL_GRID_CLASS,
  TASK_DETAIL_RAIL_CLASS,
} from "./task-detail-layout";
import {
  TaskDetailPanelsSkeleton,
  TaskPropertiesRailSkeleton,
  TaskRunMainSkeleton,
  TaskRunRailSkeleton,
  TaskSubheadSkeleton,
  type TaskDetailTab,
} from "@/systems/tasks";

// The loading frames of the task and run pages, shared by the locations (while a record loads)
// and by the window's fallback (while the window's code loads), so the two are one continuous
// frame: head above, then the page's gutter and container with the subhead block, then the grid
// of the main column and the rail as the single loading region.

const noop = () => undefined;

/** The scroll frame a task page fills; `head` publishes the window head and renders nothing. */
export function TaskPageLoadingFrame({
  head,
  bottomPadding,
  children,
}: {
  head?: ReactNode;
  bottomPadding: "pb-16" | "pb-20";
  children: ReactNode;
}) {
  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
      {head}
      <div className="min-h-0 flex-1 overflow-hidden">
        <div className={cn(PAGE_CONTENT_GUTTER, "@container pt-5", bottomPadding)}>
          <TaskSubheadSkeleton />
          {children}
        </div>
      </div>
    </div>
  );
}

/** The task's main column and rail: the open tab's placeholder beside the properties rail. */
export function TaskDetailLoadingGrid({ tab }: { tab: TaskDetailTab }) {
  return (
    <SkeletonRegion
      className={cn(TASK_DETAIL_GRID_CLASS, "pt-5")}
      data-testid="tasks-detail-loading"
      label="Loading task"
    >
      <div className="min-w-0">
        <TaskDetailPanelsSkeleton tab={tab} />
      </div>
      <aside className={TASK_DETAIL_RAIL_CLASS}>
        <TaskPropertiesRailSkeleton />
      </aside>
    </SkeletonRegion>
  );
}

/** The run's main column and rail. */
export function TaskRunLoadingGrid() {
  return (
    <SkeletonRegion
      className={TASK_DETAIL_GRID_CLASS}
      data-testid="tasks-run-detail-loading"
      label="Loading run"
    >
      <TaskRunMainSkeleton />
      <aside className={TASK_DETAIL_RAIL_CLASS}>
        <TaskRunRailSkeleton />
      </aside>
    </SkeletonRegion>
  );
}

/** The task's tab list for a frame shown before the window mounts: the real bar, out of reach. */
export function TaskDetailTabsInert({ tab }: { tab: TaskDetailTab }) {
  return (
    <div inert>
      <LaneTabs<TaskDetailTab>
        ariaLabel="Task views"
        className="gap-0"
        items={buildTabItems("pending")}
        listClassName="w-full"
        onChange={noop}
        value={tab}
      />
    </div>
  );
}
