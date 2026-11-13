// Suite: top-bar desktop indicators
// Invariant: visible controls preserve ordered desktop navigation, disclose every hidden range, and
// mark off-screen desktops that need you; a click switches; the desktop menu opens from the pointer
// and the menu keys with Rename, Move left, Move right, Close desktop and New desktop, an item
// absent where it cannot apply (US-015.AC-3, EC-1); during a tab drag every desktop is a labelled
// drop target, named for a window during a window-head drag, and resting on an overflow control
// opens the overview once (US-016.AC-1, EC-2).
// Boundary IN: DesktopIndicators control projection, callbacks, accessibility, keyboard
// navigation, the desktop menu, drop-target markup, the overflow drag hover and the desktop
// needs-you projection.
// Boundary OUT: overview rendering, the drop's daemon command, shell positioning measurements and
// browser layout.
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { desktopIdsNeedingYou } from "../../lib/desktop-pager-attention";
import type { OsAttentionRow } from "../../lib/attention-model";
import type { DesktopIndicatorItem } from "../../lib/desktop-indicator-controls";
import type { OsWindow } from "../../lib/os-types";
import { DESKTOP_DROP_TARGET_ATTRIBUTE } from "../../hooks/use-os-tab-drag";
import { useDesktopOverflowDragHover } from "../../hooks/use-desktop-overflow-drag-hover";
import { osTabDragStore, type TabDragSession } from "../../stores/os-tab-drag-store";
import { DesktopIndicators, type DesktopIndicatorsProps } from "../desktop-indicators";

const DESKTOPS: DesktopIndicatorItem[] = [
  { id: "control", name: "Control" },
  { id: "build", name: "Build" },
  { id: "review", name: "Review" },
  { id: "research", name: "Research" },
];

const MANY_DESKTOPS: DesktopIndicatorItem[] = [
  ...DESKTOPS,
  { id: "qa", name: "QA" },
  { id: "docs", name: "Docs" },
  { id: "release", name: "Release" },
  { id: "incidents", name: "Incidents" },
  { id: "archive", name: "Archive" },
];

function renderIndicators(props: Partial<DesktopIndicatorsProps> = {}) {
  const callbacks = {
    onSelectDesktop: vi.fn(),
    onOpenOverview: vi.fn(),
    onRenameDesktop: vi.fn(),
    onReorderDesktop: vi.fn(),
    onCloseDesktop: vi.fn(),
    onCreateDesktop: vi.fn(),
  };
  render(
    <DesktopIndicators desktops={DESKTOPS} activeDesktopId="build" {...callbacks} {...props} />
  );
  return callbacks;
}

async function openMenu(name: string) {
  const user = userEvent.setup();
  await user.pointer({ keys: "[MouseRight]", target: screen.getByRole("button", { name }) });
  return user;
}

function menuItems(): string[] {
  return screen.getAllByRole("menuitem").map(item => item.textContent ?? "");
}

