import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({ to, params, hash, children, ...props }: Record<string, unknown>) => (
      <a
        href={typeof to === "string" ? to : "#"}
        data-params={JSON.stringify(params)}
        data-hash={hash}
        {...(props as Record<string, unknown>)}
      >
        {children as React.ReactNode}
      </a>
    ),
  };
});

const { GoalStatusChip } = await import("../goal/goal-status-chip");
type SessionGoalSnapshot = import("../goal/goal-status-chip").SessionGoalSnapshot;

function snapshot(overrides: Partial<SessionGoalSnapshot> = {}): SessionGoalSnapshot {
  return {
    bound_session_id: "session_1",
    cause: null,
    context: {
      nudge_ratio: 0.8,
      ratio: 0.5,
      reported_at: "2026-07-10T12:00:00Z",
      size: 200_000,
      state: "known",
      used: 100_000,
    },
    contract_summary: "Ship after verification.",
    last_verdict: null,
    live: true,
    node_id: "goal",
    objective: "Ship the Goal surface.",
    origin_session_id: "session_1",
    run_id: "run_1",
    run_status: "running",
    status: "active",
    turn_limit: 20,
    turns_used: 7,
    ...overrides,
  };
}

describe("GoalStatusChip", () => {
  it.each([
    ["active", "running", true],
    ["paused", "paused", true],
    ["blocked", "blocked", false],
    ["usage-limited", "needs-approval", true],
    ["budget-limited", "needs-approval", true],
    ["complete", "done", false],
  ] as const)("Should render the canonical %s status", (status, runStatus, live) => {
    render(<GoalStatusChip snapshot={snapshot({ status, run_status: runStatus, live })} />);
    const chip = screen.getByRole("region", { name: "Goal status" });
    expect(chip).toHaveAttribute("data-goal-status", status);
    expect(chip).toHaveAttribute("data-run-status", runStatus);
    expect(chip).toHaveAttribute("data-live", String(live));
    expect(chip).toHaveTextContent(status);
  });

  it.each([
    ["known", 0.5, "Context used 50%", true],
    ["known", 0.85, "Context used 85%", true],
    ["unknown", null, "", false],
    ["pending", null, "", false],
  ] as const)(
    "Should render %s context without inventing a percentage",
    (state, ratio, progressName, hasProgress) => {
      render(
        <GoalStatusChip
          snapshot={snapshot({
            context: {
              nudge_ratio: 0.8,
              ratio,
              reported_at: state === "unknown" ? null : "2026-07-10T12:00:00Z",
              size: ratio === null ? null : 200_000,
              state,
              used: ratio === null ? null : Math.round(ratio * 200_000),
            },
          })}
        />
      );

      const context = document.querySelector(`[data-context-state="${state}"]`);
      expect(context).not.toBeNull();
      if (hasProgress) {
        expect(within(context as HTMLElement).getByRole("progressbar")).toHaveAccessibleName(
          progressName
        );
      } else {
        expect(within(context as HTMLElement).queryByRole("progressbar")).not.toBeInTheDocument();
        expect(within(context as HTMLElement).getByText(state)).toBeInTheDocument();
        expect(context).not.toHaveTextContent(`${state}%`);
      }
    }
  );

  it("Should expose only live controls backed by the current snapshot", () => {
    const onPause = vi.fn();
    const onResume = vi.fn();
    const onApprove = vi.fn();
    const onClear = vi.fn();
    const { rerender } = render(
      <GoalStatusChip snapshot={snapshot()} onPause={onPause} onClear={onClear} />
    );
    fireEvent.click(screen.getByRole("button", { name: "Pause goal" }));
    fireEvent.click(screen.getByRole("button", { name: "Clear goal" }));
    expect(onPause).toHaveBeenCalledTimes(1);
    expect(onClear).toHaveBeenCalledTimes(1);

    rerender(
      <GoalStatusChip
        snapshot={snapshot({ status: "paused", run_status: "paused" })}
        onResume={onResume}
      />
    );
    expect(screen.queryByRole("button", { name: "Pause goal" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Resume goal" }));
    expect(onResume).toHaveBeenCalledTimes(1);

    rerender(
      <GoalStatusChip
        snapshot={snapshot({ status: "usage-limited", run_status: "needs-approval" })}
        onApprove={onApprove}
      />
    );
    fireEvent.click(screen.getByRole("button", { name: "Approve goal" }));
    expect(onApprove).toHaveBeenCalledTimes(1);
  });

  it("Should prefill an expected-run replacement without executing it", () => {
    const onPrefillComposer = vi.fn();
    render(
      <GoalStatusChip
        snapshot={snapshot({ cause: "goal_replace_required" })}
        composerAffordance={{ kind: "replace", expectedRunId: "run_1", objective: "Ship it" }}
        onPrefillComposer={onPrefillComposer}
      />
    );
    fireEvent.click(screen.getByRole("button", { name: "Prefill replacement" }));
    expect(onPrefillComposer).toHaveBeenCalledWith("/goal replace run_1 Ship it");
  });

  it("Should link to the Run node and expose a moved active session", () => {
    render(
      <GoalStatusChip
        snapshot={snapshot({
          bound_session_id: "session_2",
          origin_session_id: "session_1",
        })}
      />
    );
    expect(screen.getByRole("link", { name: "Open run" })).toHaveAttribute(
      "data-params",
      JSON.stringify({ runId: "run_1" })
    );
    expect(screen.getByRole("link", { name: "Open run" })).toHaveAttribute(
      "data-hash",
      "node-goal"
    );
    expect(screen.getByRole("link", { name: "Active session" })).toHaveAttribute(
      "data-params",
      JSON.stringify({ id: "session_2" })
    );
  });
});
