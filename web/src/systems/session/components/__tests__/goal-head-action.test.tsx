import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { SessionGoalSnapshot } from "@/systems/session/types";
import { SessionGoalHeadAction } from "../goal/goal-head-action";

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
};

describe("SessionGoalHeadAction", () => {
  it("Should render nothing without a snapshot", () => {
    const { container } = render(<SessionGoalHeadAction snapshot={null} {...allCallbacks} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("Should show exactly one action per state — Pause while active", () => {
    render(<SessionGoalHeadAction snapshot={snapshot()} {...allCallbacks} />);
    expect(screen.getByTestId("goal-head-pause")).toBeInTheDocument();
    expect(screen.queryByTestId("goal-head-resume")).not.toBeInTheDocument();
    expect(screen.queryByTestId("goal-head-approve")).not.toBeInTheDocument();
    expect(screen.queryByTestId("goal-head-clear")).not.toBeInTheDocument();
  });

  it("Should show Resume for a paused live goal", () => {
    render(
      <SessionGoalHeadAction
        snapshot={snapshot({ status: "paused", run_status: "paused" })}
        {...allCallbacks}
      />
    );
    expect(screen.getByTestId("goal-head-resume")).toBeInTheDocument();
    expect(screen.queryByTestId("goal-head-pause")).not.toBeInTheDocument();
  });

  it("Should rank Approve above every other action when approval is needed", () => {
    render(
      <SessionGoalHeadAction
        snapshot={snapshot({ run_status: "needs-approval" })}
        {...allCallbacks}
      />
    );
    expect(screen.getByTestId("goal-head-approve")).toBeInTheDocument();
    expect(screen.queryByTestId("goal-head-pause")).not.toBeInTheDocument();
  });

  it("Should offer Clear only for a terminal snapshot", () => {
    render(
      <SessionGoalHeadAction
        snapshot={snapshot({ live: false, status: "complete", run_status: "done" })}
        {...allCallbacks}
      />
    );
    expect(screen.getByTestId("goal-head-clear")).toBeInTheDocument();
    expect(screen.queryByTestId("goal-head-pause")).not.toBeInTheDocument();

    const { container } = render(
      <SessionGoalHeadAction snapshot={snapshot({ live: false })} onClear={undefined} />
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("Should disable the action while a goal command is pending", () => {
    render(<SessionGoalHeadAction snapshot={snapshot()} pendingAction="pause" {...allCallbacks} />);
    expect(screen.getByTestId("goal-head-pause")).toBeDisabled();
  });
});
