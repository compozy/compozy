import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
} from "@tanstack/react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { Topbar, TopbarSlotProvider, UIProvider } from "@compozy/ui";

vi.mock("@/systems/tasks/hooks/use-task-setup-runtime", () => ({
  useTaskSetupRuntime: () => ({
    catalogError: null,
    catalogLoaded: true,
    catalogLoading: false,
    catalogRefreshing: false,
    models: [],
    onRefreshCatalog: () => undefined,
    providers: [],
    providersError: null,
    providersLoading: false,
  }),
}));

import * as taskApi from "@/systems/tasks/adapters/tasks-api";
import { TaskDetailLocation } from "@/systems/os/apps/tasks/task-detail-location";
import {
  OsShellContext,
  RoutingCoordinator,
  WindowManagerRuntime,
  type OsRouterPort,
  type OsShellHandle,
} from "@/systems/os";
import { buildDetailFixture } from "@/systems/tasks/mocks/fixtures";
import { validateTaskDetailSearch } from "@/systems/tasks";

const productionManagers: WindowManagerRuntime[] = [];

function buildProductionDetailRouter(initialUrl: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const manager = new WindowManagerRuntime(queryClient);
  productionManagers.push(manager);
  const port: OsRouterPort = { navigate: () => undefined, replace: () => undefined };
  const coordinator = new RoutingCoordinator(manager, port);
  coordinator.completeHydration();
  const shell: OsShellHandle = { projection: manager.projectionAtom, manager, coordinator };

  const rootRoute = createRootRoute({
    component: () => (
      <QueryClientProvider client={queryClient}>
        <UIProvider reducedMotion="never" skipAnimations>
          <OsShellContext.Provider value={shell}>
            <TopbarSlotProvider>
              <Topbar title="Tasks" />
              <Outlet />
            </TopbarSlotProvider>
          </OsShellContext.Provider>
        </UIProvider>
      </QueryClientProvider>
    ),
  });
  const detailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "tasks/$id",
    validateSearch: validateTaskDetailSearch,
    component: () => {
      const { id } = detailRoute.useParams();
      return <TaskDetailLocation search={detailRoute.useSearch()} taskId={id} />;
    },
  });
  return createRouter({
    routeTree: rootRoute.addChildren([detailRoute]),
    history: createMemoryHistory({ initialEntries: [initialUrl] }),
  });
}

describe("production task detail route (integration)", () => {
  afterEach(() => {
    for (const manager of productionManagers.splice(0)) manager.destroy();
  });

  beforeEach(() => {
    const detail = buildDetailFixture({
      task: {
        id: "task_abc",
        title: "Production route task",
        latest_event_seq: undefined,
      },
      summary: { id: "task_abc", title: "Production route task" },
    } as never);
    vi.spyOn(taskApi, "getTask").mockResolvedValue(detail);
    vi.spyOn(taskApi, "getTaskTimeline").mockResolvedValue([]);
    vi.spyOn(taskApi, "listTaskRuns").mockResolvedValue([]);
    vi.spyOn(taskApi, "inspectTask").mockResolvedValue(null as never);
    vi.spyOn(taskApi, "getTaskExecutionProfile").mockResolvedValue(null as never);
    vi.spyOn(taskApi, "listTaskReviews").mockResolvedValue([]);
  });

  it("Should mount TaskDetailLocation and preserve the validated Activity tab", async () => {
    const router = buildProductionDetailRouter("/tasks/task_abc?tab=activity");
    render(<RouterProvider router={router} />);

    await waitFor(() => expect(screen.getByTestId("tasks-detail-content")).toBeInTheDocument());
    expect(screen.getByTestId("tasks-detail-title")).toHaveTextContent("Production route task");
    expect(screen.getByTestId("tasks-detail-tab-activity")).toHaveAttribute(
      "aria-selected",
      "true"
    );
  });
});
