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

import { TaskDetailView } from "./task-detail-view";
import type { TaskDetailPayload } from "../types";

interface RenderProps {
  payload: TaskDetailPayload;
  isRefreshing?: boolean;
}

async function renderDetail(props: RenderProps) {
  const rootRoute = createRootRoute();
  const detailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: function RootView(): ReactElement {
      return <TaskDetailView isRefreshing={props.isRefreshing ?? false} payload={props.payload} />;
    },
  });
  const boardStub = createRoute({
    getParentRoute: () => rootRoute,
    path: "/workflows/$slug/tasks",
    component: function BoardStub(): ReactElement {
      return <div data-testid="board-stub" />;
    },
  });
  const runStub = createRoute({
    getParentRoute: () => rootRoute,
    path: "/runs/$runId",
    component: function RunStub(): ReactElement {
      return <div data-testid="run-stub" />;
    },
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([detailRoute, boardStub, runStub]),
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
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

const workflow = { id: "wf-1", slug: "alpha", workspace_id: "ws-1" };

const fullPayload: TaskDetailPayload = {
  workspace,
  workflow,
  task: {
    task_id: "task_02",
    task_number: 2,
    title: "Implement board",
    status: "in_progress",
    type: "frontend",
    depends_on: ["task_01"],
    updated_at: "2026-01-02T00:00:00Z",
  },
  document: {
    id: "task-doc",
    kind: "task",
    title: "Implement board",
    updated_at: "2026-01-02T00:00:00Z",
    markdown: "## Task body\nDo the work.",
  },
  memory_entries: [
    {
      file_id: "mem-1",
      display_path: "memory/task_02.md",
      kind: "task",
      title: "task_02 notes",
      size_bytes: 42,
      updated_at: "2026-01-02T00:00:00Z",
    },
  ],
  related_runs: [
    {
      run_id: "run-7",
      mode: "task",
      presentation_mode: "text",
      workspace_id: "ws-1",
      started_at: "2026-01-02T00:01:00Z",
      status: "running",
      workflow_slug: "alpha",
    },
  ],
  live_tail_available: true,
};

const sparsePayload: TaskDetailPayload = {
  workspace,
  workflow,
  task: {
    task_id: "task_03",
    task_number: 3,
    title: "Wire tests",
    status: "pending",
    type: "frontend",
    updated_at: "2026-01-03T00:00:00Z",
  },
  document: {
    id: "task-doc",
    kind: "task",
    title: "Wire tests",
    updated_at: "2026-01-03T00:00:00Z",
    markdown: "",
  },
  live_tail_available: false,
};

describe("TaskDetailView", () => {
  it("Should render the task metadata, document body, dependencies, memory, and related runs", async () => {
    await renderDetail({ payload: fullPayload });
    expect(screen.getByTestId("task-detail-view")).toBeInTheDocument();
    expect(screen.getByTestId("task-detail-status")).toHaveTextContent("in_progress");
    expect(screen.getByTestId("task-detail-document-body")).toHaveTextContent("Task body");
    expect(screen.getByTestId("task-detail-dependency-task_01")).toBeInTheDocument();
    expect(screen.getByTestId("task-detail-memory-row-mem-1")).toHaveTextContent("task_02 notes");
    const runLink = screen.getByTestId("task-detail-run-link-run-7") as HTMLAnchorElement;
    expect(runLink.getAttribute("href")).toBe("/runs/run-7");
    const boardLink = screen.getByTestId("task-detail-back-to-board") as HTMLAnchorElement;
    expect(boardLink.getAttribute("href")).toBe("/workflows/alpha/tasks");
  });

  it("Should render empty states for dependencies, memory, and related runs", async () => {
    await renderDetail({ payload: sparsePayload });
    expect(screen.getByTestId("task-detail-dependencies-empty")).toBeInTheDocument();
    expect(screen.getByTestId("task-detail-related-runs-empty")).toBeInTheDocument();
    expect(screen.getByTestId("task-detail-memory-empty")).toBeInTheDocument();
    expect(screen.getByTestId("task-detail-document-empty")).toBeInTheDocument();
  });

  it("Should surface the refreshing indicator", async () => {
    await renderDetail({ payload: fullPayload, isRefreshing: true });
    expect(screen.getByTestId("task-detail-refreshing")).toBeInTheDocument();
  });
});
