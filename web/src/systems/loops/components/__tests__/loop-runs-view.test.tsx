import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
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
const { dozensActiveRuns } = await import("../stories/loop-runs-scale-fixtures");
const { loopRunFixtures } = await import("../../mocks/fixtures");
type LoopRun = (typeof loopRunFixtures)[number];

/** The roster's five columns, in the order the board locks them. */
const COLUMNS = ["Loop", "Status", "Progress", "Started", "Duration"];

function run(overrides: Partial<LoopRun> & Pick<LoopRun, "id">): LoopRun {
  return { ...loopRunFixtures[0], ...overrides };
}

const NEEDS_YOU = run({
  id: "looprun_needs",
  loop_name: "revisao-paralela",
  status: "running",
  attention: { kind: "approval", count: 1, since: "2026-07-05T11:57:00Z" },
  active_gate_id: "aplicar-correcoes",
  progress: { round: 1, steps_done: 4, steps_total: 6 },
});
const ACTIVE = run({
  id: "looprun_active",
  loop_name: "fabrica-assistida",
  status: "running",
  progress: { round: 1, steps_done: 2, steps_total: 9 },
});
const RECENT = run({
  id: "looprun_recent",
  loop_name: "revisao-paralela",
  status: "done",
  progress: { round: 2, steps_done: 6, steps_total: 6 },
});

