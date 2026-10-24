import { render, screen, within } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({ to, children, ...props }: Record<string, unknown>) => (
      <a href={typeof to === "string" ? to : "#"} {...(props as Record<string, unknown>)}>
        {children as ReactNode}
      </a>
    ),
  };
});

import {
  buildDetailFixture,
  buildTaskRunRecordFixture,
  taskExecutionProfileFixture,
} from "../../mocks/fixtures";
import { TaskPropertiesRail } from "../task-properties-rail";

describe("TaskPropertiesRail", () => {
  it("Should keep approval, ids, CLI hints, and heartbeat diagnostics out of the rail", () => {
    render(
      <TaskPropertiesRail
        detail={buildDetailFixture({
          task: { approval_state: "pending", status: "blocked" },
          summary: { status: "blocked" },
        } as never)}
        onAutoEnqueueChange={vi.fn()}
        onEditSetup={vi.fn()}
        onInspect={vi.fn()}
        onPriorityChange={vi.fn()}
        runs={[]}
      />
    );

    const rail = screen.getByTestId("tasks-detail-rail");
    expect(screen.queryByTestId("tasks-rail-approve")).toBeNull();
    expect(screen.queryByTestId("tasks-rail-reject")).toBeNull();
    expect(rail).not.toHaveTextContent(/Task id|Run id|compozy task inspect/);
    expect(screen.queryByText(/heartbeat/i)).toBeNull();
    expect(screen.getByTestId("tasks-rail-inspect")).toBeInTheDocument();
  });

  it("Should hold placeholders for what the profile and run history have not answered yet", () => {
    const failed = buildDetailFixture({
      task: { status: "failed" },
      summary: { active_run: null, status: "failed" },
    } as never);
    const props = {
      detail: failed,
      onAutoEnqueueChange: vi.fn(),
      onEditSetup: vi.fn(),
      onInspect: vi.fn(),
      onPriorityChange: vi.fn(),
      runs: [],
    };
    const { rerender } = render(<TaskPropertiesRail {...props} profileLoading runsLoading />);

    const rail = screen.getByTestId("tasks-detail-rail");
    // Each unanswered read is one named loading area, and the real controls stay in place.
    expect(
      within(rail)
        .getAllByRole("status")
        .map(region => region.textContent)
    ).toEqual(["Loading last run", "Loading setup"]);
    expect(within(rail).queryByText("Agent")).not.toBeInTheDocument();
    expect(within(rail).getByText("Attempts")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-rail-edit-setup")).toBeEnabled();

    // Answered, the facts take the placeholders' places and nothing keeps announcing a load.
    rerender(
      <TaskPropertiesRail
        {...props}
        profile={taskExecutionProfileFixture}
        runs={[
          buildTaskRunRecordFixture({
            ended_at: "2026-04-17T10:09:12Z",
            started_at: "2026-04-17T10:01:00Z",
            status: "failed",
          }),
        ]}
      />
    );
    expect(within(rail).queryByRole("status")).not.toBeInTheDocument();
    expect(within(rail).getByText("Agent")).toBeInTheDocument();
    expect(within(rail).getByText("Last run")).toBeInTheDocument();
  });
});
