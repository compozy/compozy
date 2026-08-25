import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AgentCallTree, CALL_TREE_VIRTUALIZATION_THRESHOLD } from "../agent-call-tree";
import { buildCallTree } from "../../lib/agent-comms-tree";
import { activityTreeCallsFixture, buildLargeTreeFixture, completedCallFixture } from "../../mocks";

function renderTree(
  calls = activityTreeCallsFixture,
  overrides: Partial<Parameters<typeof AgentCallTree>[0]> = {}
) {
  const onSelectCall = vi.fn();
  const utils = render(
    <AgentCallTree
      data-testid="tree"
      tree={buildCallTree(calls)}
      onSelectCall={onSelectCall}
      {...overrides}
    />
  );
  return { ...utils, onSelectCall };
}

describe("AgentCallTree", () => {
  it("Should render one group per delegation tree and every call under it", () => {
    renderTree();

    const groups = screen.getAllByTestId("agent-call-tree-group");
    const rows = screen.getAllByTestId("agent-call-tree-row");
    expect(groups).toHaveLength(2);
    expect(rows).toHaveLength(activityTreeCallsFixture.length);
  });

  it("Should open the call record when a row is chosen", async () => {
    const user = userEvent.setup();
    const { onSelectCall } = renderTree();

    const rows = screen.getAllByTestId("agent-call-tree-row");
    await user.click(rows[0]!);

    expect(onSelectCall).toHaveBeenCalledTimes(1);
    expect(onSelectCall.mock.calls[0]![0]).toMatch(/^call_/);
  });

  it("Should expose folder rows to assistive tech with their expansion state", () => {
    renderTree();

    const groups = screen.getAllByTestId("agent-call-tree-group");
    // Groups open expanded: the operator came to see live work.
    expect(groups[0]).toHaveAttribute("aria-expanded", "true");
  });

  it("Should carry the tree's worst state on the group header", () => {
    renderTree();

    const groups = screen.getAllByTestId("agent-call-tree-group");
    // The marketing tree holds an invalid-result call.
    const escalated = groups.find(group => within(group).queryByText("invalid-result") !== null);
    expect(escalated).toBeDefined();
  });

  it("Should render daemon counts on the header and omit the clauses that are zero", () => {
    renderTree(activityTreeCallsFixture, {
      countsByRoot: new Map([
        [completedCallFixture.root_session_id, { total: 3, running: 2, needsYou: 0 }],
      ]),
    });

    expect(screen.getByText("3 calls · 2 running")).toBeInTheDocument();
    expect(screen.queryByText(/0 needs a look/)).not.toBeInTheDocument();
  });

  it("Should show no summary at all while the counts are still unknown", () => {
    renderTree();
    expect(screen.queryByText(/calls ·/)).not.toBeInTheDocument();
  });

  it("Should offer the drain control only when the operator can drain", async () => {
    const user = userEvent.setup();
    const onStopSubtree = vi.fn();
    const { rerender } = renderTree(activityTreeCallsFixture, { onStopSubtree });

    const stopButtons = screen.getAllByRole("button", { name: /stop subtree/i });
    await user.click(stopButtons[0]!);
    expect(onStopSubtree).toHaveBeenCalledTimes(1);

    rerender(
      <AgentCallTree tree={buildCallTree(activityTreeCallsFixture)} onSelectCall={vi.fn()} />
    );
    // Absent, not disabled: the affordance goes away with the operation.
    expect(screen.queryByRole("button", { name: /stop subtree/i })).not.toBeInTheDocument();
  });
});

/**
 * UT-129 — windowing, asserted against the component operators actually use.
 *
 * The old version of this case asserted 150 mounted rows, which is the exact
 * opposite of virtualization: a tree that mounts every row is the bug. What has
 * to hold instead is that a large tree mounts few rows, still scrolls its full
 * height, and keeps its keyboard contract while doing so.
 */
