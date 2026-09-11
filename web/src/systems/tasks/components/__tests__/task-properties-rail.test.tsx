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

  // Invariant: the approval mutation buttons register only on the local
  // surface set, so without their handlers they are absent, never disabled
  // (BR-1); the approval read rows stay on every tier.
  // Owning layer: TaskPropertiesRail composition.
  // Canonical suite: TaskPropertiesRail component tests.
  it("Should omit the approval actions when their handlers are not offered", () => {
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

    expect(screen.getByText("Pending")).toBeInTheDocument();
    expect(screen.queryByTestId("tasks-rail-approve")).not.toBeInTheDocument();
    expect(screen.queryByTestId("tasks-rail-reject")).not.toBeInTheDocument();
  });

  // Invariant: priority/auto-enqueue persist through task PATCH (UpdateTask),
  // a local-only mutation route, so the inline editors are absent, never
  // disabled, when their handlers are not offered (BR-1); the read rows keep
  // the current values on every tier.
  // Owning layer: TaskPropertiesRail composition.
  // Canonical suite: TaskPropertiesRail component tests.
  it("Should keep the priority and auto-enqueue read rows without their editors", () => {
    render(
      <TaskPropertiesRail
        detail={buildDetailFixture()}
        onEditSetup={vi.fn()}
        onInspect={vi.fn()}
        runs={[]}
      />
    );

    expect(screen.queryByTestId("tasks-rail-priority")).not.toBeInTheDocument();
    expect(screen.queryByTestId("tasks-rail-auto-enqueue")).not.toBeInTheDocument();
    expect(screen.getByText("Priority")).toBeInTheDocument();
    expect(screen.getByText("High")).toBeInTheDocument();
    expect(screen.getByText("Auto-enqueue")).toBeInTheDocument();
    expect(screen.getByText("Off")).toBeInTheDocument();
  });

  it("Should render the priority and auto-enqueue editors when their handlers are offered", () => {
    render(
      <TaskPropertiesRail
        detail={buildDetailFixture()}
        onAutoEnqueueChange={vi.fn()}
        onEditSetup={vi.fn()}
        onInspect={vi.fn()}
        onPriorityChange={vi.fn()}
        runs={[]}
      />
    );

    expect(screen.getByTestId("tasks-rail-priority")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-rail-auto-enqueue")).toBeInTheDocument();
    expect(screen.queryByText("Off")).not.toBeInTheDocument();
  });

  // Invariant: the setup editor saves through the execution-profile PUT
  // (SetTaskExecutionProfile), a local-only mutation route, so the "Edit
  // setup" entry is absent, never disabled, when its handler is not offered
  // (BR-1); the execution read rows keep the current values on every tier.
  // Owning layer: TaskPropertiesRail composition.
  // Canonical suite: TaskPropertiesRail component tests.
  it("Should omit the Edit setup entry when its handler is not offered", () => {
    render(<TaskPropertiesRail detail={buildDetailFixture()} onInspect={vi.fn()} runs={[]} />);

    expect(screen.queryByTestId("tasks-rail-edit-setup")).not.toBeInTheDocument();
    expect(screen.getByText("Attempts")).toBeInTheDocument();
    expect(screen.getByText("Auto-enqueue")).toBeInTheDocument();
  });

  it("Should render the Edit setup entry when its handler is offered", () => {
    const onEditSetup = vi.fn();
    render(
      <TaskPropertiesRail
        detail={buildDetailFixture()}
        onEditSetup={onEditSetup}
        onInspect={vi.fn()}
        runs={[]}
      />
    );

    fireEvent.click(screen.getByTestId("tasks-rail-edit-setup"));
    expect(onEditSetup).toHaveBeenCalledTimes(1);
  });
});
