import { render, screen } from "@testing-library/react";
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
    expect(screen.getByTestId("task-card-children-task_001")).toHaveTextContent("2 children");
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
});
