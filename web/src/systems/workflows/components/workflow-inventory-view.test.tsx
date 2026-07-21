import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { act, render, screen, waitFor } from "@testing-library/react";
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

const initiativeWorkflow: WorkflowSummary = {
  id: "wf-initiative",
  kind: "initiative",
  slug: "customer-management",
  workspace_id: "ws-1",
  can_start_run: false,
  start_block_reason: "select a task group",
  task_groups: [
    {
      workflow_id: "wf-task-group-1",
      task_group_id: "TG-001",
      reference: "customer-management/TG-001",
      title: "Persistence",
      outcome: "Persist customer records.",
      lifecycle_complete: true,
      unmet_dependency_count: 0,
      independently_eligible: false,
      task_counts: { total: 2, completed: 2, pending: 0 },
      can_start_run: false,
    },
    {
      workflow_id: "wf-task-group-2",
      task_group_id: "TG-002",
      reference: "customer-management/TG-002",
      title: "Interface",
      outcome: "Render customer records.",
      lifecycle_complete: false,
      unmet_dependency_count: 1,
      independently_eligible: false,
      dependencies: [
        {
          task_group_id: "TG-001",
          title: "Persistence",
          rationale: "API contract first",
        },
      ],
      unmet_dependencies: [
        {
          task_group_id: "TG-001",
          title: "Persistence",
          rationale: "API contract first",
        },
      ],
      task_counts: { total: 3, completed: 1, pending: 2 },
      can_start_run: false,
      start_block_reason: "1 unmet dependency",
    },
  ],
};

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

  it("Should render and filter accessible Task Groups only beneath their initiative", async () => {
    // CONTRACT: IT-055, E2E-004, E2E-006.
    const onStartRun = vi.fn();
    await renderInventory({
      ...defaults,
      onArchive: () => {},
      onStartRun,
      onSyncAll: () => {},
      onSyncOne: () => {},
      workflows: [initiativeWorkflow],
    });

    expect(screen.getByTestId("workflow-row-customer-management")).toBeInTheDocument();
    expect(screen.getByTestId("workflow-task-groups-customer-management")).toHaveTextContent(
      "Task Groups · 2"
    );
    expect(screen.queryByTestId("workflow-row-customer-management/TG-001")).not.toBeInTheDocument();
    const taskGroupLink = screen.getByRole("link", {
      name: /TG-002, Interface.*lifecycle incomplete.*1 unmet dependency/i,
    }) as HTMLAnchorElement;
    expect(taskGroupLink.getAttribute("href")).toBe(
      "/workflows/customer-management/tasks?task_group_id=TG-002"
    );
    expect(screen.getByTestId("workflow-task-group-lifecycle-TG-001")).toHaveTextContent(
      "Git integration is not tracked"
    );
    expect(screen.getByTestId("workflow-task-group-dependencies-TG-002")).toHaveTextContent(
      "API contract first"
    );

    const filter = screen.getByTestId("workflow-task-groups-filter-customer-management");
    await userEvent.type(filter, "TG-002");
    expect(
      screen.queryByTestId("workflow-task-group-customer-management-TG-001")
    ).not.toBeInTheDocument();
    expect(
      screen.getByTestId("workflow-task-group-customer-management-TG-002")
    ).toBeInTheDocument();

    expect(screen.getByText("1 unmet dependency")).toBeInTheDocument();
    expect(onStartRun).not.toHaveBeenCalled();
  });

  it("Should require task group selection before starting an initiative run", async () => {
    const onStartRun = vi.fn();
    const readyTaskGroup = {
      workflow_id: "wf-task-group-3",
      task_group_id: "TG-003",
      reference: "customer-management/TG-003",
      title: "Notifications",
      outcome: "Send customer notifications.",
      lifecycle_complete: false,
      unmet_dependency_count: 0,
      independently_eligible: false,
      task_counts: { total: 1, completed: 0, pending: 1 },
      can_start_run: true,
    };
    await renderInventory({
      ...defaults,
      onArchive: () => {},
      onStartRun,
      onSyncAll: () => {},
      onSyncOne: () => {},
      workflows: [{ ...initiativeWorkflow, task_groups: [readyTaskGroup] }],
    });

    expect(screen.queryByTestId("workflow-start-customer-management")).not.toBeInTheDocument();
    expect(screen.getByTestId("workflow-start-blocked-customer-management")).toHaveTextContent(
      "select a task group"
    );
    await userEvent.click(
      screen.getByTestId("workflow-task-group-start-customer-management-TG-003")
    );
    expect(onStartRun).toHaveBeenCalledWith("customer-management", { taskGroupId: "TG-003" });
  });

  it("Should require a single explicit dependency override without changing task group readiness", async () => {
    let resolveStart: (() => void) | undefined;
    const onStartRun = vi.fn(
      () =>
        new Promise<void>(resolve => {
          resolveStart = resolve;
        })
    );
    const blockedTaskGroup = {
      ...initiativeWorkflow.task_groups![1]!,
      can_start_run: true,
      requires_start_confirmation: true,
      start_block_reason: undefined,
    };
    await renderInventory({
      ...defaults,
      onArchive: () => {},
      onStartRun,
      onSyncAll: () => {},
      onSyncOne: () => {},
      workflows: [{ ...initiativeWorkflow, task_groups: [blockedTaskGroup] }],
    });

    const startButton = screen.getByTestId("workflow-task-group-start-customer-management-TG-002");
    await userEvent.click(startButton);
    expect(onStartRun).not.toHaveBeenCalled();
    expect(
      screen.getByTestId("workflow-task-group-dependency-confirmation-customer-management-TG-002")
    ).toHaveTextContent("Persistence");
    expect(
      screen.getByTestId(
        "workflow-task-group-dependency-confirmation-dependencies-customer-management-TG-002"
      )
    ).toHaveTextContent("API contract first");

    await userEvent.click(
      screen.getByTestId(
        "workflow-task-group-dependency-confirmation-cancel-customer-management-TG-002"
      )
    );
    expect(onStartRun).not.toHaveBeenCalled();

    await userEvent.click(startButton);
    const confirmButton = screen.getByTestId(
      "workflow-task-group-dependency-confirmation-confirm-customer-management-TG-002"
    );
    await userEvent.click(confirmButton);
    expect(onStartRun).toHaveBeenCalledWith("customer-management", {
      taskGroupId: "TG-002",
      allowOutOfOrder: true,
    });
    expect(confirmButton).toBeDisabled();
    await userEvent.click(confirmButton);
    expect(onStartRun).toHaveBeenCalledTimes(1);

    expect(screen.getByTestId("workflow-task-group-readiness-TG-002")).toHaveTextContent(
      "1 unmet dependency"
    );
    expect(screen.getByTestId("workflow-task-group-dependencies-TG-002")).toHaveTextContent(
      "API contract first"
    );
    resolveStart?.();
    await waitFor(() => {
      expect(
        screen.queryByTestId(
          "workflow-task-group-dependency-confirmation-customer-management-TG-002"
        )
      ).not.toBeInTheDocument();
    });
  });

  it("Should include transitive dependency context before authorizing a task group run", async () => {
    const onStartRun = vi.fn();
    const transitiveTaskGroup = {
      ...initiativeWorkflow.task_groups![1]!,
      can_start_run: true,
      requires_start_confirmation: true,
      unmet_dependencies: [],
      unmet_dependency_paths: [
        {
          task_group_ids: ["TG-001", "TG-002"],
          dependencies: [
            {
              task_group_id: "TG-001",
              title: "Persistence",
              rationale: "API contract first",
            },
          ],
        },
      ],
      start_block_reason: undefined,
    };
    await renderInventory({
      ...defaults,
      onArchive: () => {},
      onStartRun,
      onSyncAll: () => {},
      onSyncOne: () => {},
      workflows: [{ ...initiativeWorkflow, task_groups: [transitiveTaskGroup] }],
    });

    await userEvent.click(
      screen.getByTestId("workflow-task-group-start-customer-management-TG-002")
    );
    expect(onStartRun).not.toHaveBeenCalled();
    expect(
      screen.getByTestId(
        "workflow-task-group-dependency-confirmation-dependencies-customer-management-TG-002"
      )
    ).toHaveTextContent("Transitive path: TG-001 → TG-002");
    expect(
      screen.getByTestId(
        "workflow-task-group-dependency-confirmation-dependencies-customer-management-TG-002"
      )
    ).toHaveTextContent("Persistence");
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