// VC-33..36. The view-model owns the ranking and the copy (see
// `lib/__tests__/loop-runs-view.test.ts`); what these cases pin is the anatomy
// rendered over it — one section per group, five columns, the run id demoted
// under the loop name, and a degraded transport that never borrows the empty
// state's words.
describe("LoopRunsView", () => {
  it("Should render one section per group in server order, with counts and no KPI band", () => {
    render(
      <LoopRunsView
        outcome="all"
        profileScope={scopedListingScopeFixture}
        runs={[RECENT, NEEDS_YOU, ACTIVE]}
      />
    );

    const sections = screen.getAllByTestId(/^loop-runs-group-/);
    expect(sections.map(section => section.getAttribute("data-group"))).toEqual([
      "needs-you",
      "active",
      "recent",
    ]);
    expect(within(sections[0]).getByRole("heading", { name: "Needs you" })).toBeInTheDocument();
    expect(within(sections[0]).getByTestId("loop-runs-count")).toHaveTextContent("1");
    // Four counters above the list pushed the runs that need a person below the
    // fold and answered nothing the groups do not already answer.
    expect(screen.queryByTestId("loop-runs-kpis")).not.toBeInTheDocument();
  });

  // VC-35's whole subject: "scale changes the count, not the composition." The
  // scale fixture used to be thirty runs cycled off every seed, which inherited
  // the seeds' mostly-terminal mix — the dozens landed in Recent, Active held
  // eight, and the one group the contract is about stayed small.
  it("Should put the dozens in Active while Needs you stays small and first", () => {
    render(
      <LoopRunsView
        outcome="all"
        profileScope={scopedListingScopeFixture}
        runs={dozensActiveRuns}
      />
    );

    const sections = screen.getAllByTestId(/^loop-runs-group-/);
    // Composition unchanged: the same three groups, in the same order.
    expect(sections.map(section => section.getAttribute("data-group"))).toEqual([
      "needs-you",
      "active",
      "recent",
    ]);

    const counts = sections.map(section =>
      Number(within(section).getByTestId("loop-runs-count").textContent)
    );
    const [needsYou, active, recent] = counts;
    expect(active).toBeGreaterThanOrEqual(24);
    // Small enough to read at a glance, and smaller than the group it leads.
    expect(needsYou).toBeLessThan(active);
    expect(needsYou).toBeLessThanOrEqual(3);
    // The scale belongs to Active, not to the settled tail.
    expect(active).toBeGreaterThan(recent);
    // Every run is accounted for; nothing is dropped to make the shape work.
    expect(needsYou + active + recent).toBe(dozensActiveRuns.length);

    // Counters above the list are what the note rules out at scale.
    expect(screen.queryByTestId("loop-runs-kpis")).not.toBeInTheDocument();
  });

  it("Should render Loop, Status, Progress, Started and Duration, and no spend columns", () => {
    render(<LoopRunsView outcome="all" profileScope={scopedListingScopeFixture} runs={[ACTIVE]} />);

    const headers = screen.getAllByRole("columnheader").map(header => header.textContent);
    expect(headers).toEqual(COLUMNS);
    // Generations, best score and budget demote to the run page.
    expect(screen.queryByTestId("loop-run-best")).not.toBeInTheDocument();
    expect(screen.queryByTestId("loop-budget-bar")).not.toBeInTheDocument();
    const row = screen.getByTestId("loop-run-row");
    expect(within(row).getByTestId("loop-run-progress")).toHaveTextContent("step 2 of 9");
    expect(within(row).getByTestId("loop-run-duration")).toHaveTextContent("18m 00s");
  });

  it("Should lead a needs-you row with a warning chip and demote its run id under the name", () => {
    render(
      <LoopRunsView outcome="all" profileScope={scopedListingScopeFixture} runs={[NEEDS_YOU]} />
    );

    const row = screen.getByTestId("loop-run-row");
    const status = within(row).getByTestId("loop-run-status");
    expect(status).toHaveTextContent("Needs you");
    // Warning on the page; danger stays with failure and the attention bell.
    expect(status).toHaveAttribute("data-tone", "warning");
    expect(within(row).getByTestId("loop-run-summary")).toHaveTextContent(
      "an approval is waiting on “aplicar-correcoes”"
    );
    const name = within(row).getByTestId("loop-run-name");
    expect(name).toHaveTextContent("revisao-paralela");
    expect(name.getAttribute("data-params")).toContain("looprun_needs");
    expect(within(row).getByTestId("loop-run-id")).toHaveTextContent("looprun_needs");
  });

  it("Should explain how to clear a filter that matches no run", () => {
    const onEmptyAction = vi.fn();
    // No fixture run is `watching`, so the filter empties the whole roster.
    render(
      <LoopRunsView
        onEmptyAction={onEmptyAction}
        outcome="watching"
        profileScope={scopedListingScopeFixture}
        runs={loopRunFixtures}
      />
    );

    const empty = screen.getByTestId("loop-runs-empty");
    expect(within(empty).getByText("No runs match this filter")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-run-row")).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId("loop-runs-empty-action"));
    expect(onEmptyAction).toHaveBeenCalledTimes(1);
  });

  it("Should say the rows are the last read when the transport is degraded, never that it is empty", () => {
    const onRetry = vi.fn();
    render(
      <LoopRunsView
        isReconnecting
        onRetry={onRetry}
        outcome="all"
        profileScope={scopedListingScopeFixture}
        runs={[NEEDS_YOU, ACTIVE]}
      />
    );

    expect(screen.getByTestId("loop-runs-degraded")).toHaveTextContent(
      "Reconnecting to the daemon. The list below is the last read."
    );
    expect(screen.getAllByTestId("loop-run-row")).toHaveLength(2);
    expect(screen.queryByTestId("loop-runs-empty")).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId("loop-runs-degraded-retry"));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  // "The list below is the last read" without an age reads as "this is current".
  // Stating how old it is is the difference between a stale view and a lie.
  it("Should say how old the retained read is while the transport is degraded", () => {
    render(
      <LoopRunsView
        isReconnecting
        lastReadAt={new Date(Date.now() - 40_000).toISOString()}
        profileScope={scopedListingScopeFixture}
        outcome="all"
        runs={[NEEDS_YOU, ACTIVE]}
      />
    );

    expect(screen.getByTestId("loop-runs-degraded")).toHaveTextContent(
      "Reconnecting to the daemon. The list below is the last read, from 40s ago."
    );
  });

  it("Should not age a read that is not on screen", () => {
    // No rows means nothing retained to age; the sentence would be about nothing.
    render(
      <LoopRunsView
        isReconnecting
        lastReadAt={new Date(Date.now() - 40_000).toISOString()}
        outcome="all"
        profileScope={scopedListingScopeFixture}
        runs={[]}
      />
    );

    const degraded = screen.getByTestId("loop-runs-degraded");
    expect(degraded).toHaveTextContent("Reconnecting to the daemon. No runs have been read yet.");
    expect(degraded).not.toHaveTextContent("ago");
  });

  it("Should keep the shape of what is coming when a degraded read has no rows yet", () => {
    render(
      <LoopRunsView isError outcome="all" profileScope={scopedListingScopeFixture} runs={[]} />
    );

    expect(screen.getByTestId("loop-runs-degraded")).toHaveTextContent(
      "This workspace's runs could not be read."
    );
    expect(screen.getByTestId("loop-runs-skeleton")).toBeInTheDocument();
    // "No runs yet" here would blame the workspace for a transport failure.
    expect(screen.queryByTestId("loop-runs-empty")).not.toBeInTheDocument();
  });

  // A failed request and a dropped stream recover differently, so telling a
  // reader to wait for a reconnect that is not happening is its own failure.
  it("Should name a failed read separately from a dropped stream", () => {
    const { unmount } = render(
      <LoopRunsView
        isReconnecting
        outcome="all"
        profileScope={scopedListingScopeFixture}
        runs={[NEEDS_YOU]}
      />
    );
    const reconnecting = screen.getByTestId("loop-runs-degraded");
    expect(reconnecting).toHaveAttribute("data-cause", "reconnecting");
    expect(reconnecting).toHaveTextContent("Reconnecting to the daemon.");
    unmount();

    render(
      <LoopRunsView
        isError
        outcome="all"
        profileScope={scopedListingScopeFixture}
        runs={[NEEDS_YOU]}
      />
    );
    const failed = screen.getByTestId("loop-runs-degraded");
    expect(failed).toHaveAttribute("data-cause", "read-failed");
    expect(failed).toHaveTextContent("This workspace's runs could not be read.");
    expect(failed).not.toHaveTextContent("Reconnecting to the daemon.");
  });

  it("Should name the profile a scoped runs list is empty for", () => {
    render(<LoopRunsView profileScope={scopedListingScopeFixture} outcome="all" runs={[]} />);
    expect(screen.getByText("No runs in default yet")).toBeInTheDocument();
  });

  it("Should not name a profile when every profile's runs are on screen", () => {
    render(<LoopRunsView profileScope={aggregateListingScopeFixture} outcome="all" runs={[]} />);
    // `default` is the create target, never a description of what is shown.
    expect(screen.getByText("No runs in any profile yet")).toBeInTheDocument();
    expect(screen.queryByText(/in default yet/)).not.toBeInTheDocument();
  });

  it("Should label aggregate run rows with their profile owner", () => {
    render(
      <LoopRunsView
        profileScope={aggregateListingScopeFixture}
        outcome="all"
        runs={loopRunFixtures.slice(0, 2)}
      />
    );

    expect(screen.getAllByTestId("profile-owner-tag")).toHaveLength(2);
    expect(
      screen.getAllByTestId("profile-owner-tag").every(tag => tag.textContent === "default")
    ).toBe(true);
  });
});
