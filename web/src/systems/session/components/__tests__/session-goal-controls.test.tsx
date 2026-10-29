import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { SessionGoalSnapshot } from "@/systems/session/types";
import { SessionGoalControls } from "../goal/session-goal-controls";

function snapshot(overrides: Partial<SessionGoalSnapshot> = {}): SessionGoalSnapshot {
  return {
    bound_session_id: "session_1",
    cause: null,
    context: {
      nudge_ratio: 0.7,
      ratio: 0.4,
      reported_at: null,
      size: null,
      state: "unknown",
      used: null,
    },
    contract_summary: "",
    last_verdict: null,
    live: true,
    node_id: "goal",
    objective: "Objective",
    origin_session_id: "session_1",
    run_id: "run_1",
    run_status: "running",
    status: "active",
    turn_limit: 20,
    turns_used: 1,
    ...overrides,
  };
}

const allCallbacks = {
  onPause: vi.fn(),
  onResume: vi.fn(),
  onApprove: vi.fn(),
  onClear: vi.fn(),
  onStop: vi.fn(),
};

describe("SessionGoalControls", () => {
  // Invariant: Goal actions reflect durable liveness; Stop is explicit, confirmed, and distinct from Clear.
  // Owner: session Goal controls; canonical suite migrated from goal-head-action.
  it("Should render nothing without a snapshot", () => {
    const { container } = render(<SessionGoalControls snapshot={null} {...allCallbacks} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("Should pair Pause with Stop while the goal is active", () => {
    render(<SessionGoalControls snapshot={snapshot()} {...allCallbacks} />);
    expect(screen.getByTestId("goal-control-pause")).toBeInTheDocument();
    expect(screen.getByTestId("goal-control-stop")).toBeInTheDocument();
    expect(screen.queryByTestId("goal-control-resume")).not.toBeInTheDocument();
    expect(screen.queryByTestId("goal-control-approve")).not.toBeInTheDocument();
    expect(screen.queryByTestId("goal-control-clear")).not.toBeInTheDocument();
  });

  it("Should show Resume for a paused live goal", () => {
    render(
      <SessionGoalControls
        snapshot={snapshot({ status: "paused", run_status: "paused" })}
        {...allCallbacks}
      />
    );
    expect(screen.getByTestId("goal-control-resume")).toBeInTheDocument();
    expect(screen.queryByTestId("goal-control-pause")).not.toBeInTheDocument();
  });

  it("Should rank Approve above every other action when approval is needed", () => {
    render(
      <SessionGoalControls
        snapshot={snapshot({ run_status: "needs-approval" })}
        {...allCallbacks}
      />
    );
    expect(screen.getByTestId("goal-control-approve")).toBeInTheDocument();
    expect(screen.queryByTestId("goal-control-pause")).not.toBeInTheDocument();
  });

  it("Should offer Clear only for a terminal snapshot", () => {
    render(
      <SessionGoalControls
        snapshot={snapshot({ live: false, status: "complete", run_status: "done" })}
        {...allCallbacks}
      />
    );
    expect(screen.getByTestId("goal-control-clear")).toBeInTheDocument();
    expect(screen.queryByTestId("goal-control-pause")).not.toBeInTheDocument();

    const { container } = render(
      <SessionGoalControls snapshot={snapshot({ live: false })} onClear={undefined} />
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("Should disable the action while a goal command is pending", () => {
    render(<SessionGoalControls snapshot={snapshot()} pendingAction="pause" {...allCallbacks} />);
    expect(screen.getByTestId("goal-control-pause")).toBeDisabled();
    expect(screen.getByTestId("goal-control-stop")).toBeDisabled();
    expect(screen.getByTestId("goal-control-pause")).toHaveTextContent("Pausing…");
  });

  it("Should confirm a permanent stop and preserve Pause as the resumable alternative", () => {
    const onStop = vi.fn();
    render(<SessionGoalControls snapshot={snapshot()} {...allCallbacks} onStop={onStop} />);
    fireEvent.click(screen.getByRole("button", { name: "Stop goal" }));
    const dialog = screen.getByRole("dialog", { name: "Stop this goal?" });
    expect(dialog).toHaveTextContent("this one cannot be resumed");
    expect(dialog).toHaveTextContent("Use Pause goal if you want to continue later");
    expect(onStop).not.toHaveBeenCalled();
    fireEvent.click(within(dialog).getByRole("button", { name: "Stop goal" }));
    expect(onStop).toHaveBeenCalledTimes(1);
  });

  it("Should keep Stop available while a persisted pause waits to become effective", () => {
    render(
      <SessionGoalControls snapshot={snapshot({ pause_requested: true })} {...allCallbacks} />
    );
    expect(screen.getByTestId("goal-control-pause")).toBeDisabled();
    expect(screen.getByTestId("goal-control-pause")).toHaveTextContent("Pausing…");
    expect(screen.getByTestId("goal-control-stop")).toBeEnabled();
    expect(screen.queryByTestId("goal-control-resume")).not.toBeInTheDocument();
  });

  it("Should dismiss a Stop confirmation when the Goal it describes was replaced", () => {
    const onStop = vi.fn();
    const { rerender } = render(
      <SessionGoalControls snapshot={snapshot()} {...allCallbacks} onStop={onStop} />
    );
    fireEvent.click(screen.getByRole("button", { name: "Stop goal" }));
    expect(screen.getByRole("dialog")).toBeVisible();
    rerender(
      <SessionGoalControls
        snapshot={snapshot({ run_id: "run_replacement" })}
        {...allCallbacks}
        onStop={onStop}
      />
    );
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(onStop).not.toHaveBeenCalled();
  });

  it("Should offer Clear without Resume after the run was canceled", () => {
    render(
      <SessionGoalControls
        snapshot={snapshot({ live: false, status: "paused", run_status: "canceled" })}
        {...allCallbacks}
      />
    );
    expect(screen.getByTestId("goal-control-clear")).toBeEnabled();
    expect(screen.queryByTestId("goal-control-resume")).not.toBeInTheDocument();
    expect(screen.queryByTestId("goal-control-stop")).not.toBeInTheDocument();
  });
});
