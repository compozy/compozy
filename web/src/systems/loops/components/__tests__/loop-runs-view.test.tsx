import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { vi } from "vitest";
import { aggregateListingScopeFixture, scopedListingScopeFixture } from "@/systems/profiles/mocks";

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

describe("LoopRunsView", () => {
  it("Should render the four workspace KPIs with a live active count", () => {
    render(
      <LoopRunsView profile={scopedListingScopeFixture} outcome="all" runs={loopRunFixtures} />
    );
    const kpis = screen.getByTestId("loop-runs-kpis");
    expect(within(kpis).getByText("Active now")).toBeInTheDocument();
    expect(within(kpis).getByText("Awaiting you")).toBeInTheDocument();
    expect(within(kpis).getByText("Done today")).toBeInTheDocument();
    expect(within(kpis).getByText("Needs a look")).toBeInTheDocument();
    // 5 live runs (running/watching/needs-approval/paused/queued).
    expect(within(screen.getByTestId("kpi-active-now")).getByText("5")).toBeInTheDocument();
  });

  it("Should render Active and Past tables with budget mini-bars and run links", () => {
    render(
      <LoopRunsView profile={scopedListingScopeFixture} outcome="all" runs={loopRunFixtures} />
    );
    expect(screen.getByTestId("loop-runs-active")).toBeInTheDocument();
    expect(screen.getByTestId("loop-runs-past")).toBeInTheDocument();
    expect(screen.getAllByTestId("loop-budget-bar").length).toBeGreaterThan(0);
    const rows = screen.getAllByTestId("loop-run-row");
    expect(rows[0]).toHaveAttribute("href");
    expect(rows[0].getAttribute("data-params")).toContain("looprun_");
    const scoredRun = rows.find(row =>
      row.getAttribute("data-params")?.includes("looprun_exhausted")
    );
    expect(scoredRun).toBeDefined();
    expect(within(scoredRun!).getByTestId("loop-run-best")).toHaveTextContent("Gen 12 · 0.88");
  });

  it("Should label a session-origin Run with its exact origin session", () => {
    render(
      <LoopRunsView
        profile={scopedListingScopeFixture}
        outcome="all"
        runs={[
          {
            ...loopRunFixtures[0],
            started_origin_kind: "session",
            started_origin_ref: "session_42",
          },
        ]}
      />
    );
    expect(screen.getByTestId("loop-run-row")).toHaveTextContent("session · session_42");
  });

  it("Should filter the tables to the selected outcome", () => {
    render(
      <LoopRunsView profile={scopedListingScopeFixture} outcome="done" runs={loopRunFixtures} />
    );
    const rows = screen.getAllByTestId("loop-run-row");
    expect(rows.every(row => row.getAttribute("data-status") === "done")).toBe(true);
    // Done runs are terminal, so the Active table hides.
    expect(screen.queryByTestId("loop-runs-active")).not.toBeInTheDocument();
  });

  it("Should show the truthful empty state when the outcome filter matches no run", () => {
    // No fixture run is `watching`, so the filter empties both tables.
    render(
      <LoopRunsView profile={scopedListingScopeFixture} outcome="watching" runs={loopRunFixtures} />
    );
    expect(screen.getByText("No matching runs")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-run-row")).not.toBeInTheDocument();
    expect(screen.queryByTestId("loop-runs-active")).not.toBeInTheDocument();
    expect(screen.queryByTestId("loop-runs-past")).not.toBeInTheDocument();
  });

  it("Should show fully paged pending request counts only on their owning runs", () => {
    const target = loopRunFixtures[0];
    render(
      <LoopRunsView
        profile={scopedListingScopeFixture}
        outcome="all"
        pendingRequestCounts={new Map([[target.id, 3]])}
        runs={loopRunFixtures}
      />
    );

    const targetRow = screen
      .getAllByTestId("loop-run-row")
      .find(row => row.getAttribute("data-params")?.includes(target.id));
    expect(within(targetRow!).getByLabelText("3 pending requests")).toBeInTheDocument();
    expect(screen.getAllByTestId("loop-run-pending-requests")).toHaveLength(1);
  });

  it("Should name the profile a scoped runs list is empty for", () => {
    render(<LoopRunsView profile={scopedListingScopeFixture} outcome="all" runs={[]} />);
    expect(screen.getByText("No runs in default yet")).toBeInTheDocument();
  });

  it("Should not name a profile when every profile's runs are on screen", () => {
    render(<LoopRunsView profile={aggregateListingScopeFixture} outcome="all" runs={[]} />);
    // `default` is the create target, never a description of what is shown.
    expect(screen.getByText("No runs in any profile yet")).toBeInTheDocument();
    expect(screen.queryByText(/in default yet/)).not.toBeInTheDocument();
  });
});
