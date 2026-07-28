import { useDesktop } from "../../hooks/use-desktop";
import { parseTaskWindowLocation } from "./task-window-location";
import { TaskCreateDialog, TaskEditDialog } from "./task-editor-dialogs";
import { TaskDetailLocation } from "./task-detail-location";
import { TaskRunLocation } from "./task-run-location";
import { TasksCatalogLocation } from "./tasks-catalog-location";

const DEFAULT_TASKS_ROUTE = { pathname: "/tasks", search: {} } as const;

/** Tasks app controller driven exclusively by the logical window's WM location. */
export function TasksWindow({ windowId }: { windowId: string }) {
  const location = useDesktop(state => state.windows[windowId]?.route ?? DEFAULT_TASKS_ROUTE);
  const parsed = parseTaskWindowLocation(location);

  // The editor is a window-scoped dialog: its location renders the surface it
  // was opened from and layers the modal over it.
  if (parsed.kind === "create") {
    return (
      <>
        <TasksCatalogLocation mode={parsed.mode} />
        <TaskCreateDialog catalogMode={parsed.mode} search={parsed.search} />
      </>
    );
  }
  if (parsed.kind === "edit") {
    return (
      <>
        <TaskDetailLocation search={parsed.search} taskId={parsed.taskId} />
        <TaskEditDialog search={parsed.search} taskId={parsed.taskId} />
      </>
    );
  }
  if (parsed.kind === "detail") {
    return <TaskDetailLocation search={parsed.search} taskId={parsed.taskId} />;
  }
  if (parsed.kind === "run") {
    return <TaskRunLocation runId={parsed.runId} taskId={parsed.taskId} />;
  }
  return <TasksCatalogLocation mode={parsed.mode} />;
}