describe("DesktopIndicators", () => {
  it("Should render all desktops through seven and identify the active position", () => {
    renderIndicators({ desktops: MANY_DESKTOPS.slice(0, 7) });

    const active = screen.getByRole("button", { name: "Desktop 2 of 7: Build" });
    expect(active).toHaveAttribute("aria-current", "page");
    expect(active).toHaveAttribute("tabindex", "0");
    expect(screen.getAllByRole("button")).toHaveLength(7);
    expect(screen.queryByRole("button", { name: /earlier desktops/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /later desktops/ })).not.toBeInTheDocument();
  });

  it("Should expose adaptive earlier and later ranges to the overview callback", async () => {
    const user = userEvent.setup();
    const { onOpenOverview } = renderIndicators({
      desktops: MANY_DESKTOPS,
      activeDesktopId: "qa",
    });

    await user.click(screen.getByRole("button", { name: "Show 2 earlier desktops" }));
    expect(onOpenOverview).toHaveBeenLastCalledWith({
      direction: "earlier",
      hiddenDesktopIds: ["control", "build"],
      anchorDesktopId: "qa",
    });

    await user.click(screen.getByRole("button", { name: "Show 2 later desktops" }));
    expect(onOpenOverview).toHaveBeenLastCalledWith({
      direction: "later",
      hiddenDesktopIds: ["incidents", "archive"],
      anchorDesktopId: "qa",
    });
    expect(screen.getByRole("button", { name: "Desktop 5 of 9: QA" })).toHaveAttribute(
      "aria-current",
      "page"
    );
  });

  it("Should start overflow at eight desktops", () => {
    renderIndicators({ desktops: MANY_DESKTOPS.slice(0, 8), activeDesktopId: "qa" });

    expect(screen.getAllByRole("button")).toHaveLength(7);
    expect(screen.getByRole("button", { name: "Show 2 earlier desktops" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Show 1 later desktop" })).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Desktop 1 of 8: Control" })
    ).not.toBeInTheDocument();
  });

  it("Should navigate without wrapping and switch to the focused desktop", async () => {
    const user = userEvent.setup();
    const { onSelectDesktop } = renderIndicators();
    await user.click(screen.getByRole("button", { name: "Desktop 2 of 4: Build" }));

    await user.keyboard("{ArrowRight}{End}{ArrowRight}");
    const last = screen.getByRole("button", { name: "Desktop 4 of 4: Research" });
    expect(last).toHaveFocus();

    await user.keyboard(" ");
    expect(onSelectDesktop).toHaveBeenCalledWith("research");

    await user.keyboard("{Home}{ArrowLeft}{Enter}");
    const first = screen.getByRole("button", { name: "Desktop 1 of 4: Control" });
    expect(first).toHaveFocus();
    expect(onSelectDesktop).toHaveBeenCalledWith("control");
  });

  it("Should mark an off-screen desktop that needs you and clear the mark once it is active", () => {
    const needsYou = DESKTOPS.map(desktop =>
      desktop.id === "research" || desktop.id === "build" ? { ...desktop, needsYou: true } : desktop
    );
    renderIndicators({ desktops: needsYou });

    const offScreen = screen.getByRole("button", {
      name: "Desktop 4 of 4: Research — needs you",
    });
    expect(offScreen).toHaveAttribute("data-needs-you", "true");
    const active = screen.getByRole("button", { name: "Desktop 2 of 4: Build" });
    expect(active).not.toHaveAttribute("data-needs-you");
  });

  it("Should leave the active indicator inert to a click", async () => {
    const user = userEvent.setup();
    const { onSelectDesktop, onOpenOverview } = renderIndicators();

    await user.click(screen.getByRole("button", { name: "Desktop 2 of 4: Build" }));
    expect(onSelectDesktop).not.toHaveBeenCalled();
    expect(onOpenOverview).not.toHaveBeenCalled();
  });

  it("Should keep overview disclosure available while desktop switching is unavailable", async () => {
    const user = userEvent.setup();
    const { onOpenOverview, onSelectDesktop } = renderIndicators({
      desktops: MANY_DESKTOPS,
      activeDesktopId: "qa",
      canSwitchDesktop: false,
    });

    const desktop = screen.getByRole("button", { name: "Desktop 4 of 9: Research" });
    expect(desktop).toHaveAttribute("aria-disabled", "true");
    await user.click(desktop);
    expect(onSelectDesktop).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Show 2 earlier desktops" }));
    expect(onOpenOverview).toHaveBeenCalledTimes(1);
  });

  it("Should offer every desktop action on a middle desktop and run each", async () => {
    const callbacks = renderIndicators();

    let user = await openMenu("Desktop 2 of 4: Build");
    expect(menuItems()).toEqual([
      "Rename",
      "Move left",
      "Move right",
      "Close desktop",
      "New desktop",
    ]);
    await user.click(screen.getByRole("menuitem", { name: "Move right" }));
    expect(callbacks.onReorderDesktop).toHaveBeenLastCalledWith("build", 2);

    user = await openMenu("Desktop 2 of 4: Build");
    await user.click(screen.getByRole("menuitem", { name: "Move left" }));
    expect(callbacks.onReorderDesktop).toHaveBeenLastCalledWith("build", 0);

    user = await openMenu("Desktop 2 of 4: Build");
    await user.click(screen.getByRole("menuitem", { name: "Rename" }));
    expect(callbacks.onRenameDesktop).toHaveBeenCalledWith("build");

    user = await openMenu("Desktop 2 of 4: Build");
    await user.click(screen.getByRole("menuitem", { name: "Close desktop" }));
    expect(callbacks.onCloseDesktop).toHaveBeenCalledWith("build");

    user = await openMenu("Desktop 2 of 4: Build");
    await user.click(screen.getByRole("menuitem", { name: "New desktop" }));
    expect(callbacks.onCreateDesktop).toHaveBeenCalledTimes(1);
    expect(callbacks.onSelectDesktop).not.toHaveBeenCalled();
  });

  it("Should leave out Move left on the first desktop and Move right on the last", async () => {
    renderIndicators();

    let user = await openMenu("Desktop 1 of 4: Control");
    expect(menuItems()).toEqual(["Rename", "Move right", "Close desktop", "New desktop"]);
    await user.keyboard("{Escape}");

    user = await openMenu("Desktop 4 of 4: Research");
    expect(menuItems()).toEqual(["Rename", "Move left", "Close desktop", "New desktop"]);
    await user.keyboard("{Escape}");
  });

  it("Should leave out Close desktop on the last remaining desktop", async () => {
    renderIndicators({ desktops: [DESKTOPS[1]!] });

    await openMenu("Desktop 1 of 1: Build");
    expect(menuItems()).toEqual(["Rename", "New desktop"]);
  });

  it("Should open the desktop menu from the keyboard on the focused indicator", async () => {
    const user = userEvent.setup();
    const { onCloseDesktop } = renderIndicators();
    await user.click(screen.getByRole("button", { name: "Desktop 2 of 4: Build" }));
    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("button", { name: "Desktop 3 of 4: Review" })).toHaveFocus();

    await user.keyboard("{Shift>}{F10}{/Shift}");
    await user.click(await screen.findByRole("menuitem", { name: "Close desktop" }));
    expect(onCloseDesktop).toHaveBeenCalledWith("review");
  });

  it("Should grow every desktop into a labelled drop target while a tab is dragged", () => {
    renderIndicators({
      desktops: MANY_DESKTOPS,
      activeDesktopId: "qa",
      dragSourceDesktopId: "qa",
      dropTargetDesktopId: "release",
      overflowHover: "later",
    });

    const target = screen.getByRole("button", { name: "Move tab to Release" });
    expect(target).toHaveAttribute("data-drop-target", "true");
    expect(target.closest(`[${DESKTOP_DROP_TARGET_ATTRIBUTE}]`)).toHaveAttribute(
      DESKTOP_DROP_TARGET_ATTRIBUTE,
      "release"
    );
    expect(screen.getByRole("button", { name: "Move tab to QA" })).not.toHaveAttribute(
      "data-drop-target"
    );
    expect(screen.queryByRole("button", { name: /^Desktop \d/ })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Show 2 later desktops" })).toHaveTextContent("+2");
  });

  it("Should name the drop targets for a window while a window head is dragged", () => {
    renderIndicators({ dragSourceDesktopId: "build", dragKind: "window" });

    const target = screen.getByRole("button", { name: "Move window to Review" });
    expect(target.closest(`[${DESKTOP_DROP_TARGET_ATTRIBUTE}]`)).toHaveAttribute(
      DESKTOP_DROP_TARGET_ATTRIBUTE,
      "review"
    );
    expect(screen.queryByRole("button", { name: /^Move tab to/ })).not.toBeInTheDocument();
  });
});

