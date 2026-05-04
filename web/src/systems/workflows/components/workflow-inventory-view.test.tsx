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
    archive_eligible: true,
  },
  {
    id: "wf-2",
    slug: "beta",
    workspace_id: "ws-1",
    archived_at: "2026-02-01T00:00:00Z",
  },
];

const defaults = {
  archiveConfirmation: null,
  isLoading: false,
  isRefetching: false,
  workspaceName: "one",
  isSyncingAll: false,
  onCancelArchiveConfirmation: () => {},
  onConfirmArchiveConfirmation: () => {},
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
  it("Should render active, completed, and archived workflows in separate sections", async () => {
    await renderInventory({
      ...defaults,
      onArchive: () => {},
      onStartRun: () => {},
      onSyncAll: () => {},
      onSyncOne: () => {},
      workflows,
    });
    const completedSlug = workflows[1]!.slug;
    expect(screen.getByTestId("workflow-inventory-active")).toHaveTextContent("alpha");
    expect(screen.getByTestId("workflow-inventory-active")).not.toHaveTextContent(completedSlug);
    expect(screen.getByTestId("workflow-inventory-completed")).toHaveTextContent(completedSlug);
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

  it("Should drop the start-run button and surface a completed header badge when no tasks are pending", async () => {
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
      screen.queryByTestId(`workflow-start-blocked-${completedWorkflow.slug}`)
    ).not.toBeInTheDocument();
    expect(screen.getByTestId(`workflow-row-${completedWorkflow.slug}`)).toHaveTextContent(
      /completed/i
    );
  });

  it("Should place completed workflows in the Completed section while keeping Sync and Archive", async () => {
    const completedWorkflow = workflows[1]!;
    await renderInventory({
      ...defaults,
      onArchive: () => {},
      onStartRun: () => {},
      onSyncAll: () => {},
      onSyncOne: () => {},
      workflows,
    });
    const section = screen.getByTestId("workflow-inventory-completed");
    expect(section).toHaveTextContent(`Completed · 1`);
    expect(section).toHaveTextContent(completedWorkflow.slug);
    expect(
      screen.queryByTestId(`workflow-start-${completedWorkflow.slug}`)
    ).not.toBeInTheDocument();
    expect(screen.getByTestId(`workflow-sync-${completedWorkflow.slug}`)).toBeInTheDocument();
    expect(screen.getByTestId(`workflow-archive-${completedWorkflow.slug}`)).toBeInTheDocument();
  });

  it("Should classify resolved review-only workflows as completed from archive eligibility", async () => {
    const reviewOnlyWorkflow: WorkflowSummary = {
      id: "wf-review-only",
      slug: "review-only",
      workspace_id: "ws-1",
      last_synced_at: "2026-01-03T00:00:00Z",
      task_counts: { total: 0, completed: 0, pending: 0 },
      can_start_run: true,
      archive_eligible: true,
    };
    await renderInventory({
      ...defaults,
      onArchive: () => {},
      onStartRun: () => {},
      onSyncAll: () => {},
      onSyncOne: () => {},
      workflows: [reviewOnlyWorkflow],
    });
    expect(screen.queryByTestId("workflow-inventory-active")).not.toBeInTheDocument();
    expect(screen.getByTestId("workflow-inventory-completed")).toHaveTextContent("review-only");
    expect(screen.queryByTestId("workflow-start-review-only")).not.toBeInTheDocument();
    expect(screen.getByTestId("workflow-archive-review-only")).toBeInTheDocument();
  });

  it("Should keep unresolved review-only workflows active even when no tasks can start", async () => {
    const unresolvedReviewWorkflow: WorkflowSummary = {
      id: "wf-review-pending",
      slug: "review-pending",
      workspace_id: "ws-1",
      last_synced_at: "2026-01-03T00:00:00Z",
      task_counts: { total: 0, completed: 0, pending: 0 },
      can_start_run: false,
      start_block_reason: "no pending tasks",
      archive_eligible: false,
      archive_reason: "review rounds not fully resolved",
    };
    await renderInventory({
      ...defaults,
      onArchive: () => {},
      onStartRun: () => {},
      onSyncAll: () => {},
      onSyncOne: () => {},
      workflows: [unresolvedReviewWorkflow],
    });
    expect(screen.getByTestId("workflow-inventory-active")).toHaveTextContent("review-pending");
    expect(screen.queryByTestId("workflow-inventory-completed")).not.toBeInTheDocument();
    expect(screen.getByTestId("workflow-start-blocked-review-pending")).toHaveTextContent(
      "no pending tasks"
    );
    expect(screen.getByTestId("workflow-archive-review-pending")).toBeInTheDocument();
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

  it("Should render archive confirmation details and fire cancel/confirm handlers", async () => {
    const onCancelArchiveConfirmation = vi.fn();
    const onConfirmArchiveConfirmation = vi.fn();
    await renderInventory({
      ...defaults,
      archiveConfirmation: {
        slug: "alpha",
        archiveReason: "review rounds not fully resolved",
        taskNonTerminal: 2,
        reviewUnresolved: 3,
        reviewTotal: 4,
      },
      onArchive: () => {},
      onCancelArchiveConfirmation,
      onConfirmArchiveConfirmation,
      onStartRun: () => {},
      onSyncAll: () => {},
      onSyncOne: () => {},
      workflows: [workflows[0]!],
    });
    expect(screen.getByTestId("workflow-archive-confirmation")).toBeInTheDocument();
    expect(screen.getByTestId("workflow-archive-confirmation-tasks")).toHaveTextContent(
      "2 tasks will be marked as completed"
    );
    expect(screen.getByTestId("workflow-archive-confirmation-reviews")).toHaveTextContent(
      "3 review issues will be resolved locally out of 4 issues"
    );

    await userEvent.click(screen.getByTestId("workflow-archive-confirmation-cancel"));
    expect(onCancelArchiveConfirmation).toHaveBeenCalledTimes(1);

    await userEvent.click(screen.getByTestId("workflow-archive-confirmation-confirm"));
    expect(onConfirmArchiveConfirmation).toHaveBeenCalledWith("alpha");
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
