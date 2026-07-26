import { act, render, screen, waitFor } from "@testing-library/react";
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

import { Topbar, TopbarSlotProvider, UIProvider } from "@agh/ui";

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

function buildTestRouter(initialUrl: string) {
  const rootRoute = createRootRoute({
    component: () => <Outlet />,
  });

  const appRoute = createRoute({
    getParentRoute: () => rootRoute,
    id: "_app",
    component: () => (
      <div data-testid="os-desktop">
        <Outlet />
      </div>
    ),
  });

  const tasksRoute = createRoute({
    getParentRoute: () => appRoute,
    path: "tasks",
    component: () => (
      <div data-testid="tasks-shell">
        <Outlet />
      </div>
    ),
  });

  const taskDetailRoute = createRoute({
    getParentRoute: () => tasksRoute,
    path: "$id",
    validateSearch: validateTaskDetailSearch,
    component: () => {
      return (
        <div data-testid="tasks-detail">
          <Outlet />
        </div>
      );
    },
  });

  const taskCreateRoute = createRoute({
    getParentRoute: () => tasksRoute,
    path: "new",
    component: () => <div data-testid="tasks-create-route">new task</div>,
  });

  const taskEditRoute = createRoute({
    getParentRoute: () => taskDetailRoute,
    path: "edit",
    component: () => {
      const params = taskDetailRoute.useParams();
      return (
        <div data-testid="tasks-edit-route">
          <span data-testid="tasks-edit-id">{params.id}</span>
        </div>
      );
    },
  });

  const taskRunDetailRoute = createRoute({
    getParentRoute: () => taskDetailRoute,
    path: "runs/$runId",
    component: () => {
      const params = taskRunDetailRoute.useParams();
      return (
        <div data-testid="tasks-run-detail">
          <span data-testid="tasks-run-detail-task-id">{params.id}</span>
          <span data-testid="tasks-run-detail-run-id">{params.runId}</span>
        </div>
      );
    },
  });

  const routeTree = rootRoute.addChildren([
    appRoute.addChildren([
      tasksRoute.addChildren([
        taskCreateRoute,
        taskDetailRoute.addChildren([taskEditRoute, taskRunDetailRoute]),
      ]),
    ]),
  ]);

  return createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: [initialUrl] }),
  });
}

describe("tasks router registration (integration)", () => {
  it("resolves the base /tasks route inside the tasks shell", async () => {
    const router = buildTestRouter("/tasks");
    render(<RouterProvider router={router} />);
    await waitFor(() => expect(screen.getByTestId("tasks-shell")).toBeInTheDocument());
    expect(screen.queryByTestId("tasks-detail")).not.toBeInTheDocument();
    expect(screen.queryByTestId("tasks-run-detail")).not.toBeInTheDocument();
  });

  it("resolves /tasks/$id with the shell still mounted", async () => {
    const router = buildTestRouter("/tasks/task_abc");
    render(<RouterProvider router={router} />);
    await waitFor(() => expect(screen.getByTestId("tasks-shell")).toBeInTheDocument());
    expect(screen.getByTestId("tasks-detail")).toBeInTheDocument();
    expect(screen.queryByTestId("tasks-run-detail")).not.toBeInTheDocument();
  });

  it("resolves /tasks/$id/runs/$runId as a child of the detail route", async () => {
    const router = buildTestRouter("/tasks/task_abc/runs/run_001");
    render(<RouterProvider router={router} />);
    await waitFor(() => expect(screen.getByTestId("tasks-shell")).toBeInTheDocument());
    expect(screen.getByTestId("tasks-detail")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-run-detail")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-run-detail-task-id")).toHaveTextContent("task_abc");
    expect(screen.getByTestId("tasks-run-detail-run-id")).toHaveTextContent("run_001");
  });

  it("resolves /tasks/new inside the shared tasks shell", async () => {
    const router = buildTestRouter("/tasks/new");
    render(<RouterProvider router={router} />);
    await waitFor(() => expect(screen.getByTestId("tasks-shell")).toBeInTheDocument());
    expect(screen.getByTestId("tasks-create-route")).toBeInTheDocument();
  });

  it("resolves /tasks/$id/edit inside the shared tasks shell", async () => {
    const router = buildTestRouter("/tasks/task_abc/edit");
    render(<RouterProvider router={router} />);
    await waitFor(() => expect(screen.getByTestId("tasks-shell")).toBeInTheDocument());
    expect(screen.getByTestId("tasks-edit-route")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-edit-id")).toHaveTextContent("task_abc");
  });

  it("keeps the tasks shell mounted while navigating between base, create, detail, edit, and run-detail routes", async () => {
    const router = buildTestRouter("/tasks");
    render(<RouterProvider router={router} />);
    await waitFor(() => expect(screen.getByTestId("tasks-shell")).toBeInTheDocument());
    const baseShell = screen.getByTestId("tasks-shell");

    await act(() => router.history.push("/tasks/new"));
    await waitFor(() => expect(screen.getByTestId("tasks-create-route")).toBeInTheDocument());
    expect(screen.getByTestId("tasks-shell")).toBe(baseShell);

    await act(() => router.navigate({ to: "/tasks/$id", params: { id: "task_abc" } }));
    await waitFor(() => expect(screen.getByTestId("tasks-detail")).toBeInTheDocument());
    expect(screen.getByTestId("tasks-shell")).toBe(baseShell);

    await act(() => router.history.push("/tasks/task_abc/edit"));
    await waitFor(() => expect(screen.getByTestId("tasks-edit-route")).toBeInTheDocument());
    expect(screen.getByTestId("tasks-shell")).toBe(baseShell);

    await act(() =>
      router.navigate({
        to: "/tasks/$id/runs/$runId",
        params: { id: "task_abc", runId: "run_001" },
      })
    );
    await waitFor(() => expect(screen.getByTestId("tasks-run-detail")).toBeInTheDocument());
    expect(screen.getByTestId("tasks-shell")).toBe(baseShell);
  });
});

const productionManagers: WindowManagerRuntime[] = [];

function buildProductionDetailRouter(initialUrl: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const manager = new WindowManagerRuntime(queryClient);
  productionManagers.push(manager);
  const port: OsRouterPort = { navigate: () => undefined, replace: () => undefined };
  const coordinator = new RoutingCoordinator(manager, port);
  coordinator.completeHydration();
  const shell: OsShellHandle = { store: manager, manager, coordinator };

  const rootRoute = createRootRoute({
    component: () => (
      <QueryClientProvider client={queryClient}>
        <UIProvider reducedMotion="always">
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
