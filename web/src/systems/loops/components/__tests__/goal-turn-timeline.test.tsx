import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({ to, params, children, ...props }: Record<string, unknown>) => (
      <a
        href={typeof to === "string" ? to : "#"}
        data-params={JSON.stringify(params)}
        {...(props as Record<string, unknown>)}
      >
        {children as React.ReactNode}
      </a>
    ),
  };
});

const { GoalTurnTimeline } = await import("../run-page/goal-turn-timeline");
type GoalTurnTimelineItem = import("../../hooks/use-goal-turns").GoalTurnTimelineItem;

function turn(overrides: Partial<GoalTurnTimelineItem> = {}): GoalTurnTimelineItem {
  return {
    key: "goal:0:1:1:1:prompt_1",
    seq: 1,
    generation: 1,
    nodeId: "goal",
    itemIndex: 0,
    turn: 1,
    promptAttempt: 1,
    promptId: "prompt_1",
    sessionId: "session_1",
    resultStatus: "completed",
    stopReason: "end_turn",
    reasonCode: null,
    verdictOutcome: "rejected",
    blockingIssues: [{ id: "issue_1", note: "Evidence is incomplete." }],
    evidenceRef: "blob_1",
    tokensUsed: 420,
    startedAt: "2026-07-10T12:00:00Z",
    endedAt: "2026-07-10T12:01:00Z",
    ...overrides,
  };
}

describe("GoalTurnTimeline", () => {
  it("Should render total-order turns with stop reason, verdict, issues, and evidence", () => {
    render(
      <GoalTurnTimeline
        turns={[turn({ key: "turn-2", seq: 2, turn: 2, promptId: "prompt_2" }), turn()]}
      />
    );
    const timeline = screen.getByRole("region", { name: "Goal turn timeline" });
    const headings = within(timeline).getAllByText(/^Turn \d$/);
    expect(headings.map(node => node.textContent)).toEqual(["Turn 1", "Turn 2"]);
    expect(timeline).toHaveTextContent("End turn");
    expect(timeline).toHaveTextContent("Rejected");
    expect(timeline).toHaveTextContent("issue_1");
    expect(timeline).toHaveTextContent("Evidence is incomplete.");
    expect(timeline).toHaveTextContent("blob_1");
    expect(timeline).toHaveTextContent("420 tokens");
  });

  it("Should link each turn's session to the session route", () => {
    render(<GoalTurnTimeline turns={[turn()]} />);
    expect(screen.getByTestId("goal-turn-session-link-1")).toHaveAttribute(
      "data-params",
      JSON.stringify({ id: "session_1" })
    );
  });

  it("Should preserve nullable in-progress facts without inventing terminal evidence", () => {
    render(
      <GoalTurnTimeline
        live
        turns={[
          turn({
            resultStatus: null,
            stopReason: null,
            verdictOutcome: null,
            blockingIssues: [],
            evidenceRef: null,
            tokensUsed: null,
            endedAt: null,
          }),
        ]}
      />
    );
    const timeline = screen.getByRole("region", { name: "Goal turn timeline" });
    expect(timeline).toHaveAttribute("data-live", "true");
    expect(timeline).toHaveTextContent("In progress");
    expect(timeline).toHaveTextContent("Not reported");
    expect(timeline).toHaveTextContent("Not evaluated");
    expect(timeline).toHaveTextContent("No evidence reference");
    expect(timeline).toHaveTextContent("tokens not reported");
  });

  it("Should render recovery ambiguity and its durable reason code", () => {
    render(
      <GoalTurnTimeline
        turns={[
          turn({
            resultStatus: "ambiguous",
            reasonCode: "goal_recovery_ambiguous",
            verdictOutcome: null,
          }),
        ]}
      />
    );
    expect(screen.getByRole("region", { name: "Goal turn timeline" })).toHaveTextContent(
      "goal_recovery_ambiguous"
    );
  });

  it("Should render the truthful empty state before the first claimed prompt", () => {
    render(<GoalTurnTimeline turns={[]} live />);
    expect(screen.getByText(/No Goal turns yet/)).toBeInTheDocument();
  });
});
