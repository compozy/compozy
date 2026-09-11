// Suite: task run topbar composition
// Invariant: the run head offers exactly the affordances the latched tier's
// route matrix registers — run mutations (retry/cancel/recover/release/
// force-fail) register only on the local surface set, so they are absent
// (never disabled) on remote tiers (BR-1), while the session and copy reads
// stay on every tier.
// Boundary IN: the topbar composition over the run page data.
// Boundary OUT: the presentational action/overflow components (task-run
// page-head suite) and the run page hook suites.
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { latchGatewayTierForTest } from "@/test/gateway-tier";
import { renderWithTopbar } from "@/test/render-with-topbar";

import type { TaskRunLocationController } from "../use-task-run-location";
import { TaskRunTopbar } from "../task-run-topbar";

function runController({
  runStatus = "failed",
  canRecover = false,
}: { runStatus?: "failed" | "running"; canRecover?: boolean } = {}) {
  return {
    backToTask: vi.fn(),
    backToTasks: vi.fn(),
    canRecover,
    copyRunId: vi.fn(),
    forceFailDialog: { open: vi.fn() },
    openSession: vi.fn(),
    page: {
      handleCancelRun: vi.fn(),
      handleForceReleaseRun: vi.fn(),
      handleRecoverRun: vi.fn(),
      handleRetryRun: vi.fn(),
      isCancelPending: false,
      isForceReleasePending: false,
      isRecoverPending: false,
      isRetryPending: false,
      run: {
        run: {
          attempt: 1,
          session_id: "sess_001",
          status: runStatus,
        },
      },
      task: { task: { max_attempts: 3, title: "Scratch plan" } },
    },
    record: { attempt: 1, status: runStatus },
    taskId: "task_001",
  } as unknown as TaskRunLocationController;
}

describe("TaskRunTopbar", () => {
  let unlatch: () => void;
  beforeEach(() => {
    unlatch = latchGatewayTierForTest("local");
  });
  afterEach(() => unlatch());

  it("Should offer the run retry on the local tier when attempts remain", () => {
    renderWithTopbar(<TaskRunTopbar controller={runController()} />);

    expect(screen.getByTestId("tasks-run-retry")).toBeInTheDocument();
  });

  it("Should drop the run retry on a remote tier while keeping the session read", () => {
    unlatch();
    unlatch = latchGatewayTierForTest("private");
    renderWithTopbar(<TaskRunTopbar controller={runController()} />);

    expect(screen.queryByTestId("tasks-run-retry")).not.toBeInTheDocument();
    expect(screen.getByTestId("tasks-run-open-session")).toBeInTheDocument();
  });

  it("Should offer the run mutations behind the overflow on the local tier", async () => {
    const user = userEvent.setup();
    renderWithTopbar(
      <TaskRunTopbar controller={runController({ canRecover: true, runStatus: "running" })} />
    );

    await user.click(screen.getByRole("button", { name: "More actions" }));
    expect(await screen.findByTestId("tasks-run-recover")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-run-cancel")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-run-copy-id")).toBeInTheDocument();
  });

  it("Should drop the run mutations behind the overflow on a remote tier", async () => {
    unlatch();
    unlatch = latchGatewayTierForTest("private");
    const user = userEvent.setup();
    renderWithTopbar(
      <TaskRunTopbar controller={runController({ canRecover: true, runStatus: "running" })} />
    );

    await user.click(screen.getByRole("button", { name: "More actions" }));
    expect(await screen.findByTestId("tasks-run-copy-id")).toBeInTheDocument();
    expect(screen.queryByTestId("tasks-run-recover")).not.toBeInTheDocument();
    expect(screen.queryByTestId("tasks-run-cancel")).not.toBeInTheDocument();
    expect(screen.queryByTestId("tasks-run-force-fail")).not.toBeInTheDocument();
  });
});
