import { useNavigate } from "@tanstack/react-router";

import { useGatewayCapabilities } from "@/systems/gateway";
import {
  DEFAULT_TASK_TEMPLATE_ID,
  taskCatalogSearchFor,
  type TasksRouteSearch,
  type TaskTemplateId,
  type TaskViewMode,
  useTasksPage,
  validateTasksSearch,
} from "@/systems/tasks";

import { useCurrentWindowLiveDataEnabled } from "../../../hooks/use-window-live-data-enabled";
import { useOsShell } from "../../../hooks/use-os-shell";

/**
 * Tasks catalog data and behavior in one hook so the route component stays
 * under the component-complexity budget.
 */
export function useTasksCatalogLocation({ search }: { search: TasksRouteSearch }) {
  const { coordinator } = useOsShell();
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const routeNavigate = useNavigate();
  // Task/run mutations and the scheduler exist only on the local surface set.
  // On remote tiers those affordances are absent, never disabled (BR-1); the
  // read views (list, kanban, dashboard, inbox reads) render normally.
  const { localTaskLifecycle } = useGatewayCapabilities();
  const mode: TaskViewMode = search.mode ?? "list";
  const page = useTasksPage({
    liveDataEnabled,
    search,
    onSearchChange: update => {
      void routeNavigate({
        to: "/tasks",
        search: current => update(validateTasksSearch(current)),
        replace: true,
      });
    },
  });
  const navigate = (pathname: string, search: Record<string, unknown> = {}) =>
    void coordinator.userOpen({ app: "tasks", route: { pathname, search } });
  // The create dialog layers over this catalog, so the active view rides along
  // and dismissal lands back on the view the operator was reading.
  const openCreate = (template?: TaskTemplateId) =>
    navigate("/tasks/new", {
      ...taskCatalogSearchFor(mode, search),
      ...(template && template !== DEFAULT_TASK_TEMPLATE_ID ? { template } : {}),
    });
  return { localTaskLifecycle, mode, navigate, openCreate, page };
}
