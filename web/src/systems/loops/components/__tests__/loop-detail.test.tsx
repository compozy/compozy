import { fireEvent, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { renderWithTopbar } from "@/test/render-with-topbar";

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
    useNavigate: () => vi.fn(),
  };
});

const { LoopDetailView } = await import("../detail/loop-detail");
const { LoopDetailSkeleton } = await import("../detail/loop-detail-skeleton");
const { loopCatalogFixtures, loopDetailByName, loopEffectiveConfigFixture, loopRunFixtures } =
  await import("../../mocks/fixtures");
const { readLoopGraph } = await import("../../lib/loop-graph");
type LoopBindingRow = import("../../lib/loop-bindings").LoopBindingRow;

const loop = loopDetailByName.get("implement-tasks")!;
const workspaceLoop = { ...loop, source: "workspace" as const };
const catalogEntry = loopCatalogFixtures.find(entry => entry.name === "implement-tasks")!;
const recentRuns = loopRunFixtures.filter(run => run.loop_name === "implement-tasks").slice(0, 5);
const bindings: LoopBindingRow[] = [
  { id: "job", name: "nightly", kind: "schedule", enabled: false, meta: "Cron 0 3 * * *" },
];
const savedConfig = {
  iteration_cap: 3,
  budget_on_exceeded: "escalate" as const,
  no_progress_window: 2,
  fan_out_width: 4,
  gate_max_revisions: 2,
};

function renderDetail(
  handlers: Partial<Record<string, () => void>> = {},
  config = savedConfig,
  renderedLoop = loop
) {
  const noop = vi.fn();
  const merged = {
    onRun: noop,
    onConfigure: noop,
    onOpenEditor: noop,
    onDelete: noop,
    onDeleteReset: noop,
    onAutomate: noop,
    ...handlers,
  };
  renderWithTopbar(
    <LoopDetailView
      loop={renderedLoop}
      effectiveConfig={{ ...loopEffectiveConfigFixture, ...config }}
      graph={readLoopGraph(renderedLoop.definition)}
      recentRuns={recentRuns}
      bindings={bindings}
      bindingsLoading={false}
      successRate={catalogEntry.success_rate_30d}
      aggregate={catalogEntry.aggregate_30d}
      deleteError={null}
      deletePending={false}
      {...merged}
    />
  );
  return merged;
}

/** Rail cards fold by default; open one the way an operator would. */
function openRail(name: RegExp) {
  fireEvent.click(screen.getByRole("button", { name }));
}

