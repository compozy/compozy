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
const { loopCatalogFixtures, loopDetailByName, loopEffectiveConfigFixture, loopRunFixtures } =
  await import("../../mocks/fixtures");
const { readLoopGraph } = await import("../../lib/loop-graph");
type LoopBindingRow = import("../../lib/loop-bindings").LoopBindingRow;

const loop = loopDetailByName.get("software-delivery")!;
const catalogEntry = loopCatalogFixtures.find(entry => entry.name === "software-delivery")!;
const recentRuns = loopRunFixtures.filter(run => run.loop_name === "software-delivery").slice(0, 5);
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
    onAddTrigger: noop,
    onAddSchedule: noop,
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

describe("LoopDetailView", () => {
  it("Should render the full definition page: header, contract, DAG, runs, and the right rail", () => {
    renderDetail();
    expect(screen.getByText("software-delivery")).toBeInTheDocument();
    expect(screen.getByTestId("loop-contract")).toBeInTheDocument();
    expect(screen.getByTestId("loop-dag")).toBeInTheDocument();
    expect(screen.getByTestId("loop-recent-runs")).toBeInTheDocument();
    expect(screen.getByTestId("loop-declared-inputs")).toBeInTheDocument();
    expect(screen.getByTestId("loop-start-bindings")).toBeInTheDocument();
    expect(screen.getByTestId("loop-limits")).toBeInTheDocument();
    expect(screen.getByTestId("loop-versions")).toBeInTheDocument();
    expect(screen.getByTestId("loop-stats")).toBeInTheDocument();
  });

  it("Should render the 8-node read-only body graph in order", () => {
    renderDetail();
    const nodes = screen.getAllByTestId("loop-dag-node").map(node => node.dataset.nodeId);
    expect(nodes).toEqual([
      "slug",
      "load_tasks",
      "implement",
      "execute_task",
      "collect",
      "review",
      "verify",
      "approve",
    ]);
  });

  it("Should render the six terminal outcomes and the 30d stats truthfully", () => {
    renderDetail();
    expect(screen.getAllByTestId("loop-terminal-chip")).toHaveLength(6);
    const stats = screen.getByTestId("loop-stats");
    expect(stats).toHaveTextContent("Success rate");
    expect(stats).toHaveTextContent("90%");
    expect(stats).toHaveTextContent("Total runs");
  });

  it("Should invoke the header actions", () => {
    const onRun = vi.fn();
    const onConfigure = vi.fn();
    const onOpenEditor = vi.fn();
    renderDetail({ onRun, onConfigure, onOpenEditor });
    fireEvent.click(screen.getByTestId("loop-run-action"));
    fireEvent.click(screen.getByTestId("loop-detail-overflow"));
    expect(screen.getByTestId("loop-edit-action")).toHaveTextContent("Edit");
    fireEvent.click(screen.getByTestId("loop-edit-action"));
    fireEvent.click(screen.getByTestId("loop-detail-overflow"));
    fireEvent.click(screen.getByTestId("loop-configure-action"));
    expect(onRun).toHaveBeenCalledTimes(1);
    expect(onConfigure).toHaveBeenCalledTimes(1);
    expect(onOpenEditor).toHaveBeenCalledTimes(1);
  });

  it("Should require the exact Loop name before deleting a workspace definition", () => {
    const onDelete = vi.fn();
    renderDetail({ onDelete });

    fireEvent.click(screen.getByTestId("loop-detail-overflow"));
    fireEvent.click(screen.getByTestId("loop-delete-action"));
    const dialog = screen.getByRole("dialog", { name: "Delete software-delivery?" });
    const confirm = within(dialog).getByRole("button", { name: "Delete loop" });
    expect(confirm).toBeDisabled();
    expect(onDelete).not.toHaveBeenCalled();

    fireEvent.change(within(dialog).getByRole("textbox", { name: "Type to confirm" }), {
      target: { value: "software-delivery" },
    });
    expect(confirm).toBeEnabled();
    fireEvent.click(confirm);
    expect(onDelete).toHaveBeenCalledTimes(1);
  });

  it("Should never offer Delete for a read-only Loop", () => {
    const readOnlyLoop = loopDetailByName.get("reviews-watch")!;
    renderDetail({}, savedConfig, readOnlyLoop);

    fireEvent.click(screen.getByTestId("loop-detail-overflow"));
    expect(screen.queryByTestId("loop-delete-action")).not.toBeInTheDocument();
    expect(screen.getByTestId("loop-edit-action")).toHaveTextContent("Fork & edit");
  });

  it("Should render the version without an unowned publish-state suffix", () => {
    renderDetail();
    expect(screen.getAllByText("v4").length).toBeGreaterThan(0);
    expect(screen.queryByText(/published/i)).not.toBeInTheDocument();
  });

  it("Should render saved per-Loop limits instead of authored contract values", () => {
    renderDetail();
    const limits = within(screen.getByTestId("loop-limits"));

    expect(limits.getByText("Iteration cap").parentElement).toHaveTextContent("3 / 100");
    expect(limits.getByText("No-progress window").parentElement).toHaveTextContent("2 / 10");
    expect(limits.queryByText("50")).not.toBeInTheDocument();
    expect(limits.getByText("escalate")).toBeInTheDocument();
  });
});
