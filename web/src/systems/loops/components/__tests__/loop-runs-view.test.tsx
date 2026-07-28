import { useState } from "react";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { vi } from "vitest";

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

const { LoopRunsView } = await import("../runs/loop-runs-view");
const { loopRunFixtures } = await import("../../mocks/fixtures");
type LoopOutcomeValue = import("../runs/loop-runs-outcome-filter").LoopOutcomeValue;

function Harness() {
  const [outcome, setOutcome] = useState<LoopOutcomeValue>("all");
  return <LoopRunsView runs={loopRunFixtures} outcome={outcome} onOutcomeChange={setOutcome} />;
}

describe("LoopRunsView", () => {
  it("Should render the four workspace KPIs with a live active count", () => {
    render(<Harness />);
    const kpis = screen.getByTestId("loop-runs-kpis");
    expect(within(kpis).getByText("Active now")).toBeInTheDocument();
    expect(within(kpis).getByText("Awaiting you")).toBeInTheDocument();
    expect(within(kpis).getByText("Done today")).toBeInTheDocument();
    expect(within(kpis).getByText("Needs a look")).toBeInTheDocument();
    // 5 live runs (running/watching/needs-approval/paused/queued).
    expect(within(screen.getByTestId("kpi-active-now")).getByText("5")).toBeInTheDocument();
  });

  it("Should render a data-driven outcome filter with per-status counts", () => {
    render(<Harness />);
    const filter = screen.getByTestId("loop-runs-outcome-filter");
    expect(within(filter).getByTestId("loop-outcome-all")).toBeInTheDocument();
    expect(within(filter).getByTestId("loop-outcome-running")).toBeInTheDocument();
    expect(within(filter).getByTestId("loop-outcome-queued")).toBeInTheDocument();
  });

  it("Should render Active and Past tables with budget mini-bars and run links", () => {
    render(<Harness />);
    expect(screen.getByTestId("loop-runs-active")).toBeInTheDocument();
    expect(screen.getByTestId("loop-runs-past")).toBeInTheDocument();
    expect(screen.getAllByTestId("loop-budget-bar").length).toBeGreaterThan(0);
    const rows = screen.getAllByTestId("loop-run-row");
    expect(rows[0]).toHaveAttribute("href");
    expect(rows[0].getAttribute("data-params")).toContain("looprun_");
  });

  it("Should label a session-origin Run with its exact origin session", () => {
    render(
      <LoopRunsView
        runs={[
          {
            ...loopRunFixtures[0],
            started_origin_kind: "session",
            started_origin_ref: "session_42",
          },
        ]}
        outcome="all"
        onOutcomeChange={() => undefined}
      />
    );
    expect(screen.getByTestId("loop-run-row")).toHaveTextContent("session · session_42");
  });

  it("Should filter the tables to the selected outcome", () => {
    render(<Harness />);
    fireEvent.click(screen.getByTestId("loop-outcome-done"));
    const rows = screen.getAllByTestId("loop-run-row");
    expect(rows.every(row => row.getAttribute("data-status") === "done")).toBe(true);
    // Done runs are terminal, so the Active table hides.
    expect(screen.queryByTestId("loop-runs-active")).not.toBeInTheDocument();
  });
});
