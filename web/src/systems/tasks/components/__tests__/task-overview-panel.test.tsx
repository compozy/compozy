import { fireEvent, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({ to, params, children, ...props }: Record<string, unknown>) => (
      <a
        data-params={JSON.stringify(params)}
        href={typeof to === "string" ? to : "#"}
        {...(props as Record<string, unknown>)}
      >
        {children as ReactNode}
      </a>
    ),
  };
});

import {
  buildDetailFixture,
  buildTaskChildFixture,
  buildTaskDependencyReferenceFixture,
  buildTaskFixture,
  buildTaskRecordFixture,
  buildTaskRunFixture,
} from "../../mocks/fixtures";
import type { TaskNowStripHandlers } from "../task-now-strip";
import { TaskOverviewPanel } from "../task-overview-panel";

function buildHandlers(): TaskNowStripHandlers {
  return {
    onApprove: vi.fn(),
    onClearBlock: vi.fn(),
    onOpenRun: vi.fn(),
    onOpenTask: vi.fn(),
    onRecover: vi.fn(),
    onReject: vi.fn(),
    onResume: vi.fn(),
    onViewResult: vi.fn(),
  };
}

describe("TaskOverviewPanel", () => {
  it("Should render description fallback, child states, deep links, and resolved dependencies", () => {
    const resolvedDependency = buildTaskDependencyReferenceFixture({
      depends_on: buildTaskRecordFixture(
        buildTaskFixture({
          id: "task_dep_done",
          status: "completed",
          title: "Archived dependency",
        })
      ),
    });
    const detail = buildDetailFixture({
      task: { description: " ", status: "ready" },
      summary: { active_run: null, status: "ready" },
      children: [
        buildTaskChildFixture({ id: "task_child_done", status: "completed", title: "Done child" }),
        buildTaskChildFixture({
          id: "task_child_active",
          status: "in_progress",
          title: "Active child",
        }),
        buildTaskChildFixture({ id: "task_child_todo", status: "ready", title: "Todo child" }),
      ],
      dependency_references: [buildTaskDependencyReferenceFixture(), resolvedDependency],
    } as never);

    render(
      <TaskOverviewPanel
        detail={detail}
        isLive={false}
        nowHandlers={buildHandlers()}
        onViewAllActivity={vi.fn()}
        runs={[]}
        timeline={[]}
      />
    );

    expect(screen.getByText("No description.")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Subtasks" })).toBeInTheDocument();
    expect(screen.getByText("Done child")).toBeInTheDocument();
    expect(
      screen.getByRole("link", {
        name: "Finalize compliance sign-off on public timeout copy, Not started",
      })
    ).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Archived dependency, Completed" })).toBeNull();

    fireEvent.click(screen.getByTestId("tasks-detail-resolved-toggle"));
    expect(
      screen.getByRole("link", { name: "Archived dependency, Completed" })
    ).toBeInTheDocument();
  });

  it("Should forward approval, blocking-task, and active-run actions to the controller", () => {
    const handlers = buildHandlers();
    const detail = buildDetailFixture({
      task: {
        approval_state: "pending",
        blocked_reasons: [
          {
            source: "dependency",
            depends_on_task_ids: ["task_dep_001"],
            reason: "Waiting for compliance",
          },
          {
            source: "dependency",
            depends_on_task_ids: ["task_dep_002"],
            reason: "Waiting for release approval",
          },
          {
            block_id: "block_001",
            kind: "needs_input",
            source: "block",
            reason: "Choose a release channel",
          },
          {
            block_id: "block_002",
            kind: "capability",
            source: "block",
            reason: "Grant deploy access",
          },
        ],
        status: "blocked",
      },
      summary: {
        active_run: buildTaskRunFixture({ id: "run_active", status: "running" }),
        status: "blocked",
      },
    } as never);

    render(
      <TaskOverviewPanel
        detail={detail}
        isLive
        nowHandlers={handlers}
        onViewAllActivity={vi.fn()}
        runs={[]}
        timeline={[]}
      />
    );

    fireEvent.click(screen.getByTestId("tasks-detail-now-approve"));
    fireEvent.click(screen.getByTestId("tasks-detail-now-reject"));
    fireEvent.click(screen.getByTestId("tasks-detail-now-open-blocking-task_dep_001"));
    fireEvent.click(screen.getByTestId("tasks-detail-now-open-blocking-task_dep_002"));
    fireEvent.click(screen.getByTestId("tasks-detail-now-clear-block-block_001"));
    fireEvent.click(screen.getByTestId("tasks-detail-now-clear-block-block_002"));
    fireEvent.click(screen.getByTestId("tasks-detail-now-open-run"));

    expect(handlers.onApprove).toHaveBeenCalledTimes(1);
    expect(handlers.onReject).toHaveBeenCalledTimes(1);
    expect(handlers.onOpenTask).toHaveBeenNthCalledWith(1, "task_dep_001");
    expect(handlers.onOpenTask).toHaveBeenNthCalledWith(2, "task_dep_002");
    expect(handlers.onClearBlock).toHaveBeenNthCalledWith(1, "block_001");
    expect(handlers.onClearBlock).toHaveBeenNthCalledWith(2, "block_002");
    expect(handlers.onOpenRun).toHaveBeenCalledWith("run_active");
  });
});