describe("LoopDetailView", () => {
  it("Should render the full definition page: header, contract, DAG, runs, and the right rail", () => {
    renderDetail();
    expect(screen.getByTestId("loop-detail-header")).toHaveTextContent("implement-tasks");
    expect(screen.getByTestId("loop-contract")).toBeInTheDocument();
    expect(screen.getByTestId("loop-dag")).toBeInTheDocument();
    expect(screen.getByTestId("loop-recent-runs")).toBeInTheDocument();
    expect(screen.getAllByTestId("loop-recent-run-duration")).toHaveLength(recentRuns.length);
    // Run ids, rounds, and best scores live on the run page, not in this compact list.
    expect(screen.queryByTestId("loop-recent-run-best")).not.toBeInTheDocument();
    expect(screen.queryByText(recentRuns[0].id)).not.toBeInTheDocument();
    expect(screen.getByTestId("loop-declared-inputs")).toBeInTheDocument();
    expect(screen.getByTestId("loop-start-bindings")).toBeInTheDocument();
    expect(screen.getByTestId("loop-limits")).toBeInTheDocument();
    // Version rides the lede tag and the 30-day stats ride the lede meta, not rail cards.
    expect(screen.queryByTestId("loop-versions")).not.toBeInTheDocument();
    expect(screen.queryByTestId("loop-stats")).not.toBeInTheDocument();
  });

  it("Should render both implementation modes in order", () => {
    renderDetail();
    const nodes = screen.getAllByTestId("loop-dag-node").map(node => node.dataset.nodeId);
    expect(nodes).toEqual([
      "slug_input",
      "load_tasks",
      "implement",
      "select_mode",
      "select_category",
      "stage_orchestrated",
      "execute_backend",
      "execute_frontend",
      "execute_default",
      "collect",
      "select_delivery",
      "per_task_done",
      "orchestrate",
    ]);
  });

  it("Should fold the possible endings in plain words and state the 30d stats in the lede", () => {
    renderDetail();
    expect(screen.queryByTestId("loop-terminal-endings")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Details" }));
    expect(screen.getByTestId("loop-terminal-endings")).toHaveTextContent(
      "Done · Nothing to do · Blocked · Failed · Exhausted · Stalled"
    );
    expect(screen.queryByTestId("loop-terminal-chip")).not.toBeInTheDocument();
    const header = screen.getByTestId("loop-detail-header");
    expect(header).toHaveTextContent("90% success · 42 runs this month");
    expect(header).not.toHaveTextContent("compozy.loop/v1");
    expect(header).toHaveTextContent("13 steps");
  });

  it("Should invoke the header actions", () => {
    const onRun = vi.fn();
    const onConfigure = vi.fn();
    const onOpenEditor = vi.fn();
    renderDetail({ onRun, onConfigure, onOpenEditor });
    fireEvent.click(screen.getByTestId("loop-run-action"));
    fireEvent.click(screen.getByTestId("loop-detail-overflow"));
    expect(screen.getByTestId("loop-edit-action")).toHaveTextContent("Copy and edit");
    fireEvent.click(screen.getByTestId("loop-edit-action"));
    fireEvent.click(screen.getByTestId("loop-detail-overflow"));
    fireEvent.click(screen.getByTestId("loop-configure-action"));
    expect(onRun).toHaveBeenCalledTimes(1);
    expect(onConfigure).toHaveBeenCalledTimes(1);
    expect(onOpenEditor).toHaveBeenCalledTimes(1);
  });

  it("Should offer one Automate menu with the starts the Loop allows [UT-114]", async () => {
    const onAutomate = vi.fn();
    const automatable = {
      ...loop,
      definition: {
        ...loop.definition,
        start: [{ kind: "manual" }, { kind: "schedule" }, { kind: "webhook" }],
      },
    };
    renderDetail({ onAutomate }, savedConfig, automatable);

    const actions = screen.getByTestId("loop-detail-actions");
    expect(actions).not.toHaveTextContent("Add trigger");
    expect(actions).not.toHaveTextContent("Add schedule");
    fireEvent.click(within(actions).getByTestId("loop-automate-action"));
    const menu = await screen.findByTestId("loop-automate-menu");
    expect(
      within(menu)
        .getAllByRole("menuitem")
        .map(item => item.textContent)
    ).toEqual(["On a schedule", "When something happens"]);
    fireEvent.click(within(menu).getByText("When something happens"));
    expect(onAutomate).toHaveBeenCalledExactlyOnceWith("event");
  });

  it("Should not offer Automate when the Loop only starts by hand", () => {
    const manualOnly = {
      ...loop,
      definition: { ...loop.definition, start: [{ kind: "manual" }, { kind: "cli" }] },
    };
    renderDetail({}, savedConfig, manualOnly);
    expect(screen.queryByTestId("loop-automate-action")).not.toBeInTheDocument();
  });

  it("Should require the exact Loop name before deleting a workspace definition", () => {
    const onDelete = vi.fn();
    renderDetail({ onDelete }, savedConfig, workspaceLoop);

    fireEvent.click(screen.getByTestId("loop-detail-overflow"));
    fireEvent.click(screen.getByTestId("loop-delete-action"));
    const dialog = screen.getByRole("dialog", { name: "Delete implement-tasks?" });
    const confirm = within(dialog).getByRole("button", { name: "Delete loop" });
    expect(confirm).toBeDisabled();
    expect(onDelete).not.toHaveBeenCalled();

    fireEvent.change(within(dialog).getByRole("textbox", { name: "Type to confirm" }), {
      target: { value: "implement-tasks" },
    });
    expect(confirm).toBeEnabled();
    fireEvent.click(confirm);
    expect(onDelete).toHaveBeenCalledTimes(1);
  });

  it("Should never offer Delete for a read-only Loop", () => {
    const readOnlyLoop = loopDetailByName.get("review-and-fix")!;
    renderDetail({}, savedConfig, readOnlyLoop);

    fireEvent.click(screen.getByTestId("loop-detail-overflow"));
    expect(screen.queryByTestId("loop-delete-action")).not.toBeInTheDocument();
    expect(screen.getByTestId("loop-edit-action")).toHaveTextContent("Copy and edit");
    expect(screen.getByTestId("loop-open-builder")).toHaveTextContent("Copy and edit");
  });

  it("Should render the version without an unowned publish-state suffix", () => {
    renderDetail();
    expect(screen.getAllByText("v0").length).toBeGreaterThan(0);
    expect(screen.queryByText(/published/i)).not.toBeInTheDocument();
  });

  it("Should render saved per-Loop limits instead of authored contract values", () => {
    renderDetail();
    openRail(/Limits/);
    const limits = within(screen.getByTestId("loop-limits"));

    expect(limits.getByText("Max rounds").parentElement).toHaveTextContent("3");
    expect(limits.getByText("3").closest("[title]")).toHaveAttribute("title", "Up to 100");
    expect(limits.getByText("Rounds without progress").parentElement).toHaveTextContent("2");
    expect(limits.queryByText("50")).not.toBeInTheDocument();
    expect(limits.getByText("Pause and ask me")).toBeInTheDocument();
    expect(limits.queryByText("escalate")).not.toBeInTheDocument();
  });
});

// A read still in flight is not an answer. The page holds the place of what is coming, in one
// named status region per area, and never says "has not run yet" or "Manual only" about a read
// that has not returned.
describe("LoopDetailView pending reads", () => {
  function renderPending(pending: { recentRunsLoading?: boolean; bindingsLoading?: boolean }) {
    const noop = vi.fn();
    renderWithTopbar(
      <LoopDetailView
        loop={loop}
        effectiveConfig={{ ...loopEffectiveConfigFixture, ...savedConfig }}
        graph={readLoopGraph(loop.definition)}
        recentRuns={[]}
        recentRunsLoading={pending.recentRunsLoading}
        bindings={[]}
        bindingsLoading={pending.bindingsLoading ?? false}
        successRate={catalogEntry.success_rate_30d}
        aggregate={catalogEntry.aggregate_30d}
        deleteError={null}
        deletePending={false}
        onRun={noop}
        onConfigure={noop}
        onOpenEditor={noop}
        onDelete={noop}
        onDeleteReset={noop}
        onAutomate={noop}
      />
    );
  }

  it("Should not call recent runs that are still being read a Loop that has not run", () => {
    renderPending({ recentRunsLoading: true });

    const region = screen.getByText("Loading recent runs").closest('[role="status"]');
    expect(region).toHaveAttribute("aria-busy", "true");
    expect(screen.getByTestId("loop-recent-runs-loading")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-recent-runs-empty")).not.toBeInTheDocument();
    expect(screen.queryByTestId("loop-recent-runs")).not.toBeInTheDocument();
  });

  it("Should not say a Loop is manual-only while its automations are still being read", () => {
    renderPending({ bindingsLoading: true });

    const trigger = screen.getByRole("button", { name: /Automations/ });
    expect(trigger).not.toHaveTextContent("Manual only");
    openRail(/Automations/);
    expect(screen.getByText("Loading automations")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-bindings-empty")).not.toBeInTheDocument();
  });
});

describe("LoopDetailSkeleton", () => {
  // Invariant: before the definition and settings arrive the page already names the Loop (from the
  // route), prints its static headings and publishes the window head's way back, with everything
  // the reads supply held as placeholders in one named status region.
  it("Should name the Loop and hold the page's frame while its reads are pending", () => {
    renderWithTopbar(<LoopDetailSkeleton name="implement-tasks" />);

    const page = screen.getByTestId("loop-detail-loading");
    expect(page).toHaveAttribute("role", "status");
    expect(page).toHaveAttribute("aria-busy", "true");
    expect(within(page).getByText("Loading Loop")).toBeInTheDocument();
    expect(within(page).getByRole("heading", { level: 1 })).toHaveTextContent("implement-tasks");
    for (const heading of [
      "Goal and finish line",
      "Steps",
      "Recent runs",
      "Inputs",
      "Automations",
      "Limits",
    ]) {
      expect(within(page).getByText(heading)).toBeInTheDocument();
    }
    // Nothing is claimed about a Loop that has not been read.
    expect(within(page).queryByText(/has not run yet/)).not.toBeInTheDocument();
    expect(within(page).queryByText("Manual only")).not.toBeInTheDocument();

    const trail = screen.getByRole("navigation", { name: "Window path" });
    expect(within(trail).getByRole("button", { name: "Loops" })).toBeInTheDocument();
    expect(within(trail).getByText("implement-tasks")).toBeInTheDocument();
  });
});
