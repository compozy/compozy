import { fireEvent, render, screen } from "@testing-library/react";
import type { ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";

import { TasksKanbanBoard as TasksKanbanBoardComponent } from "../tasks-kanban-board";
import { getKanbanColumns, groupTasksForKanban } from "../../lib/task-grouping";
import { countTasksByStatus } from "../../lib/task-formatters";
import type { TaskListItem } from "../../types";

function buildTask(overrides: Partial<TaskListItem> = {}): TaskListItem {
  return {
    id: overrides.id ?? "task_001",
    title: overrides.title ?? "Generate API client",
    identifier: overrides.identifier ?? "TASK-1",
    status: overrides.status ?? "ready",
    scope: "workspace",
    origin: { kind: "web", ref: "op" },
    created_at: "2026-04-11T09:00:00Z",
    updated_at: "2026-04-11T09:00:00Z",
    created_by: { kind: "human", ref: "op" },
    owner: { kind: "agent_session", ref: "claude" },
    ...overrides,
  } as TaskListItem;
}

type TestBoardProps = Omit<ComponentProps<typeof TasksKanbanBoardComponent>, "statusCounts"> & {
  statusCounts?: ComponentProps<typeof TasksKanbanBoardComponent>["statusCounts"];
};

function TasksKanbanBoard({ statusCounts, ...props }: TestBoardProps) {
  const tasks = props.columns.flatMap(column => column.tasks);
  return (
    <TasksKanbanBoardComponent
      {...props}
      statusCounts={statusCounts ?? countTasksByStatus(tasks)}
    />
  );
}

describe("TasksKanbanBoard", () => {
  it("Should render the canonical columns including a distinct Needs attention column", () => {
    render(
      <TasksKanbanBoard
        columns={groupTasksForKanban([])}
        onSelectTask={vi.fn()}
        selectedTaskId={null}
      />
    );

    const board = screen.getByTestId("tasks-kanban-board");
    expect(board).toBeInTheDocument();
    const columns = screen.getAllByRole("listitem");
    // The rendered column count AND the grid track count must both derive from
    // the canonical column set, so a future status column can never wrap onto a
    // broken second row (guards the round-4 B-001 regression).
    expect(columns).toHaveLength(getKanbanColumns().length);
    expect(board.getAttribute("style")).toContain(
      `repeat(${getKanbanColumns().length}, minmax(0, 1fr))`
    );
    expect(screen.getByTestId("tasks-kanban-column-pending")).toHaveTextContent(/Pending/);
    expect(screen.getByTestId("tasks-kanban-column-in_progress")).toHaveTextContent(/In progress/);
    expect(screen.getByTestId("tasks-kanban-column-blocked")).toHaveTextContent(/Blocked/);
    expect(screen.getByTestId("tasks-kanban-column-needs_attention")).toHaveTextContent(
      /Needs attention/
    );
    expect(screen.getByTestId("tasks-kanban-column-done")).toHaveTextContent(/Done/);
  });

  it("Should route an in-progress task into the In progress column", () => {
    const tasks = [buildTask({ id: "live", status: "in_progress" })];

    render(
      <TasksKanbanBoard
        columns={groupTasksForKanban(tasks)}
        onSelectTask={vi.fn()}
        selectedTaskId={null}
      />
    );

    expect(screen.getByTestId("tasks-kanban-card-live")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-kanban-column-in_progress")).toContainElement(
      screen.getByTestId("tasks-kanban-card-live")
    );
    expect(screen.getByTestId("tasks-kanban-column-empty-pending")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-kanban-column-empty-blocked")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-kanban-column-empty-done")).toBeInTheDocument();
  });

  it("Should collapse draft, pending, and ready into Pending; blocked into Blocked", () => {
    const tasks = [
      buildTask({ id: "d", status: "draft" }),
      buildTask({ id: "p", status: "pending" }),
      buildTask({ id: "r", status: "ready" }),
      buildTask({ id: "b", status: "blocked" }),
    ];

    render(
      <TasksKanbanBoard
        columns={groupTasksForKanban(tasks)}
        onSelectTask={vi.fn()}
        selectedTaskId={null}
      />
    );

    const pendingColumn = screen.getByTestId("tasks-kanban-column-pending");
    for (const id of ["d", "p", "r"]) {
      expect(pendingColumn).toContainElement(screen.getByTestId(`tasks-kanban-card-${id}`));
    }
    expect(screen.getByTestId("tasks-kanban-column-blocked")).toContainElement(
      screen.getByTestId("tasks-kanban-card-b")
    );
  });

  it("Should collapse terminal statuses (completed, failed, canceled) into Done", () => {
    const tasks = [
      buildTask({ id: "c", status: "completed" }),
      buildTask({ id: "f", status: "failed" }),
      buildTask({ id: "x", status: "canceled" }),
    ];

    render(
      <TasksKanbanBoard
        columns={groupTasksForKanban(tasks)}
        onSelectTask={vi.fn()}
        selectedTaskId={null}
      />
    );

    const doneColumn = screen.getByTestId("tasks-kanban-column-done");
    for (const id of ["c", "f", "x"]) {
      expect(doneColumn).toContainElement(screen.getByTestId(`tasks-kanban-card-${id}`));
    }
  });

  it("Should emit selection events when a card is clicked", () => {
    const onSelectTask = vi.fn();
    const tasks = [buildTask({ id: "a", status: "ready" })];

    render(
      <TasksKanbanBoard
        columns={groupTasksForKanban(tasks)}
        onSelectTask={onSelectTask}
        selectedTaskId={null}
      />
    );

    fireEvent.click(screen.getByTestId("tasks-kanban-card-a"));
    expect(onSelectTask).toHaveBeenCalledWith("a");
  });

  it("Should expose a generic create action without promising a target column", () => {
    const onCreate = vi.fn();
    render(
      <TasksKanbanBoard
        columns={groupTasksForKanban([])}
        onCreate={onCreate}
        onSelectTask={vi.fn()}
        selectedTaskId={null}
      />
    );

    const createButtons = screen.getAllByRole("button", { name: "Create task" });
    expect(createButtons).toHaveLength(getKanbanColumns().length);
    const firstCreateButton = createButtons[0];
    if (!firstCreateButton) throw new Error("expected a generic create button");
    fireEvent.click(firstCreateButton);
    expect(onCreate).toHaveBeenCalledWith();
    expect(screen.queryByRole("button", { name: /Add task to/i })).not.toBeInTheDocument();
  });

  it("Should render a retry affordance for failed cards and surface their error", () => {
    const onRetryTask = vi.fn();
    const tasks = [
      buildTask({
        id: "fail",
        status: "failed",
        active_run: {
          id: "run_fail",
          task_id: "fail",
          attempt: 3,
          max_attempts: 3,
          recovery_count: 0,
          status: "failed",
          queued_at: "2026-04-11T09:00:00Z",
          error: "boom",
        },
      }),
    ];

    render(
      <TasksKanbanBoard
        columns={groupTasksForKanban(tasks)}
        onRetryTask={onRetryTask}
        onSelectTask={vi.fn()}
        selectedTaskId={null}
      />
    );

    expect(screen.getByTestId("tasks-kanban-card-error-fail")).toHaveTextContent("boom");
    fireEvent.click(screen.getByTestId("tasks-kanban-card-retry-fail"));
    expect(onRetryTask).toHaveBeenCalledWith("run_fail");
  });

  it("Should render loading and error states without crashing", () => {
    const { rerender } = render(
      <TasksKanbanBoard
        columns={groupTasksForKanban([])}
        isLoading
        onSelectTask={vi.fn()}
        selectedTaskId={null}
      />
    );
    expect(screen.getByTestId("tasks-kanban-loading")).toBeInTheDocument();
    expect(screen.queryByTestId("tasks-kanban-column-count-pending")).not.toBeInTheDocument();

    rerender(
      <TasksKanbanBoard
        columns={groupTasksForKanban([])}
        errorMessage="oops"
        onSelectTask={vi.fn()}
        selectedTaskId={null}
      />
    );
    expect(screen.getByTestId("tasks-kanban-error")).toHaveTextContent("oops");
  });

  it("Should continue the backend-ordered board through an accessible control", () => {
    const onLoadMore = vi.fn();
    render(
      <TasksKanbanBoard
        columns={groupTasksForKanban([buildTask()])}
        hasMore
        onLoadMore={onLoadMore}
        onSelectTask={vi.fn()}
        selectedTaskId={null}
      />
    );

    fireEvent.click(screen.getByRole("button", { name: "Load more tasks" }));
    expect(onLoadMore).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("tasks-kanban-card-task_001")).toBeInTheDocument();
  });

  it("Should retry the failed catalog operation without requesting another page", () => {
    const onLoadMore = vi.fn();
    const onRetry = vi.fn();
    render(
      <TasksKanbanBoard
        columns={groupTasksForKanban([buildTask()])}
        errorMessage="Catalog refresh failed"
        hasMore
        onLoadMore={onLoadMore}
        onRetryLoad={onRetry}
        onSelectTask={vi.fn()}
        selectedTaskId={null}
      />
    );

    fireEvent.click(screen.getByRole("button", { name: "Retry loading tasks" }));
    expect(onRetry).toHaveBeenCalledTimes(1);
    expect(onLoadMore).not.toHaveBeenCalled();
  });

  it("Should label partial columns as loaded of the exact facet total", () => {
    const tasks = [buildTask({ status: "ready" })];
    const statusCounts = countTasksByStatus([]);
    statusCounts.ready = 10;
    render(
      <TasksKanbanBoard
        columns={groupTasksForKanban(tasks)}
        onSelectTask={vi.fn()}
        selectedTaskId={null}
        statusCounts={statusCounts}
      />
    );

    expect(screen.getByTestId("tasks-kanban-column-count-pending")).toHaveTextContent("1 of 10");
  });
});

