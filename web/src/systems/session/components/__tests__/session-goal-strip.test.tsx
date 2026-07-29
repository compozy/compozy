import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { SessionGoalSnapshot } from "@/systems/session/types";

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

const { SessionGoalStrip } = await import("../goal/session-goal-strip");

function snapshot(overrides: Partial<SessionGoalSnapshot> = {}): SessionGoalSnapshot {
  return {
    bound_session_id: "session_1",
    cause: null,
    context: {
      nudge_ratio: 0.7,
      ratio: 0.41,
      reported_at: "2026-07-10T12:00:00Z",
      size: 200_000,
      state: "known",
      used: 84_120,
    },
    contract_summary: "All 16 transcript components on session.css tokens.",
    last_verdict: null,
    live: true,
    node_id: "goal",
    objective: "Migrate the session transcript",
    origin_session_id: "session_1",
    run_id: "run_1",
    run_status: "running",
    status: "active",
    turn_limit: 20,
    turns_used: 3,
    ...overrides,
  };
}

describe("SessionGoalStrip", () => {
  it.each([
    ["active", "running", "active"],
    ["paused", "paused", "paused"],
    ["blocked", "blocked", "blocked"],
    ["usage-limited", "stalled", "blocked"],
    ["complete", "done", "done"],
  ] as const)("Should map status %s to strip state %s", (status, runStatus, expected) => {
    render(<SessionGoalStrip snapshot={snapshot({ status, run_status: runStatus })} />);
    expect(screen.getByTestId("session-goal-strip")).toHaveAttribute("data-state", expected);
  });

  it("Should render the mono facts line with turn and context percentages", () => {
    render(<SessionGoalStrip snapshot={snapshot()} />);
    expect(screen.getByTestId("goal-strip-facts")).toHaveTextContent("turn 3/20 · ctx 41%");
  });

  it.each(["pending", "unknown"] as const)(
    "Should render ctx — for %s context without inventing a percentage",
    state => {
      render(
        <SessionGoalStrip
          snapshot={snapshot({
            context: {
              nudge_ratio: 0.7,
              ratio: null,
              reported_at: null,
              size: null,
              state,
              used: null,
            },
          })}
        />
      );
      expect(screen.getByTestId("goal-strip-facts")).toHaveTextContent("ctx —");

      fireEvent.click(screen.getByTestId("goal-strip-line"));
      expect(screen.getByTestId("goal-strip-body")).toHaveTextContent(
        state === "pending"
          ? "Waiting for a newer usage report."
          : "This agent has not reported context usage."
      );
    }
  );

  it("Should expand to the quiet body rows: contract, run link, context tokens, node", () => {
    render(<SessionGoalStrip snapshot={snapshot({ cause: "goal_agent_refused" })} />);
    const line = screen.getByTestId("goal-strip-line");
    expect(line).toHaveAttribute("aria-expanded", "false");

    fireEvent.click(line);

    const body = screen.getByTestId("goal-strip-body");
    expect(body).toHaveTextContent("All 16 transcript components on session.css tokens.");
    expect(body).toHaveTextContent("84,120 / 200,000 tokens · nudge at 70%");
    expect(body).toHaveTextContent("goal_agent_refused");
    const runLink = screen.getByRole("link", { name: "Open run" });
    expect(runLink).toHaveAttribute("data-params", JSON.stringify({ runId: "run_1" }));
    expect(runLink).toHaveAttribute("data-hash", "node-goal");
  });

  it("Should render the moved variant with the faint dot and active-session link", () => {
    render(
      <SessionGoalStrip
        snapshot={snapshot({ bound_session_id: "session_2", origin_session_id: "session_1" })}
      />
    );
    const strip = screen.getByTestId("session-goal-strip");
    expect(strip).toHaveAttribute("data-state", "moved");
    expect(screen.getByTestId("goal-strip-facts")).toHaveTextContent(/^moved · /);

    fireEvent.click(screen.getByTestId("goal-strip-line"));
    const sessionLink = screen.getByTestId("goal-strip-active-session");
    expect(sessionLink).toHaveAttribute("data-params", JSON.stringify({ id: "session_2" }));
  });

  it("Should stage the draft goal command into the composer without sending", () => {
    const onPrefillComposer = vi.fn();
    render(
      <SessionGoalStrip
        snapshot={snapshot()}
        composerAffordance={{ kind: "draft", expandedObjective: "Ship the calm transcript" }}
        onPrefillComposer={onPrefillComposer}
      />
    );

    fireEvent.click(screen.getByTestId("goal-strip-line"));
    fireEvent.click(screen.getByRole("button", { name: "Draft goal command" }));
    expect(onPrefillComposer).toHaveBeenCalledWith("/goal Ship the calm transcript");
  });

  it("Should render the last verdict as quiet body text with mono evidence", () => {
    render(
      <SessionGoalStrip
        snapshot={snapshot({
          last_verdict: {
            blocking_issues: [
              { id: "issue_1", note: "a" },
              { id: "issue_2", note: "b" },
            ],
            evaluated_at: "2026-07-10T12:00:00Z",
            evidence_ref: "verdict_007",
            outcome: "approved",
          },
        })}
      />
    );

    fireEvent.click(screen.getByTestId("goal-strip-line"));
    const body = screen.getByTestId("goal-strip-body");
    expect(body).toHaveTextContent("Approved · 2 blocking issues");
    expect(body).toHaveTextContent("verdict_007");
  });
});
