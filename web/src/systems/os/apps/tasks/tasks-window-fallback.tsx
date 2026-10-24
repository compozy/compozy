import { Separator } from "@compozy/ui";

import {
  ToolbarFilterSkeleton,
  ToolbarSearchSkeleton,
  ToolbarSegmentsSkeleton,
  ToolbarSlotSkeleton,
} from "../../components/os-window-toolbar-skeleton";
import type { OsWindowRoute } from "../../lib/os-types";
import {
  TaskDetailLoadingGrid,
  TaskDetailTabsInert,
  TaskPageLoadingFrame,
  TaskRunLoadingGrid,
} from "./task-loading-frames";
import { parseTaskWindowLocation } from "./task-window-location";
import { TasksCatalogSkeleton, type TaskDetailTab, type TaskViewMode } from "@/systems/tasks";

/** The catalog strip: the four views, and the list's own tools when the list is the open view. */
function TasksCatalogStripSkeleton({ mode }: { mode: TaskViewMode }) {
  const list = mode === "list";
  return (
    <ToolbarSlotSkeleton
      leading={
        <>
          <ToolbarSegmentsSkeleton className="w-72" />
          {list ? (
            <>
              <Separator className="h-4 self-center" orientation="vertical" />
              <ToolbarSearchSkeleton className="max-w-48 flex-1" />
              <ToolbarFilterSkeleton />
            </>
          ) : null}
        </>
      }
      trailing={
        list ? (
          <div className="flex items-center gap-2.5">
            <ToolbarFilterSkeleton />
            <ToolbarFilterSkeleton />
          </div>
        ) : undefined
      }
    />
  );
}

function TasksCatalogFallback({ mode }: { mode: TaskViewMode }) {
  return (
    <>
      <TasksCatalogStripSkeleton mode={mode} />
      <div
        className="flex min-h-0 flex-1 flex-col overflow-hidden"
        data-density="route"
        data-testid="tasks-shell"
      >
        <TasksCatalogSkeleton mode={mode} />
      </div>
    </>
  );
}

function TaskDetailFallback({ tab }: { tab: TaskDetailTab }) {
  return (
    <TaskPageLoadingFrame bottomPadding="pb-20">
      <TaskDetailTabsInert tab={tab} />
      <TaskDetailLoadingGrid tab={tab} />
    </TaskPageLoadingFrame>
  );
}

/**
 * The tasks window's body while its code loads: the surface the route is about to show, in the
 * state the window itself shows while that surface's read is out. The catalog publishes the
 * strip its location will; a task, its edit route and a run show the page frame.
 */
export function TasksWindowFallback({ route }: { route: OsWindowRoute }) {
  const location = parseTaskWindowLocation(route);
  if (location.kind === "run") {
    return (
      <TaskPageLoadingFrame bottomPadding="pb-16">
        <TaskRunLoadingGrid />
      </TaskPageLoadingFrame>
    );
  }
  if (location.kind === "detail" || location.kind === "edit") {
    return <TaskDetailFallback tab={location.search.tab} />;
  }
  return <TasksCatalogFallback mode={location.mode} />;
}
