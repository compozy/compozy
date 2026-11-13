// Suite: OS split view
// Invariant: a tab's split tree lays out its members by the snapshot's weights with read-only
// separators, and a collapsed split keeps every member mounted while its switcher names them and
// asks the shell to show the picked one. Configured gaps lay out exactly as the projection allots
// them — outer gaps pad the content and each seam spans the inner gap — so drop zones and seams
// drawn from the projection sit on the panes. A caller composing the seam (the Settings layout
// editor) can name it and overlay its own readout, but the shell's gap and limit label still win.
// Owning layer: OsSplitView rendering a TabProjection's collapsed set. Projection math: lib/tab-projection.
// Boundary OUT: window panes (renderWindow) and focus commands (onShowMember).
import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { OsWindow } from "../../lib/os-types";
import type { ProjectionGaps } from "../../lib/tab-projection-types";
import type { CollapsedSplit, LayoutNode } from "../../lib/window-manager-types";
import { OsSplitSeam, OsSplitView } from "../os-split-view";

function win(id: string, app: OsWindow["app"]): OsWindow {
  return {
    id,
    app,
    instanceKey: null,
    route: { pathname: `/${app}`, search: {} },
    navStack: [],
    desktopId: "desktop:main",
    tabId: "tab:main",
    nodeId: `leaf:${id}`,
    pinned: false,
    inSplit: true,
    tabActive: true,
    shown: true,
  };
}

const WINDOWS: Record<string, OsWindow> = {
  "w-tasks": win("w-tasks", "tasks"),
  "w-agents": win("w-agents", "agents"),
  "w-loops": win("w-loops", "loops"),
};

const ROOT: LayoutNode = {
  id: "split:root",
  kind: "split",
  axis: "horizontal",
  weights: [0.6, 0.4],
  children: [
    { id: "leaf:w-tasks", kind: "leaf", windowId: "w-tasks" },
    {
      id: "split:right",
      kind: "split",
      axis: "vertical",
      weights: [0.5, 0.5],
      children: [
        { id: "leaf:w-agents", kind: "leaf", windowId: "w-agents" },
        { id: "leaf:w-loops", kind: "leaf", windowId: "w-loops" },
      ],
    },
  ],
};

function renderSplit(collapsed: CollapsedSplit[] = [], gaps?: ProjectionGaps) {
  const onShowMember = vi.fn();
  const { container } = render(
    <OsSplitView
      collapsed={collapsed}
      gaps={gaps}
      onShowMember={onShowMember}
      renderWindow={windowId => <div data-testid={`pane-${windowId}`} />}
      root={ROOT}
      sessionOf={() => undefined}
      windows={WINDOWS}
    />
  );
  return { container, onShowMember };
}

describe("OsSplitView", () => {
  it("Should lay out members by the split's weights with read-only separators", () => {
    const { container } = renderSplit();

    const root = container.querySelector('[data-split-id="split:root"]')!;
    const members = Array.from(root.children).filter(
      child => child.getAttribute("data-slot") === "os-split-member"
    ) as HTMLElement[];
    expect(members.map(member => member.style.flex)).toEqual(["0.6 1 0px", "0.4 1 0px"]);

    const separators = screen.getAllByRole("separator");
    expect(separators.map(seam => seam.getAttribute("aria-orientation"))).toEqual([
      "vertical",
      "horizontal",
    ]);
    for (const seam of separators) expect(seam).not.toHaveAttribute("tabindex");
    expect(screen.queryByRole("group", { name: "Windows in this split" })).toBeNull();
  });

  it("Should keep a collapsed split's members mounted and show the picked one through the shell", () => {
    const { container, onShowMember } = renderSplit([
      {
        splitId: "split:right",
        windowIds: ["w-agents", "w-loops"],
        shownWindowId: "w-agents",
        rect: { x: 0, y: 0, w: 300, h: 600 },
      },
    ]);

    expect(container.querySelector('[data-split-id="split:right"]')).toBeNull();
    expect(screen.getByTestId("pane-w-agents")).toBeInTheDocument();
    expect(screen.getByTestId("pane-w-loops")).toBeInTheDocument();

    const switcher = screen.getByRole("group", { name: "Windows in this split" });
    expect(within(switcher).getByRole("button", { name: "Agents" })).toHaveAttribute(
      "aria-pressed",
      "true"
    );
    fireEvent.click(within(switcher).getByRole("button", { name: "Loops" }));

    expect(onShowMember).toHaveBeenCalledWith("w-loops");
  });

  it("Should pad the content by the outer gaps and space members by the inner gap", () => {
    const { container } = renderSplit([], { inner: 8, top: 4, right: 6, bottom: 4, left: 6.4 });

    const view = container.querySelector<HTMLElement>('[data-slot="os-split-view"]')!;
    expect([
      view.style.paddingTop,
      view.style.paddingRight,
      view.style.paddingBottom,
      view.style.paddingLeft,
    ]).toEqual(["4px", "6px", "4px", "6px"]);
    const [vertical, horizontal] = screen.getAllByRole("separator");
    expect(vertical!.style.width).toBe("8px");
    expect(horizontal!.style.height).toBe("8px");
  });

  it("Should take no layout space for a seam without an inner gap", () => {
    renderSplit();
    const [vertical] = screen.getAllByRole("separator");
    expect(vertical!.style.width).toBe("0px");
  });

  it("Should keep the shell's gap and limit label over a composing caller's props", () => {
    const { rerender } = render(
      <OsSplitSeam
        aria-valuetext="1/2 and 1/2"
        axis="horizontal"
        data-testid="editor-seam"
        gap={8}
        style={{ width: 99 }}
        valueNow={50}
        onPointerDown={vi.fn()}
      >
        <span>readout</span>
      </OsSplitSeam>
    );

    const seam = screen.getByTestId("editor-seam");
    expect(seam).toHaveAttribute("role", "separator");
    expect(seam).toHaveAttribute("aria-valuetext", "1/2 and 1/2");
    expect(seam).toHaveAttribute("aria-valuenow", "50");
    expect(seam.style.width).toBe("8px");
    expect(within(seam).getByText("readout")).toBeInTheDocument();

    rerender(
      <OsSplitSeam
        axis="horizontal"
        data-testid="editor-seam"
        state="limit"
        onPointerDown={vi.fn()}
      >
        <span>readout</span>
      </OsSplitSeam>
    );

    expect(within(screen.getByTestId("editor-seam")).getByText("Minimum size")).toBeInTheDocument();
    expect(screen.queryByText("readout")).not.toBeInTheDocument();
  });
});