describe("AgentCallTree — windowing", () => {
  const ROW_HEIGHT = 34;
  const VIEWPORT_HEIGHT = 408;

  // jsdom reports every box as zero-sized, so the virtualizer would see a
  // viewport with no room and window down to nothing. `offsetHeight` on the
  // scroll element is the only measurement it reads — row heights come from
  // `estimateSize`, since these rows are not individually measured.
  const originalOffsetHeight = Object.getOwnPropertyDescriptor(
    HTMLElement.prototype,
    "offsetHeight"
  );

  function stubLayout() {
    Object.defineProperty(HTMLElement.prototype, "offsetHeight", {
      configurable: true,
      get(this: HTMLElement) {
        return this.dataset.testid === "tree-viewport" ? VIEWPORT_HEIGHT : ROW_HEIGHT;
      },
    });
  }

  afterEach(() => {
    if (originalOffsetHeight) {
      Object.defineProperty(HTMLElement.prototype, "offsetHeight", originalOffsetHeight);
    }
    vi.restoreAllMocks();
  });

  it("Should stay fully mounted below the threshold, where windowing costs more than it saves", () => {
    stubLayout();
    renderTree(buildLargeTreeFixture(CALL_TREE_VIRTUALIZATION_THRESHOLD - 2));

    expect(screen.getByTestId("tree")).not.toHaveAttribute("data-virtualized");
    expect(screen.queryByTestId("tree-viewport")).not.toBeInTheDocument();
  });

  it("Should window a 150-call tree, mounting a fraction of its rows", () => {
    stubLayout();
    renderTree(buildLargeTreeFixture(150));

    expect(screen.getByTestId("tree")).toHaveAttribute("data-virtualized", "true");
    const mounted = screen.getAllByTestId("agent-call-tree-row").length;
    expect(mounted).toBeGreaterThan(0);
    // A viewport this tall holds ~12 rows; overscan pads it. Anything near 150
    // would mean the window is not a window.
    expect(mounted).toBeLessThan(60);
  });

  it("Should keep the scroll height of every row it did not mount", () => {
    stubLayout();
    const { container } = renderTree(buildLargeTreeFixture(150));

    // Spacers stand in for the unmounted rows, so the scrollbar still measures
    // the whole tree — the operator can reach row 150 by dragging it.
    const spacers = Array.from(container.querySelectorAll<HTMLElement>("[aria-hidden='true']"));
    const spacerHeight = spacers.reduce(
      (total, node) => total + Number.parseFloat(node.style.height || "0"),
      0
    );
    const mounted = screen.getAllByTestId("agent-call-tree-row").length;
    const groups = screen.queryAllByTestId("agent-call-tree-group").length;
    expect(spacerHeight + (mounted + groups) * ROW_HEIGHT).toBeGreaterThanOrEqual(150 * ROW_HEIGHT);
  });

  it("Should scroll an unmounted row into view when the keyboard reaches it", async () => {
    stubLayout();
    const user = userEvent.setup();
    renderTree(buildLargeTreeFixture(150));

    const viewport = screen.getByTestId("tree-viewport");
    const scrollTo = vi.spyOn(viewport, "scrollTo");
    const rows = screen.getAllByTestId("agent-call-tree-row");
    rows[0]!.focus();

    // Walk past the mounted window. Without the `scrollToItem` seam, ↓ would
    // step into rows that do not exist in the DOM and focus would stall.
    for (let step = 0; step < 40; step += 1) {
      await user.keyboard("{ArrowDown}");
    }

    expect(scrollTo).toHaveBeenCalled();
  });

  it("Should open the record on Enter, through the tree's own primary action", async () => {
    stubLayout();
    const user = userEvent.setup();
    const { onSelectCall } = renderTree(buildLargeTreeFixture(150));

    screen.getAllByTestId("agent-call-tree-row")[0]!.focus();
    await user.keyboard("{Enter}");

    expect(onSelectCall).toHaveBeenCalledTimes(1);
    expect(onSelectCall.mock.calls[0]![0]).toMatch(/^call_/);
  });
});

/**
 * Invariant: a row shows what became of its child only when the catalog can
 * answer. Owning layer: `AgentCallTree` / `AgentCallTreeRow`. Canonical suite:
 * this file.
 */
describe("AgentCallTree — child state", () => {
  it("Should show a child-state pill for the children the catalog resolved", () => {
    const child = completedCallFixture.child_session_id!;
    renderTree(activityTreeCallsFixture, {
      childStates: new Map([[child, "parked" as const]]),
    });

    const pill = screen.getAllByTestId("agent-call-tree-child-state")[0]!;
    expect(pill).toHaveAttribute("data-child-state", "parked");
  });

  it("Should show no pill at all when the catalog has not answered", () => {
    // Absence of a pill is the honest rendering of "not known yet" — an
    // unresolved child must never borrow another child's state.
    renderTree(activityTreeCallsFixture, { childStates: new Map() });
    expect(screen.queryAllByTestId("agent-call-tree-child-state")).toHaveLength(0);
  });
});
