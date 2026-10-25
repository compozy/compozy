import { act, render, screen, waitFor, within } from "@testing-library/react";
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
import { tasksKeys, validateTaskDetailSearch, type TaskDetailView } from "@/systems/tasks";

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
  const router = createRouter({
    routeTree: rootRoute.addChildren([detailRoute]),
    history: createMemoryHistory({ initialEntries: [initialUrl] }),
  });
  return { queryClient, router };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>(settle => {
    resolve = settle;
  });
  return { promise, resolve };
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
    const { router } = buildProductionDetailRouter("/tasks/task_abc?tab=activity");
    render(<RouterProvider router={router} />);

    await waitFor(() => expect(screen.getByTestId("tasks-detail-content")).toBeInTheDocument());
    expect(screen.getByTestId("tasks-detail-title")).toHaveTextContent("Production route task");
    expect(screen.getByTestId("tasks-detail-tab-activity")).toHaveAttribute(
      "aria-selected",
      "true"
    );
  });

  it("Should swap the tab panel when the location's tab changes, and leave only that panel", async () => {
    const { router } = buildProductionDetailRouter("/tasks/task_abc");
    render(<RouterProvider router={router} />);
    await waitFor(() => expect(screen.getByTestId("tasks-detail-content")).toBeInTheDocument());
    expect(screen.getByTestId("tasks-detail-overview")).toBeInTheDocument();

    await act(async () => {
      await router.navigate({
        to: "/tasks/$id",
        params: { id: "task_abc" },
        search: { tab: "runs" },
      });
    });

    // The tabs and their panel follow the location together, as a deferred swap that settles.
    await waitFor(() =>
      expect(screen.getByTestId("tasks-detail-tab-runs")).toHaveAttribute("aria-selected", "true")
    );
    expect(await screen.findByTestId("tasks-runs-empty")).toBeInTheDocument();
    expect(screen.queryByTestId("tasks-detail-overview")).not.toBeInTheDocument();
    expect(screen.getByTestId("tasks-detail-tab-overview")).toHaveAttribute(
      "aria-selected",
      "false"
    );
  });

  it("Should stand in with the task's own shape while it loads, and not again on a refetch", async () => {
    const detail = buildDetailFixture({
      task: { id: "task_abc", title: "Production route task", latest_event_seq: undefined },
      summary: { id: "task_abc", title: "Production route task" },
    } as never);
    const record = deferred<TaskDetailView>();
    vi.mocked(taskApi.getTask).mockReturnValueOnce(record.promise);
    const { queryClient, router } = buildProductionDetailRouter("/tasks/task_abc?tab=runs");
    render(<RouterProvider router={router} />);

    // One named loading area under the real tabs, on the tab the location asked for, and a head
    // that already shows the way back to the list.
    const region = await screen.findByTestId("tasks-detail-loading");
    expect(region).toHaveAttribute("role", "status");
    expect(region).toHaveTextContent("Loading task");
    expect(screen.getByTestId("tasks-detail-tab-runs")).toHaveAttribute("aria-selected", "true");
    expect(screen.getByTestId("tasks-detail-tab-overview")).toBeInTheDocument();
    expect(within(region).getByText("Attempt")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Tasks" })).toBeInTheDocument();
    expect(screen.queryByTestId("tasks-detail-content")).not.toBeInTheDocument();
    // The Runs tab already holds its count's slot, so the strip is the width it will be.
    const runsCount = () =>
      screen.getByTestId("tasks-detail-tab-runs").querySelector('[data-slot="tabs-trigger-count"]');
    expect(runsCount()).toHaveAttribute("data-pending");

    record.resolve(detail);
    await waitFor(() => expect(screen.getByTestId("tasks-detail-content")).toBeInTheDocument());
    expect(screen.queryByTestId("tasks-detail-loading")).not.toBeInTheDocument();
    await waitFor(() => expect(runsCount()).toHaveTextContent("0"));
    expect(runsCount()).not.toHaveAttribute("data-pending");

    // A later refetch has data to show: the record stays and no loading area comes back.
    const refetch = deferred<TaskDetailView>();
    vi.mocked(taskApi.getTask).mockReturnValueOnce(refetch.promise);
    const reads = vi.mocked(taskApi.getTask).mock.calls.length;
    void queryClient.invalidateQueries({ queryKey: tasksKeys.all });
    await waitFor(() => expect(vi.mocked(taskApi.getTask).mock.calls.length).toBe(reads + 1));
    expect(screen.getByTestId("tasks-detail-content")).toBeInTheDocument();
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    await act(async () => refetch.resolve(detail));
  });
});
