import { QueryClientProvider } from "@tanstack/react-query";
import { createMemoryHistory, createRouter, RouterProvider } from "@tanstack/react-router";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createTestQueryClient, installFetchStub } from "@/test/utils";

import { routeTree } from "../routeTree.gen";
import { resetActiveWorkspaceStoreForTests } from "../systems/app-shell";

const workspaceOne = {
  id: "ws-1",
  name: "one",
  root_dir: "/tmp/one",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

const workflow = { id: "wf-1", slug: "alpha", workspace_id: "ws-1" };

const boardPayload = {
  board: {
    workspace: workspaceOne,
    workflow,
    task_counts: { total: 2, completed: 1, pending: 1 },
    lanes: [
      {
        status: "pending",
        title: "Pending",
        items: [
          {
            task_id: "task_01",
            task_number: 1,
            title: "Bootstrap workspace",
            status: "pending",
            type: "frontend",
            depends_on: [],
            updated_at: "2026-01-01T00:00:00Z",
          },
        ],
      },
      {
        status: "completed",
        title: "Completed",
        items: [
          {
            task_id: "task_00",
            task_number: 0,
            title: "Scaffold repo",
            status: "completed",
            type: "infra",
            depends_on: [],
            updated_at: "2026-01-02T00:00:00Z",
          },
        ],
      },
    ],
  },
};

const taskDetailPayload = {
  task: {
    workspace: workspaceOne,
    workflow,
    task: {
      task_id: "task_01",
      task_number: 1,
      title: "Bootstrap workspace",
      status: "pending",
      type: "frontend",
      depends_on: [],
      updated_at: "2026-01-01T00:00:00Z",
    },
    document: {
      id: "doc-1",
      kind: "task",
      title: "Bootstrap workspace",
      updated_at: "2026-01-01T00:00:00Z",
      markdown: "## Bootstrap\nSetup the workspace.",
    },
    memory_entries: [],
    related_runs: [
      {
        run_id: "run-1",
        mode: "task",
        presentation_mode: "text",
        workspace_id: "ws-1",
        started_at: "2026-01-01T00:10:00Z",
        status: "completed",
        ended_at: "2026-01-01T00:12:00Z",
        workflow_slug: "alpha",
      },
    ],
    live_tail_available: false,
  },
};

function matchUrl(pattern: string, method: string = "GET") {
  return (input: RequestInfo | URL, init?: RequestInit) => {
    const url =
      typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
    const requestMethod =
      input instanceof Request ? input.method.toUpperCase() : (init?.method ?? "GET").toUpperCase();
    return url.includes(pattern) && requestMethod === method.toUpperCase();
  };
}

async function renderApp(initialPath: string) {
  const queryClient = createTestQueryClient();
  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: [initialPath] }),
    defaultPreload: false,
  });
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
  await router.load();
  return { router, queryClient };
}

