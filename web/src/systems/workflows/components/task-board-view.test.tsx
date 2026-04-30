import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { act, render, screen } from "@testing-library/react";
import type { ReactElement } from "react";
import { describe, expect, it } from "vitest";

import { TaskBoardView, resolveStatusTone } from "./task-board-view";
import type { TaskBoardPayload } from "../types";

interface RenderProps {
  board?: TaskBoardPayload;
  isLoading?: boolean;
  isRefetching?: boolean;
  error?: string | null;
}

async function renderBoard(props: RenderProps = {}) {
  const rootRoute = createRootRoute();
  const listRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: function ListRoute(): ReactElement {
      return (
        <TaskBoardView
          board={props.board}
          error={props.error ?? null}
          isLoading={props.isLoading ?? false}
          isRefetching={props.isRefetching ?? false}
          workflowSlug="alpha"
          workspaceName="one"
        />
      );
    },
  });
  const detailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/workflows/$slug/tasks/$taskId",
    component: function DetailStub(): ReactElement {
      return <div data-testid="detail-stub" />;
    },
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([listRoute, detailRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    defaultPreload: false,
  });
  await router.load();
  await act(async () => {
    render(<RouterProvider router={router} />);
    await Promise.resolve();
  });
}

const workspace = {
  id: "ws-1",
  name: "one",
  root_dir: "/tmp/one",
  filesystem_state: "present",
  read_only: false,
  has_catalog_data: true,
  workflow_count: 1,
  run_count: 0,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
} as const;

const workflow = { id: "wf-1", slug: "alpha", workspace_id: "ws-1" };

const populatedBoard: TaskBoardPayload = {
  workspace,
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
};

const emptyBoard: TaskBoardPayload = {
  workspace,
  workflow,
  task_counts: { total: 0, completed: 0, pending: 0 },
  lanes: [],
};

describe("TaskBoardView", () => {
  it("Should render lanes and task rows with deep links", async () => {
    await renderBoard({ board: populatedBoard });
    expect(screen.getByTestId("task-board-view")).toBeInTheDocument();
    expect(screen.getByTestId("task-board-count-total")).toHaveTextContent("2");
    expect(screen.getByTestId("task-board-lane-pending")).toBeInTheDocument();
    const link = screen.getByTestId("task-board-link-task_01") as HTMLAnchorElement;
    expect(link.getAttribute("href")).toBe("/workflows/alpha/tasks/task_01");
    expect(link).toHaveClass("truncate");
    expect(screen.getByTestId("task-board-status-task_01")).toHaveTextContent("pending");
  });

  it("Should render the empty state when the workflow has no tasks", async () => {
    await renderBoard({ board: emptyBoard });
    expect(screen.getByTestId("task-board-empty")).toBeInTheDocument();
  });

  it("Should render the loading state", async () => {
    await renderBoard({ isLoading: true });
    expect(screen.getByTestId("task-board-loading")).toBeInTheDocument();
  });

  it("Should render the error alert when provided", async () => {
    await renderBoard({ error: "boom" });
    expect(screen.getByTestId("task-board-error")).toHaveTextContent("boom");
  });

  it("Should render the refreshing indicator", async () => {
    await renderBoard({ board: populatedBoard, isRefetching: true });
    expect(screen.getByTestId("task-board-refreshing")).toBeInTheDocument();
  });
});

describe("resolveStatusTone (task)", () => {
  it("Should map statuses to tones", () => {
    expect(resolveStatusTone("completed")).toBe("success");
    expect(resolveStatusTone("in_progress")).toBe("accent");
    expect(resolveStatusTone("failed")).toBe("danger");
    expect(resolveStatusTone("needs_review")).toBe("warning");
    expect(resolveStatusTone("pending")).toBe("info");
    expect(resolveStatusTone("unknown")).toBe("neutral");
  });
});
