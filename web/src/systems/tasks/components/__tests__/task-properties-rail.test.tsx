import { render, screen } from "@testing-library/react";
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

import { buildDetailFixture } from "../../mocks/fixtures";
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
});
