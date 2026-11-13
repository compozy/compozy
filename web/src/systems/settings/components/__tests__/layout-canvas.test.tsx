// Suite: the layout canvas ↔ tab tree contract
// Invariant: the canvas draws one tab's split tree — one tile per leaf, one
// seam between neighbouring members — and every seam edit it emits is a tree
// that still satisfies the daemon's weight rules.
// Boundary IN: a v6 draft document and one of its tabs.
// Boundary OUT: tab and window edits (owned by the layout document edit suite),
// the inspector.
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type {
  WindowManagerLayoutDocument,
  WindowManagerLayoutNode,
  WindowManagerLayoutTab,
} from "../../lib/window-manager-layout-types";
import { LayoutCanvas } from "../layouts/layout-canvas";

function windowAt(id: string, tabId: string) {
  return {
    id,
    app: "terminal",
    route: { pathname: `/terminal/${id}`, search: "", hash: "" },
    navStack: [],
    desktopId: "desktop:main",
    tabId,
  } as unknown as WindowManagerLayoutDocument["windows"][string];
}

const NESTED_TAB: WindowManagerLayoutTab = {
  id: "tab:work",
  pinned: false,
  root: {
    id: "split:outer",
    kind: "split",
    axis: "horizontal",
    weights: [0.6, 0.4],
    children: [
      { id: "leaf:one", kind: "leaf", windowId: "app:one" },
      {
        id: "split:inner",
        kind: "split",
        axis: "vertical",
        weights: [0.5, 0.5],
        children: [
          { id: "leaf:two", kind: "leaf", windowId: "app:two" },
          { id: "leaf:three", kind: "leaf", windowId: "app:three" },
        ],
      },
    ],
  },
};

const DOCUMENT: WindowManagerLayoutDocument = {
  version: 6,
  workspaceId: "ws-1",
  desktops: [
    {
      id: "desktop:main",
      name: "Main",
      order: 0,
      activeTabId: NESTED_TAB.id,
      tabs: [NESTED_TAB],
    } as unknown as WindowManagerLayoutDocument["desktops"][number],
  ],
  windows: {
    "app:one": windowAt("app:one", NESTED_TAB.id),
    "app:two": windowAt("app:two", NESTED_TAB.id),
    "app:three": windowAt("app:three", NESTED_TAB.id),
  },
  overrides: {},
};

function renderCanvas(
  tab: WindowManagerLayoutTab = NESTED_TAB,
  { onSelect = vi.fn(), onRootChange = vi.fn(), selectedWindowId = null as string | null } = {}
) {
  render(
    <LayoutCanvas
      document={DOCUMENT}
      selectedWindowId={selectedWindowId}
      tab={tab}
      onRootChange={onRootChange}
      onSelect={onSelect}
    />
  );
  return { onSelect, onRootChange };
}

function weightsOf(node: WindowManagerLayoutNode, splitId: string): number[] {
  if (node.kind === "leaf") return [];
  if (node.id === splitId) return node.weights;
  return node.children.flatMap(child => weightsOf(child, splitId));
}

describe("LayoutCanvas", () => {
  it("Should draw one tile per leaf and one seam between neighbouring members", () => {
    renderCanvas();

    expect(screen.getByTestId("layout-canvas")).toHaveAttribute("data-tab-id", "tab:work");
    for (const id of ["app:one", "app:two", "app:three"]) {
      expect(screen.getByTestId(`layout-canvas-tile-${id}`)).toBeInTheDocument();
    }
    expect(screen.getByTestId("layout-canvas-split-split:outer")).toBeInTheDocument();
    expect(screen.getByTestId("layout-canvas-split-split:inner")).toBeInTheDocument();
    expect(screen.getAllByRole("slider")).toHaveLength(2);
    expect(screen.getByTestId("layout-canvas-seam-split:outer-0")).toBeInTheDocument();
    expect(screen.getByTestId("layout-canvas-seam-split:inner-0")).toBeInTheDocument();
  });

  it("Should draw a single-window tab as one tile with no seams", () => {
    renderCanvas({
      id: "tab:solo",
      pinned: false,
      root: { id: "leaf:solo", kind: "leaf", windowId: "app:one" },
    });

    expect(screen.getByTestId("layout-canvas-tile-app:one")).toBeInTheDocument();
    expect(screen.queryAllByRole("slider")).toHaveLength(0);
  });

  it("Should select a window when its tile is clicked and mark the selected one", async () => {
    const user = userEvent.setup();
    const { onSelect } = renderCanvas(NESTED_TAB, { selectedWindowId: "app:two" });

    expect(screen.getByTestId("layout-canvas-tile-app:two")).toHaveAttribute(
      "aria-pressed",
      "true"
    );
    expect(screen.getByTestId("layout-canvas-tile-app:one")).toHaveAttribute(
      "aria-pressed",
      "false"
    );

    await user.click(screen.getByTestId("layout-canvas-tile-app:three"));

    expect(onSelect).toHaveBeenCalledWith("app:three");
  });

  it("Should move a seam from the keyboard and keep the split's weights summing to 1", () => {
    const { onRootChange } = renderCanvas();
    const seam = screen.getByTestId("layout-canvas-seam-split:outer-0");

    expect(seam).toHaveAttribute("aria-valuenow", "60");
    fireEvent.keyDown(seam, { key: "ArrowRight" });

    expect(onRootChange).toHaveBeenCalledTimes(1);
    const root = onRootChange.mock.calls[0]?.[0] as WindowManagerLayoutNode;
    const weights = weightsOf(root, "split:outer");
    expect(weights[0]).toBeCloseTo(0.62, 6);
    expect(weights.reduce((sum, weight) => sum + weight, 0)).toBeCloseTo(1, 6);
    expect(weightsOf(root, "split:inner")).toEqual([0.5, 0.5]);
  });

  it("Should balance a seam's pair on double-click", () => {
    const { onRootChange } = renderCanvas();

    fireEvent.doubleClick(screen.getByTestId("layout-canvas-seam-split:outer-0"));

    const root = onRootChange.mock.calls[0]?.[0] as WindowManagerLayoutNode;
    expect(weightsOf(root, "split:outer")).toEqual([0.5, 0.5]);
  });
});
