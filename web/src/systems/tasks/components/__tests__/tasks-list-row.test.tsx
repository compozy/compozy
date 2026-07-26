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

const { TasksListRow } = await import("../tasks-list-row");
type TaskListItem = import("../../types").TaskListItem;

function buildTask(overrides: Partial<TaskListItem> = {}): TaskListItem {
  return {
    id: "task_abcdef0_tail",
    title: "Summarize feedback",
    identifier: "TASK-1",
    status: "in_progress",
    scope: "workspace",
    origin: { kind: "web", ref: "op" },
    created_at: "2026-04-11T09:00:00Z",
    updated_at: "2026-04-11T09:00:00Z",
    created_by: { kind: "human", ref: "op" },
    ...overrides,
  } as TaskListItem;
}

describe("TasksListRow", () => {
  it("omits the status-dot column by default", () => {
    const { container, rerender } = render(
      <TasksListRow task={buildTask({ status: "completed" })} />
    );
    expect(container.querySelector('[data-slot="status-dot"]')).toBeNull();
    expect(container.querySelector('[data-slot="tasks-list-row-dot"]')).toBeNull();

    rerender(<TasksListRow task={buildTask({ status: "in_progress" })} />);
    expect(container.querySelector('[data-slot="status-dot"]')).toBeNull();
    expect(container.querySelector('[data-slot="tasks-list-row-dot"]')).toBeNull();
  });

  it("renders the identifier as bare mono text (proposal `.task-row__id`, not a Pill)", () => {
    render(<TasksListRow task={buildTask({ identifier: "TASK-42" })} />);
    const id = screen.getByText("task-42").closest('[data-slot="tasks-list-row-id"]');
    expect(id).not.toBeNull();
    expect(id).toHaveAttribute("data-slot", "tasks-list-row-id");
  });

  it("falls back to the 7-character short id when the identifier is absent", () => {
    render(<TasksListRow task={buildTask({ identifier: undefined })} />);
    const id = screen.getByText("task_ab").closest('[data-slot="tasks-list-row-id"]');
    expect(id).not.toBeNull();
    expect(id).toHaveAttribute("data-slot", "tasks-list-row-id");
  });

  it("links the main region to /tasks/$id", () => {
    render(<TasksListRow task={buildTask({ id: "task_xyz" })} />);
    const link = screen.getByRole("link", { name: "Open Summarize feedback" });
    expect(link).toHaveAttribute("href", "/tasks/$id");
    expect(link).toHaveAttribute("data-params", JSON.stringify({ id: "task_xyz" }));
  });

  it("keeps trail content outside the link region", () => {
    render(
      <TasksListRow
        task={buildTask({ id: "task_trail" })}
        trailing={<span data-testid="trail-pill">High</span>}
      />
    );
    const link = screen.getByRole("link", { name: "Open Summarize feedback" });
    const trail = screen.getByTestId("trail-pill");
    expect(link).not.toContainElement(trail);
  });
});
