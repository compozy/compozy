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

import type { TaskCommandState } from "../../lib/task-command-state";
import { TaskPageActions, TaskPageOverflow } from "../task-page-head";

const command: TaskCommandState = {
  canFanOut: false,
  overflow: {
    cancel: true,
    delete: false,
    edit: false,
    pause: false,
    resume: false,
    startNewRun: false,
  },
  primary: { kind: "approve" },
  secondary: { edit: false, pause: false, reject: true },
};

const handlers = {
  onApprove: vi.fn(),
  onEdit: vi.fn(),
  onOpenRun: vi.fn(),
  onPause: vi.fn(),
  onPublish: vi.fn(),
  onRecover: vi.fn(),
  onReject: vi.fn(),
  onResume: vi.fn(),
  onRetry: vi.fn(),
  onStartRun: vi.fn(),
};

describe("TaskPageActions", () => {
  it("Should identify the exact primary mutation that is pending", () => {
    render(<TaskPageActions command={command} handlers={handlers} pending={{ approve: true }} />);

    const approve = screen.getByTestId("tasks-detail-primary-approve");
    expect(approve).toBeDisabled();
    expect(approve).toHaveAttribute("aria-busy", "true");
    expect(approve).toHaveTextContent("Approving…");
    expect(screen.getByTestId("tasks-detail-reject-button")).toHaveTextContent("Reject");
  });

  it("Should identify a pending secondary rejection without relabeling approval", () => {
    render(<TaskPageActions command={command} handlers={handlers} pending={{ reject: true }} />);

    const reject = screen.getByTestId("tasks-detail-reject-button");
    expect(reject).toHaveAttribute("aria-busy", "true");
    expect(reject).toHaveTextContent("Rejecting…");
    expect(screen.getByTestId("tasks-detail-primary-approve")).toHaveTextContent("Approve");
  });
});

describe("TaskPageOverflow", () => {
  it("Should render operation-specific progress for pending overflow actions", async () => {
    render(
      <TaskPageOverflow
        command={command}
        onCancel={vi.fn()}
        onCopyId={vi.fn()}
        onDelete={vi.fn()}
        onFanOut={vi.fn()}
        onPause={vi.fn()}
        onResume={vi.fn()}
        onStartNewRun={vi.fn()}
        pending={{ cancel: true }}
        showFanOut={false}
        taskId="task_001"
      />
    );

    fireEvent.click(screen.getByRole("button", { name: "More actions" }));
    const cancel = await screen.findByTestId("tasks-detail-cancel");
    expect(cancel).toHaveAttribute("aria-busy", "true");
    expect(cancel).toHaveTextContent("Canceling…");
  });
});
