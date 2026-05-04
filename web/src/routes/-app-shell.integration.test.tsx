import { QueryClientProvider } from "@tanstack/react-query";
import { createMemoryHistory, createRouter, RouterProvider } from "@tanstack/react-router";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

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

const workspaceTwo = {
  id: "ws-2",
  name: "two",
  root_dir: "/tmp/two",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

const dashboardPayload = {
  dashboard: {
    workspace: workspaceOne,
    daemon: {
      active_run_count: 0,
      http_port: 2123,
      pid: 42,
      started_at: "2026-01-01T00:00:00Z",
      version: "0.2.0",
      workspace_count: 1,
    },
    health: { ready: true, degraded: false, details: [] },
    pending_reviews: 3,
    queue: { active: 0, canceled: 0, completed: 4, failed: 0, total: 4 },
    workflows: [
      {
        workflow: { id: "wf-1", slug: "alpha", workspace_id: "ws-1" },
        active_runs: 0,
        review_round_count: 0,
        task_completed: 3,
        task_pending: 1,
        task_total: 4,
      },
    ],
    active_runs: [],
  },
};

function matchUrl(path: string, method: string = "GET") {
  return (input: RequestInfo | URL, init?: RequestInit) => {
    const url =
      typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
    return url.endsWith(path) && (init?.method ?? "GET").toUpperCase() === method.toUpperCase();
  };
}

async function renderApp(initialPath: string = "/") {
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

describe("app shell integration", () => {
  let restore: (() => void) | null = null;

  beforeEach(() => {
    resetActiveWorkspaceStoreForTests();
    window.sessionStorage.clear();
  });

  afterEach(() => {
    restore?.();
    restore = null;
    resetActiveWorkspaceStoreForTests();
  });

  it("Should show onboarding when the daemon has no workspaces", async () => {
    const stub = installFetchStub([
      { matcher: matchUrl("/api/workspaces"), status: 200, body: { workspaces: [] } },
    ]);
    restore = stub.restore;
    await renderApp();
    expect(await screen.findByTestId("workspace-onboarding")).toBeInTheDocument();
  });

  it("Should render the dashboard after selecting one of many workspaces", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne, workspaceTwo] },
      },
      {
        matcher: matchUrl("/api/ui/dashboard"),
        status: 200,
        body: dashboardPayload,
      },
    ]);
    restore = stub.restore;
    await renderApp();
    expect(await screen.findByTestId("workspace-picker-list")).toBeInTheDocument();
    await userEvent.click(screen.getByTestId("workspace-picker-select-ws-1"));
    expect(await screen.findByTestId("dashboard-view")).toBeInTheDocument();
    expect(screen.getByTestId("app-shell-active-workspace-name")).toHaveTextContent("one");
  });

  it("Should render the dashboard immediately when only one workspace exists", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchUrl("/api/ui/dashboard"),
        status: 200,
        body: dashboardPayload,
      },
    ]);
    restore = stub.restore;
    await renderApp();
    expect(await screen.findByTestId("dashboard-view")).toBeInTheDocument();
    expect(screen.getByTestId("dashboard-queue-completed")).toHaveTextContent("4");
  });

  it("Should fall back to the workspace picker after a stale workspace error", async () => {
    window.sessionStorage.setItem("compozy.web.active-workspace", "ws-gone");
    const { useActiveWorkspaceStore } =
      await import("../systems/app-shell/stores/active-workspace-store");
    useActiveWorkspaceStore.setState({ selectedWorkspaceId: "ws-gone" });

    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne, workspaceTwo] },
      },
    ]);
    restore = stub.restore;
    await renderApp();
    expect(await screen.findByTestId("workspace-picker-list")).toBeInTheDocument();
    expect(screen.getByTestId("workspace-picker-stale")).toBeInTheDocument();
    await waitFor(() =>
      expect(window.sessionStorage.getItem("compozy.web.active-workspace")).toBeNull()
    );
  });

  it("Should trigger sync all from the dashboard", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchUrl("/api/ui/dashboard"),
        status: 200,
        body: dashboardPayload,
      },
      {
        matcher: matchUrl("/api/sync", "POST"),
        status: 200,
        body: {
          workspace_id: "ws-1",
          workflows_scanned: 2,
          task_items_upserted: 7,
        },
      },
    ]);
    restore = stub.restore;
    await renderApp();
    expect(await screen.findByTestId("dashboard-view")).toBeInTheDocument();
    await userEvent.click(screen.getByTestId("dashboard-sync-all"));
    await waitFor(
      () => {
        const syncCalls = stub.calls.filter(call => call.url.includes("/api/sync"));
        expect(syncCalls).toHaveLength(1);
        expect(syncCalls[0]?.method).toBe("POST");
        expect(syncCalls[0]?.headers["x-compozy-workspace-id"]).toBe("ws-1");
      },
      { timeout: 3000 }
    );
  });

  it("Should sync and archive from the workflow inventory route", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchUrl("/api/tasks"),
        status: 200,
        body: {
          workflows: [{ id: "wf-1", slug: "alpha", workspace_id: "ws-1" }],
        },
      },
      {
        matcher: matchUrl("/api/sync", "POST"),
        status: 200,
        body: { workflow_slug: "alpha", workspace_id: "ws-1", task_items_upserted: 2 },
      },
      {
        matcher: matchUrl("/api/tasks/alpha/archive", "POST"),
        status: 200,
        body: { archived: true, archived_at: "2026-02-01T00:00:00Z" },
      },
    ]);
    restore = stub.restore;
    await renderApp("/workflows");
    const syncButton = await screen.findByTestId("workflow-sync-alpha", undefined, {
      timeout: 3000,
    });
    await userEvent.click(syncButton);
    await waitFor(
      () => {
        const syncCalls = stub.calls.filter(call => call.url.includes("/api/sync"));
        expect(syncCalls).toHaveLength(1);
        expect(JSON.parse(syncCalls[0]?.body ?? "{}")).toMatchObject({
          workflow_slug: "alpha",
          workspace: "ws-1",
        });
      },
      { timeout: 3000 }
    );
    await userEvent.click(screen.getByTestId("workflow-archive-alpha"));
    await waitFor(
      () => {
        const archiveCalls = stub.calls.filter(call =>
          call.url.includes("/api/tasks/alpha/archive")
        );
        expect(archiveCalls).toHaveLength(1);
        expect(archiveCalls[0]?.method).toBe("POST");
        expect(JSON.parse(archiveCalls[0]?.body ?? "{}")).toMatchObject({ workspace: "ws-1" });
      },
      { timeout: 3000 }
    );
  });
});