describe("workflow tasks integration", () => {
  let restore: (() => void) | null = null;

  beforeEach(async () => {
    resetActiveWorkspaceStoreForTests();
    window.sessionStorage.clear();
    const { useActiveWorkspaceStore } =
      await import("../systems/app-shell/stores/active-workspace-store");
    useActiveWorkspaceStore.setState({ selectedWorkspaceId: "ws-1" });
  });

  afterEach(() => {
    restore?.();
    restore = null;
    resetActiveWorkspaceStoreForTests();
    vi.clearAllMocks();
  });

  it("Should render the workflow task board against the typed daemon contract", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchUrl("/api/tasks/alpha/board"),
        status: 200,
        body: boardPayload,
      },
    ]);
    restore = stub.restore;
    await renderApp("/workflows/alpha/tasks");
    expect(await screen.findByTestId("task-board-count-total")).toHaveTextContent("2");
    expect(screen.getByTestId("task-board-row-task_01")).toBeInTheDocument();
    expect(screen.getByTestId("task-board-row-task_00")).toBeInTheDocument();
    const boardCalls = stub.calls.filter(call => call.url.includes("/api/tasks/alpha/board"));
    expect(boardCalls[0]?.headers["x-compozy-workspace-id"]).toBe("ws-1");
  });

  it("Should render the board empty state when the workflow has zero tasks", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchUrl("/api/tasks/empty/board"),
        status: 200,
        body: {
          board: {
            workspace: workspaceOne,
            workflow: { ...workflow, slug: "empty" },
            task_counts: { total: 0, completed: 0, pending: 0 },
            lanes: [],
          },
        },
      },
    ]);
    restore = stub.restore;
    await renderApp("/workflows/empty/tasks");
    await screen.findByTestId("task-board-empty");
  });

  it("Should render the board error state when the daemon returns not-found", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchUrl("/api/tasks/ghost/board"),
        status: 404,
        body: { code: "workflow_not_found", message: "workflow missing", request_id: "r" },
      },
    ]);
    restore = stub.restore;
    await renderApp("/workflows/ghost/tasks");
    const alert = await screen.findByTestId("task-board-error");
    expect(alert).toHaveTextContent("workflow missing");
  });

  it("Should render task detail with related runs against the typed contract", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchUrl("/api/tasks/alpha/items/task_01"),
        status: 200,
        body: taskDetailPayload,
      },
    ]);
    restore = stub.restore;
    await renderApp("/workflows/alpha/tasks/task_01");
    await screen.findByTestId("task-detail-view");
    expect(screen.getByTestId("task-detail-status")).toHaveTextContent("pending");
    expect(screen.getByTestId("task-detail-run-link-run-1")).toHaveTextContent("run-1");
  });

  it("Should surface a not-found alert for stale or invalid task identifiers", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchUrl("/api/tasks/alpha/items/task_missing"),
        status: 404,
        body: { code: "task_not_found", message: "task missing", request_id: "r" },
      },
    ]);
    restore = stub.restore;
    await renderApp("/workflows/alpha/tasks/task_missing");
    const alert = await screen.findByTestId("task-detail-load-error");
    expect(alert).toHaveTextContent("task missing");
  });

  it("Should link from the workflow inventory into the task board", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchUrl("/api/tasks", "GET"),
        status: 200,
        body: { workflows: [workflow] },
      },
      {
        matcher: matchUrl("/api/tasks/alpha/board"),
        status: 200,
        body: boardPayload,
      },
    ]);
    restore = stub.restore;
    await renderApp("/workflows");
    await screen.findByTestId("workflow-inventory-view");
    const boardLink = await screen.findByTestId("workflow-view-board-alpha");
    await userEvent.click(boardLink);
    await waitFor(() => {
      expect(screen.getByTestId("task-board-view")).toBeInTheDocument();
    });
  });

  it("Should start a workflow run from the workflow inventory", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchUrl("/api/tasks", "GET"),
        status: 200,
        body: { workflows: [workflow] },
      },
      {
        matcher: matchUrl("/api/tasks/alpha/runs", "POST"),
        status: 201,
        body: {
          run: {
            run_id: "run-new",
            mode: "task",
            presentation_mode: "text",
            workspace_id: "ws-1",
            started_at: "2026-01-01T00:00:00Z",
            status: "queued",
            workflow_slug: "alpha",
          },
        },
      },
    ]);
    restore = stub.restore;
    await renderApp("/workflows");
    await screen.findByTestId("workflow-row-alpha");
    await userEvent.click(screen.getByTestId("workflow-start-alpha"));
    await screen.findByTestId("workflow-inventory-start-success");
    const startCalls = stub.calls.filter(
      call => call.url.includes("/api/tasks/alpha/runs") && call.method === "POST"
    );
    expect(startCalls).toHaveLength(1);
    expect(startCalls[0]?.headers["x-compozy-workspace-id"]).toBe("ws-1");
    expect(JSON.parse(startCalls[0]?.body ?? "{}")).toMatchObject({
      workspace: "ws-1",
      presentation_mode: "detach",
    });
    const runLink = screen.getByTestId(
      "workflow-inventory-start-success-link"
    ) as HTMLAnchorElement;
    expect(runLink.getAttribute("href")).toBe("/runs/run-new");
  });
});
