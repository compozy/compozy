import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({ to, params, children, ...props }: Record<string, unknown>) => (
      <a
        href={typeof to === "string" ? to : "#"}
        data-params={JSON.stringify(params)}
        {...(props as Record<string, unknown>)}
      >
        {children as React.ReactNode}
      </a>
    ),
  };
});

const { TaskCard } = await import("../task-card");
type TaskListItem = import("../../types").TaskListItem;

function buildTask(overrides: Partial<TaskListItem> = {}): TaskListItem {
  return {
    id: "task_001",
    title: "Summarize feedback",
    identifier: "TASK-1",
    status: "in_progress",
    scope: "workspace",
    origin: { kind: "web", ref: "op" },
    created_at: "2026-04-11T09:00:00Z",
    updated_at: "2026-04-11T09:00:00Z",
    created_by: { kind: "human", ref: "op" },
    owner: { kind: "agent_session", ref: "Coder" },
    priority: "high",
    child_count: 2,
    dependency_count: 1,
    active_run: {
      id: "run_001",
      task_id: "task_001",
      attempt: 2,
      max_attempts: 3,
      status: "running",
      queued_at: "2026-04-11T09:00:00Z",
    },
    ...overrides,
  } as TaskListItem;
}

describe("TaskCard", () => {
  it("Should render enriched task data inline through the meta slot", () => {
    const { container } = render(<TaskCard task={buildTask()} />);

    expect(screen.getByTestId("task-card-task_001")).toBeInTheDocument();
    expect(screen.getByText("task-1")).toBeInTheDocument();
    expect(screen.getByText("Summarize feedback")).toBeInTheDocument();
    expect(screen.getByTestId("task-card-owner-task_001")).toHaveTextContent("Coder");
    expect(screen.getByTestId("task-card-attempt-task_001")).toHaveTextContent("attempt 2 of 3");
    expect(screen.getByTestId("task-card-children-task_001")).toHaveTextContent("2 subtasks");
    expect(screen.getByTestId("task-card-deps-task_001")).toHaveTextContent("1 dep");
    expect(container.querySelector('[data-slot="status-dot"]')).toBeNull();
    expect(screen.getByText("High")).toBeInTheDocument();
  });

  it("Should link the main region to /tasks/$id", () => {
    render(<TaskCard task={buildTask()} />);

    const link = screen.getByRole("link", { name: "Open Summarize feedback" });
    expect(link).toHaveAttribute("href", "/tasks/$id");
    expect(link).toHaveAttribute("data-params", JSON.stringify({ id: "task_001" }));
  });

  it("Should render the failed-run error inline in the meta row (no inline retry button)", () => {
    render(
      <TaskCard
        task={buildTask({
          status: "failed",
          active_run: {
            id: "run_002",
            task_id: "task_001",
            attempt: 3,
            max_attempts: 3,
            recovery_count: 0,
            status: "failed",
            queued_at: "2026-04-11T09:00:00Z",
            error: "rate-limited by upstream",
          },
        })}
      />
    );

    expect(screen.getByTestId("task-card-error-task_001")).toHaveTextContent(
      "rate-limited by upstream"
    );
    expect(screen.queryByTestId("task-card-retry-task_001")).not.toBeInTheDocument();
  });

  it("Should not render a publish button on draft rows (publish lives on the detail header)", () => {
    render(<TaskCard task={buildTask({ status: "draft", draft: true, active_run: null })} />);
    expect(screen.queryByTestId("task-card-publish-task_001")).not.toBeInTheDocument();
  });

  it("Should render a Blocked pill in the trailing slot for blocked tasks", () => {
    render(<TaskCard task={buildTask({ status: "blocked", active_run: null })} />);
    expect(screen.getByTestId("task-card-blocked-task_001")).toBeInTheDocument();
    expect(screen.queryByTestId("task-card-needs-attention-task_001")).not.toBeInTheDocument();
  });

  it("Should surface needs_attention as its own truthful pill, distinct from blocked", () => {
    render(<TaskCard task={buildTask({ status: "needs_attention", active_run: null })} />);

    const pill = screen.getByTestId("task-card-needs-attention-task_001");
    expect(pill).toHaveTextContent("Needs attention");
    expect(screen.queryByTestId("task-card-blocked-task_001")).not.toBeInTheDocument();
  });

  it("Should nest subtasks behind a collapsed summary that keeps escalations visible", () => {
    const parent = buildTask({ id: "loop.run-1.coordinator", child_count: 2, active_run: null });
    const subtasks = [
      buildTask({
        id: "loop.run-1.g2.node.execute_task.0",
        title: "Loop delivery node execute_task",
        identifier: undefined,
        status: "needs_attention",
        active_run: null,
        parent_task_id: "loop.run-1.coordinator",
      }),
      buildTask({
        id: "loop.run-1.g2.node.review.0",
        title: "Loop delivery node review",
        identifier: undefined,
        status: "in_progress",
        active_run: null,
        parent_task_id: "loop.run-1.coordinator",
      }),
    ];
    render(<TaskCard subtasks={subtasks} task={parent} />);

    const toggle = screen.getByTestId("task-subtasks-toggle-loop.run-1.coordinator");
    expect(toggle).toHaveTextContent("2 subtasks · 1 needs attention · 1 running");
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(
      screen.queryByTestId("task-subtask-row-loop.run-1.g2.node.execute_task.0")
    ).not.toBeInTheDocument();

    fireEvent.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(
      screen.getByTestId("task-subtask-row-loop.run-1.g2.node.execute_task.0")
    ).toBeInTheDocument();
    expect(screen.getByText("g2.execute_task")).toBeInTheDocument();
  });

  it("Should drop the task-level attempt ceiling for loop cells (the loop owns retries)", () => {
    render(
      <TaskCard
        task={buildTask({
          id: "loop.run-1.g1.node.execute_task.0",
          identifier: undefined,
          active_run: {
            id: "run_cell",
            task_id: "loop.run-1.g1.node.execute_task.0",
            attempt: 1,
            max_attempts: 10,
            recovery_count: 0,
            status: "queued",
            queued_at: "2026-04-11T09:00:00Z",
          },
        })}
      />
    );

    const attempt = screen.getByTestId("task-card-attempt-loop.run-1.g1.node.execute_task.0");
    expect(attempt).toHaveTextContent("attempt 1");
    expect(attempt).not.toHaveTextContent("of 10");
  });
});