describe("TaskKanbanCard", () => {
  it("Should render the OwnerAvatar primitive (no plain text owner fallback alone)", () => {
    const tasks = [
      buildTask({
        id: "owned",
        owner: { kind: "agent_session", ref: "claude" },
      }),
    ];

    render(
      <TasksKanbanBoard
        columns={groupTasksForKanban(tasks)}
        onSelectTask={vi.fn()}
        selectedTaskId={null}
      />
    );

    expect(screen.getByTestId("tasks-kanban-card-avatar-owned")).toHaveAttribute(
      "data-slot",
      "owner-avatar"
    );
  });

  it("Should render needs_attention cards with a human status label", () => {
    const tasks = [buildTask({ id: "attention", status: "needs_attention" })];
    render(
      <TasksKanbanBoard
        columns={groupTasksForKanban(tasks)}
        onSelectTask={vi.fn()}
        selectedTaskId={null}
      />
    );

    const card = screen.getByTestId("tasks-kanban-card-attention");
    expect(card).toHaveTextContent("Needs attention");
    expect(card).not.toHaveTextContent("needs_attention");
  });

  it("Should paint the card with an inset ring instead of a border class", () => {
    const tasks = [buildTask({ id: "ring" })];
    render(
      <TasksKanbanBoard
        columns={groupTasksForKanban(tasks)}
        onSelectTask={vi.fn()}
        selectedTaskId={null}
      />
    );

    const card = screen.getByTestId("tasks-kanban-card-ring");
    expect(card.className).toContain("shadow-hairline-inset");
    expect(card.className).not.toContain("border-line");
  });

  it("Should not render an accent rail when the card is selected", () => {
    const tasks = [buildTask({ id: "sel" })];
    const { container } = render(
      <TasksKanbanBoard
        columns={groupTasksForKanban(tasks)}
        onSelectTask={vi.fn()}
        selectedTaskId="sel"
      />
    );

    const card = screen.getByTestId("tasks-kanban-card-sel");
    expect(card).toHaveAttribute("data-selected", "true");
    expect(card.querySelector("[class*='bg-accent']")).toBeNull();
    // Ensure no accent rail wrapper anywhere inside the card.
    expect(container.querySelectorAll(".bg-\\(--color-accent\\)").length).toBe(0);
  });
});
