import { render, screen } from "@testing-library/react";
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

import { buildTaskChildFixture } from "../../mocks/fixtures";
import { TaskSubtasksSection } from "../task-subtasks-section";

describe("TaskSubtasksSection", () => {
  it("Should render progress, non-color state labels, and task deep links", () => {
    render(
      <TaskSubtasksSection
        items={[
          buildTaskChildFixture({ id: "task_done", status: "completed", title: "Done child" }),
          buildTaskChildFixture({
            id: "task_active",
            status: "in_progress",
            title: "Active child",
          }),
          buildTaskChildFixture({ id: "task_todo", status: "ready", title: "Todo child" }),
        ]}
      />
    );

    expect(
      screen.getByLabelText("1 done, 1 active, 1 not started; 3 subtasks total")
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Done child, Completed" })).toHaveAttribute(
      "data-params",
      JSON.stringify({ id: "task_done" })
    );
    expect(screen.getByRole("link", { name: "Active child, Active" })).toHaveAttribute(
      "data-params",
      JSON.stringify({ id: "task_active" })
    );
    expect(screen.getByRole("link", { name: "Todo child, Not started" })).toHaveAttribute(
      "data-params",
      JSON.stringify({ id: "task_todo" })
    );
  });
});
