import { fireEvent, render, screen } from "@testing-library/react";
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
  it("Should expose exact approval progress and keep heartbeat diagnostics out of the rail", () => {
    const onApprove = vi.fn();
    render(
      <TaskPropertiesRail
        approvalPending={{ approve: true }}
        detail={buildDetailFixture({
          task: { approval_state: "pending", status: "blocked" },
          summary: { status: "blocked" },
        } as never)}
        onApprove={onApprove}
        onAutoEnqueueChange={vi.fn()}
        onEditSetup={vi.fn()}
        onInspect={vi.fn()}
        onPriorityChange={vi.fn()}
        onReject={vi.fn()}
        runs={[]}
      />
    );

    const approve = screen.getByTestId("tasks-rail-approve");
    expect(approve).toBeDisabled();
    expect(approve).toHaveAttribute("aria-busy", "true");
    expect(approve).toHaveTextContent("Approving…");
    expect(screen.queryByText(/heartbeat/i)).toBeNull();

    fireEvent.click(approve);
    expect(onApprove).not.toHaveBeenCalled();
  });
});