const DRAG_SESSION: TabDragSession = {
  desktopId: "qa",
  revision: 1,
  source: { kind: "tab", tabId: "tab:a", windowId: "win:a", nodeId: "leaf:a" },
  tabOrder: ["tab:a"],
  tabLeads: ["win:a"],
  pinnedCount: 0,
  activeTabId: "tab:a",
  splitTabId: null,
};

function OverflowHoverProbe({ onEnter }: { onEnter: (direction: string) => void }) {
  const [container, setContainer] = useState<HTMLElement | null>(null);
  const hovered = useDesktopOverflowDragHover(container, onEnter);
  return (
    <div ref={setContainer} data-hovered={hovered ?? "none"} data-testid="indicators">
      <button data-desktop-overflow="later" type="button">
        +2
      </button>
    </div>
  );
}

describe("useDesktopOverflowDragHover", () => {
  afterEach(() => {
    act(() => osTabDragStore.trigger.cancelled());
  });

  it("Should open once when a dragged tab comes to rest on an overflow control", () => {
    const onEnter = vi.fn();
    render(<OverflowHoverProbe onEnter={onEnter} />);
    const overflow = screen.getByRole("button", { name: "+2" });
    overflow.getBoundingClientRect = () =>
      ({
        x: 200,
        y: 0,
        left: 200,
        top: 0,
        right: 230,
        bottom: 30,
        width: 30,
        height: 30,
      }) as DOMRect;
    const move = (x: number) =>
      act(() =>
        osTabDragStore.trigger.moved({
          point: { x, y: 15 },
          zone: { kind: "outside" },
          swapHeld: false,
        })
      );

    act(() => osTabDragStore.trigger.pressed({ session: DRAG_SESSION, point: { x: 20, y: 15 } }));
    move(120);
    expect(onEnter).not.toHaveBeenCalled();

    move(210);
    move(215);
    expect(onEnter).toHaveBeenCalledTimes(1);
    expect(onEnter).toHaveBeenCalledWith("later");
    expect(screen.getByTestId("indicators")).toHaveAttribute("data-hovered", "later");

    move(120);
    expect(screen.getByTestId("indicators")).toHaveAttribute("data-hovered", "none");
  });
});

describe("desktopIdsNeedingYou", () => {
  function sessionWindow(id: string, sessionId: string, desktopId: string): OsWindow {
    return {
      id,
      app: "session",
      instanceKey: sessionId,
      desktopId,
    } as OsWindow;
  }

  function sessionRow(id: string, stale = false): OsAttentionRow {
    return {
      kind: "session",
      id,
      title: id,
      agentName: "codex",
      workspaceId: "ws",
      workspaceLabel: "ws",
      badge: "needs-attention",
      reason: "Needs input",
      changedAt: "2026-09-29T00:00:00Z",
      muted: false,
      stale,
    } as OsAttentionRow;
  }

  it("Should return the desktops whose session windows need you, ignoring stale rows", () => {
    const windows = {
      a: sessionWindow("a", "sess-a", "desktop:one"),
      b: sessionWindow("b", "sess-b", "desktop:two"),
      c: sessionWindow("c", "sess-c", "desktop:three"),
      tasks: {
        id: "tasks",
        app: "tasks",
        instanceKey: "sess-a",
        desktopId: "desktop:four",
      } as OsWindow,
    };

    const ids = desktopIdsNeedingYou(windows, [sessionRow("sess-a"), sessionRow("sess-c", true)]);

    expect([...ids]).toEqual(["desktop:one"]);
  });
});
