import { SkeletonRegion, TableRowSkeleton } from "@compozy/ui";

import { TaskGroupSkeleton } from "./task-group";
import { TASKS_LIST_SKELETON_COLUMNS } from "./tasks-list-row";
import { TasksListTableFrame } from "./tasks-list-table-frame";

/**
 * The buckets a loading list stands in with: two groups of rows, enough to fill a window, with no
 * bucket named because none is known yet. Ids are static; the placeholders carry no identity of
 * their own.
 */
const LOADING_GROUPS = [
  { id: "first", rows: 4 },
  { id: "second", rows: 6 },
] as const;

/** The table before its first rows: header and two groups of placeholder rows, one status region. */
export function TasksListLoading() {
  return (
    <div
      className="@container flex min-h-0 flex-1 flex-col overflow-hidden"
      data-slot="tasks-list-surface"
      data-testid="tasks-list-surface"
    >
      <SkeletonRegion
        className="flex flex-col pb-4"
        data-testid="tasks-list-surface-loading"
        label="Loading tasks"
      >
        <TasksListTableFrame>
          {LOADING_GROUPS.map(group => (
            <TaskGroupSkeleton key={group.id}>
              <TableRowSkeleton columns={TASKS_LIST_SKELETON_COLUMNS} count={group.rows} />
            </TaskGroupSkeleton>
          ))}
        </TasksListTableFrame>
      </SkeletonRegion>
    </div>
  );
}
