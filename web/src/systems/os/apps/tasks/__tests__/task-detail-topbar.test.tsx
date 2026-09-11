// Suite: task detail topbar composition
// Invariant: the window head offers exactly the affordances the latched tier's
// route matrix registers — task/run lifecycle mutations register only on the
// local surface set, so they are absent (never disabled) on remote tiers
// (BR-1), while reads (open run, copy id) stay on every tier.
// Boundary IN: the topbar composition over the task command state.
// Boundary OUT: the command state machine (task-command-state suite) and the
// presentational action/overflow components (task-page-head suite).
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { latchGatewayTierForTest } from "@/test/gateway-tier";
import { renderWithTopbar } from "@/test/render-with-topbar";

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

import type { TaskCommandState } from "@/systems/tasks";
import type { TaskDetailLocationController } from "../use-task-detail-location";
import { TaskDetailTopbar } from "../task-detail-topbar";

/** A mid-lifecycle task: every mutation affordance the head can offer. */
const MUTATION_COMMAND: TaskCommandState = {
  canFanOut: true,
  overflow: {
    cancel: true,
    delete: true,
    edit: true,
    pause: true,
    resume: true,
    startNewRun: true,
  },
  primary: { kind: "retry", runId: "run_001" },
  secondary: { edit: true, pause: true, reject: true },
};

const OPEN_RUN_COMMAND: TaskCommandState = {
  canFanOut: false,
  overflow: {
    cancel: false,
    delete: false,
    edit: false,
    pause: false,
    resume: false,
    startNewRun: false,
  },
  primary: { kind: "open_run", runId: "run_001" },
  secondary: { edit: false, pause: false, reject: false },
};

type DetailOverrides = Partial<Pick<TaskDetailLocationController, "command" | "showFanOut">>;

function detailController({ command = MUTATION_COMMAND, showFanOut = true }: DetailOverrides = {}) {
  return {
    backToTasks: vi.fn(),
    command,
    copyTaskId: vi.fn(),
    openEdit: vi.fn(),
    openRun: vi.fn(),
    page: {
      handleApproveTask: vi.fn(),
      handleCancelTask: vi.fn(),
      handleEnqueueRun: vi.fn(),
      handlePublishTask: vi.fn(),
      handleRecoverTask: vi.fn(),
      handleRejectTask: vi.fn(),
      handleResumeTask: vi.fn(),
      handleRetryRun: vi.fn(),
      isApprovePending: false,
      isCancelPending: false,
      isEnqueuePending: false,
      isPausePending: false,
      isPublishPending: false,
      isRecoverPending: false,
      isRejectPending: false,
      isResumePending: false,
      isRetryPending: false,
    },
    pauseDialog: { open: vi.fn() },
    record: { id: "task_001", status: "failed", title: "Scratch plan" },
    setDeleteOpen: vi.fn(),
    setFanOutOpen: vi.fn(),
    showFanOut,
    taskId: "task_001",
  } as unknown as TaskDetailLocationController;
}

describe("TaskDetailTopbar", () => {
  let unlatch: () => void;
  beforeEach(() => {
    unlatch = latchGatewayTierForTest("local");
  });
  afterEach(() => unlatch());

  it("Should offer the lifecycle mutations on the local tier", async () => {
    const user = userEvent.setup();
    renderWithTopbar(<TaskDetailTopbar controller={detailController()} />);

    expect(screen.getByTestId("tasks-detail-primary-retry")).toHaveTextContent("Retry");
    expect(screen.getByTestId("tasks-detail-edit-button")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-detail-reject-button")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "More actions" }));
    expect(await screen.findByTestId("tasks-detail-cancel")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-detail-start-new-run")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-detail-fan-out")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-detail-delete")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-detail-copy-id")).toBeInTheDocument();
  });

  it("Should drop the lifecycle mutations on a remote tier while keeping the reads", async () => {
    unlatch();
    unlatch = latchGatewayTierForTest("private");
    const user = userEvent.setup();
    renderWithTopbar(<TaskDetailTopbar controller={detailController()} />);

    expect(screen.queryByTestId("tasks-detail-primary-retry")).not.toBeInTheDocument();
    expect(screen.queryByTestId("tasks-detail-edit-button")).not.toBeInTheDocument();
    expect(screen.queryByTestId("tasks-detail-reject-button")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "More actions" }));
    expect(await screen.findByTestId("tasks-detail-copy-id")).toBeInTheDocument();
    expect(screen.queryByTestId("tasks-detail-cancel")).not.toBeInTheDocument();
    expect(screen.queryByTestId("tasks-detail-start-new-run")).not.toBeInTheDocument();
    expect(screen.queryByTestId("tasks-detail-fan-out")).not.toBeInTheDocument();
    expect(screen.queryByTestId("tasks-detail-delete")).not.toBeInTheDocument();
  });

  it("Should keep the open-run read affordance on a remote tier", () => {
    unlatch();
    unlatch = latchGatewayTierForTest("private");
    renderWithTopbar(
      <TaskDetailTopbar
        controller={detailController({ command: OPEN_RUN_COMMAND, showFanOut: false })}
      />
    );

    expect(screen.getByTestId("tasks-detail-primary-open_run")).toBeInTheDocument();
  });
});
