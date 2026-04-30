import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactElement } from "react";
import { describe, expect, it, vi } from "vitest";

import type { WorkflowSummary } from "../types";
import { WorkflowInventoryView } from "./workflow-inventory-view";

const workflows: WorkflowSummary[] = [
  { id: "wf-1", slug: "alpha", workspace_id: "ws-1", last_synced_at: "2026-01-01T00:00:00Z" },
  {
    id: "wf-done",
    slug: "completed-workflow-with-a-title-that-should-not-push-into-badges",
    workspace_id: "ws-1",
    last_synced_at: "2026-01-02T00:00:00Z",
    task_counts: { total: 2, completed: 2, pending: 0 },
    can_start_run: false,
    start_block_reason: "no pending tasks",
  },
  {
    id: "wf-2",
    slug: "beta",
    workspace_id: "ws-1",
    archived_at: "2026-02-01T00:00:00Z",
  },
];

const defaults = {
  isLoading: false,
  isRefetching: false,
  workspaceName: "one",
  isSyncingAll: false,
  pendingSyncSlug: null,
  pendingStartSlug: null,
  pendingArchiveSlug: null,
  startedRun: null,
};

type ViewProps = Parameters<typeof WorkflowInventoryView>[0];

async function renderInventory(props: ViewProps) {
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: function IndexRoute(): ReactElement {
      return <WorkflowInventoryView {...props} />;
    },
  });
  const boardRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/workflows/$slug/tasks",
    component: function BoardStub(): ReactElement {
      return <div data-testid="board-stub" />;
    },
  });
  const runRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/runs/$runId",
    component: function RunStub(): ReactElement {
      return <div data-testid="run-stub" />;
    },
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute, boardRoute, runRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    defaultPreload: false,
  });
  await router.load();
  await act(async () => {
    render(<RouterProvider router={router} />);
    await Promise.resolve();
  });
}

describe("WorkflowInventoryView", () => {
  it("Should render active and archived workflows in separate sections", async () => {
    await renderInventory({
      ...defaults,
      onArchive: () => {},
      onStartRun: () => {},
      onSyncAll: () => {},
      onSyncOne: () => {},
      workflows,
    });
    expect(screen.getByTestId("workflow-inventory-active")).toHaveTextContent("alpha");
    expect(screen.getByTestId("workflow-inventory-archived")).toHaveTextContent("beta");
    const openLink = screen.getByTestId("workflow-view-board-alpha") as HTMLAnchorElement;
    expect(openLink.getAttribute("href")).toBe("/workflows/alpha/tasks");
  });

  it("Should show the empty state when no workflows exist", async () => {
    await renderInventory({
      ...defaults,
      onArchive: () => {},
      onStartRun: () => {},
      onSyncAll: () => {},
      onSyncOne: () => {},
      workflows: [],
    });
    expect(screen.getByTestId("workflow-inventory-empty")).toBeInTheDocument();
  });

  it("Should fire sync-all, start-run, sync-one, and archive handlers", async () => {
    const onSyncAll = vi.fn();
    const onStartRun = vi.fn();
    const onSyncOne = vi.fn();
    const onArchive = vi.fn();
    await renderInventory({
      ...defaults,
      onArchive,
      onStartRun,
      onSyncAll,
      onSyncOne,
      workflows: [workflows[0]!],
    });
    await userEvent.click(screen.getByTestId("workflow-inventory-sync-all"));
    await userEvent.click(screen.getByTestId("workflow-start-alpha"));
    await userEvent.click(screen.getByTestId("workflow-sync-alpha"));
    await userEvent.click(screen.getByTestId("workflow-archive-alpha"));
    expect(onSyncAll).toHaveBeenCalledTimes(1);
    expect(onStartRun).toHaveBeenCalledWith("alpha");
    expect(onSyncOne).toHaveBeenCalledWith("alpha");
    expect(onArchive).toHaveBeenCalledWith("alpha");
  });

  it("Should replace start-run with a completed state when no tasks are pending", async () => {
    const completedWorkflow = workflows[1]!;
    await renderInventory({
      ...defaults,
      onArchive: () => {},
      onStartRun: () => {},
      onSyncAll: () => {},
      onSyncOne: () => {},
      workflows: [completedWorkflow],
    });
    expect(
      screen.queryByTestId(`workflow-start-${completedWorkflow.slug}`)
    ).not.toBeInTheDocument();
    expect(
      screen.getByTestId(`workflow-start-blocked-${completedWorkflow.slug}`)
    ).toHaveTextContent("completed");
  });

  it("Should disable filesystem actions when the workspace is read-only", async () => {
    const onSyncAll = vi.fn();
    const onStartRun = vi.fn();
    const onSyncOne = vi.fn();
    const onArchive = vi.fn();
    await renderInventory({
      ...defaults,
      isReadOnly: true,
      onArchive,
      onStartRun,
      onSyncAll,
      onSyncOne,
      workflows: [workflows[0]!],
    });
    expect(screen.getByTestId("workflow-inventory-readonly")).toBeInTheDocument();
    expect(screen.getByTestId("workflow-inventory-sync-all")).toBeDisabled();
    expect(screen.getByTestId("workflow-start-alpha")).toBeDisabled();
    expect(screen.getByTestId("workflow-sync-alpha")).toBeDisabled();
    expect(screen.getByTestId("workflow-archive-alpha")).toBeDisabled();
    await userEvent.click(screen.getByTestId("workflow-inventory-sync-all"));
    await userEvent.click(screen.getByTestId("workflow-start-alpha"));
    await userEvent.click(screen.getByTestId("workflow-sync-alpha"));
    await userEvent.click(screen.getByTestId("workflow-archive-alpha"));
    expect(onSyncAll).not.toHaveBeenCalled();
    expect(onStartRun).not.toHaveBeenCalled();
    expect(onSyncOne).not.toHaveBeenCalled();
    expect(onArchive).not.toHaveBeenCalled();
  });

  it("Should surface load and action errors", async () => {
    await renderInventory({
      ...defaults,
      error: "load failed",
      lastActionError: "sync blew up",
      onArchive: () => {},
      onStartRun: () => {},
      onSyncAll: () => {},
      onSyncOne: () => {},
      workflows,
    });
    expect(screen.getByTestId("workflow-inventory-load-error")).toHaveTextContent("load failed");
    expect(screen.getByTestId("workflow-inventory-error")).toHaveTextContent("sync blew up");
  });

  it("Should render the started run banner with a run detail link", async () => {
    await renderInventory({
      ...defaults,
      onArchive: () => {},
      onStartRun: () => {},
      onSyncAll: () => {},
      onSyncOne: () => {},
      startedRun: {
        run_id: "run-42",
        mode: "task",
        presentation_mode: "text",
        workspace_id: "ws-1",
        started_at: "2026-01-01T00:00:00Z",
        status: "queued",
        workflow_slug: "alpha",
      },
      workflows: [workflows[0]!],
    });
    expect(screen.getByTestId("workflow-inventory-start-success")).toHaveTextContent("run-42");
    const link = screen.getByTestId("workflow-inventory-start-success-link") as HTMLAnchorElement;
    expect(link.getAttribute("href")).toBe("/runs/run-42");
  });
});
