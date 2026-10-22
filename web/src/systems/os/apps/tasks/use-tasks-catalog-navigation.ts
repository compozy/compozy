import {
  DEFAULT_TASK_TEMPLATE_ID,
  taskCatalogSearchFor,
  type TaskListItem,
  type TasksRouteSearch,
  type TaskTemplateId,
  type TaskViewMode,
} from "@/systems/tasks";
import { ownerDataScope } from "@/systems/workspace";

import { useOpenTarget } from "../../hooks/use-open-target";
import { useOsShell } from "../../hooks/use-os-shell";

/**
 * The Tasks catalog's moves: a task opens through the one item path — in place
 * under All projects, bound to its own project, and where it lives otherwise —
 * and the create dialog layers over the view the operator was reading.
 */
export function useTasksCatalogNavigation({
  mode,
  search,
  tasks,
}: {
  mode: TaskViewMode;
  search: TasksRouteSearch;
  tasks: readonly Pick<TaskListItem, "id" | "workspace_id">[];
}) {
  const { coordinator } = useOsShell();
  const openItem = useOpenTarget();
  const navigate = (pathname: string, routeSearch: Record<string, unknown> = {}) =>
    void coordinator.userOpen({ app: "tasks", route: { pathname, search: routeSearch } });

  return {
    openTask: (taskId: string) => {
      const pathname = `/tasks/${encodeURIComponent(taskId)}`;
      const task = tasks.find(candidate => candidate.id === taskId);
      if (!task) {
        navigate(pathname);
        return;
      }
      openItem({
        app: "tasks",
        binding: ownerDataScope(task.workspace_id),
        route: { pathname, search: {} },
      });
    },
    // The create dialog layers over this catalog, so the active view rides along
    // and dismissal lands back on the view the operator was reading.
    openCreate: (template?: TaskTemplateId) =>
      navigate("/tasks/new", {
        ...taskCatalogSearchFor(mode, search),
        ...(template && template !== DEFAULT_TASK_TEMPLATE_ID ? { template } : {}),
      }),
  };
}
