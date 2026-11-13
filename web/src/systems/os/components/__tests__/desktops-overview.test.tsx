// Suite: desktops overview
// Invariant: overview entry focuses the nearest requested hidden desktop (else the current one);
// each card shows its name, tab count, thumbnail and first tabs with "+N more"; choosing a tab or
// a thumbnail emits one semantic callback and closes the overlay; rename, Move left / Move right
// and Move a tab emit one callback each; closing uses the in-place confirm, asks for a destination
// when the desktop has tabs and is absent on the last desktop; Escape inside a card's rename or
// confirm leaves the overlay open; opened from a desktop indicator's Rename or Close desktop, the
// card starts in that action with focus inside it (US-015.EC-2); async states stay recoverable; the thumbnail draws the active
// tab's split tree once per window, by weight.
// Boundary IN: DesktopsOverview state rendering, local forms, accessibility, keyboard, callback
// payloads, and DesktopLayoutThumbnail.
// Boundary OUT: TanStack Query snapshots, the tab drag store and its hit tests, window-manager
// coordination, mutations, revisions, and persistence.
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { OsWindow } from "../../lib/os-types";
import { DesktopLayoutThumbnail } from "../desktop-layout-thumbnail";
import {
  DesktopsOverview,
  type DesktopOverviewItem,
  type DesktopOverviewTab,
  type DesktopsOverviewProps,
} from "../desktops-overview";

function osWindow(id: string, app: OsWindow["app"]): OsWindow {
  return {
    id,
    app,
    instanceKey: null,
    route: { pathname: `/${app}`, search: {} },
    navStack: [],
    desktopId: "control",
    tabId: `tab-${id}`,
    nodeId: `n-${id}`,
    pinned: false,
    inSplit: false,
    tabActive: true,
    shown: true,
  };
}

function tab(id: string, ...members: OsWindow[]): DesktopOverviewTab {
  return { id, leadWindowId: members[0]!.id, members };
}

const DASHBOARD = osWindow("dashboard", "dashboard");
const AGENTS = osWindow("agents", "agents");
const TASKS = osWindow("tasks", "tasks");

const DESKTOPS: DesktopOverviewItem[] = [
  {
    id: "control",
    name: "Control",
    tabs: [tab("tab-dashboard", DASHBOARD)],
    activeTabId: "tab-dashboard",
    activeRoot: { id: "n-dashboard", kind: "leaf", windowId: "dashboard" },
  },
  {
    id: "build",
    name: "Build",
    tabs: [tab("tab-agents", AGENTS, TASKS)],
    activeTabId: "tab-agents",
    activeRoot: null,
    needsYou: true,
    switchShortcut: "⌃2",
  },
  {
    id: "research",
    name: "Research",
    tabs: [],
    activeTabId: null,
    activeRoot: null,
  },
];

function renderOverview(overrides: Partial<DesktopsOverviewProps> = {}) {
  const callbacks = {
    onOpenChange: vi.fn(),
    onCreateDesktop: vi.fn(),
    onSwitchDesktop: vi.fn(),
    onRenameDesktop: vi.fn(),
    onReorderDesktop: vi.fn(),
    onDeleteDesktop: vi.fn(),
    onMoveTab: vi.fn(),
    onChooseTab: vi.fn(),
    onRetry: vi.fn(),
    onResolveConflict: vi.fn(),
  };
  const props: DesktopsOverviewProps = {
    open: true,
    state: { status: "ready", desktops: DESKTOPS, activeDesktopId: "control" },
    ...callbacks,
    ...overrides,
  };
  render(<DesktopsOverview {...props} />);
  return callbacks;
}

function card(name: string): HTMLElement {
  return screen.getByRole("listitem", { name });
}

/** Opens a card's context menu the keyboard way: Shift+F10 on its focused thumbnail. */
async function openCardMenu(user: ReturnType<typeof userEvent.setup>, thumbnail: string) {
  screen.getByRole("button", { name: thumbnail }).focus();
  await user.keyboard("{Shift>}{F10}{/Shift}");
}

describe("DesktopsOverview", () => {
  it.each([
    {
      direction: "earlier" as const,
      hiddenDesktopIds: ["control", "build"],
      expectedName: "Switch to Build — needs you",
    },
    {
      direction: "later" as const,
      hiddenDesktopIds: ["research"],
      expectedName: "Switch to Research",
    },
  ])(
    "Should focus the nearest $direction hidden desktop when opened from pager overflow",
    async ({ direction, hiddenDesktopIds, expectedName }) => {
      renderOverview({ initialFocusSegment: { direction, hiddenDesktopIds } });

      await waitFor(() => expect(screen.getByRole("button", { name: expectedName })).toHaveFocus());
    }
  );

  it("Should focus the current desktop when opened without a pager request", async () => {
    renderOverview();

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Current desktop Control" })).toHaveFocus()
    );
  });

  it("US-014.AC-2: Should show each card's name, tab count, needs-you mark, chord and tabs", () => {
    renderOverview();

    const build = card("Build");
    expect(within(build).getByText("1 tab")).toBeInTheDocument();
    expect(within(build).getByText("⌃2")).toBeInTheDocument();
    expect(within(build).getByLabelText("Needs you")).toBeInTheDocument();
    const tabs = within(build).getByRole("list", { name: "Tabs on Build" });
    expect(within(tabs).getByRole("button", { name: "Go to Agents and 1 more" })).toBeVisible();
    expect(within(card("Research")).getByText("0 tabs")).toBeInTheDocument();
    expect(card("Control")).toHaveAttribute("aria-current", "true");
  });

  it("US-014.EC-3: Should list the first four tabs and count the rest", () => {
    const many = ["a", "b", "c", "d", "e", "f"].map(id => tab(`tab-${id}`, osWindow(id, "files")));
    renderOverview({
      state: {
        status: "ready",
        desktops: [{ ...DESKTOPS[0]!, tabs: many }, DESKTOPS[1]!],
        activeDesktopId: "control",
      },
    });

    const control = card("Control");
    expect(within(control).getByText("6 tabs")).toBeInTheDocument();
    expect(within(control).getAllByRole("button", { name: /^Go to / })).toHaveLength(4);
    expect(within(control).getByText("+2 more")).toBeInTheDocument();
  });

  it("US-014.AC-5: Should jump to a chosen tab and close the overlay", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview();

    await user.click(screen.getByRole("button", { name: "Go to Agents and 1 more" }));
    expect(callbacks.onChooseTab).toHaveBeenCalledWith("build", "agents");
    expect(callbacks.onOpenChange).toHaveBeenCalledWith(false);
  });

  it("Should switch from a card's thumbnail and create from the header", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview();

    await user.click(screen.getByRole("button", { name: "New desktop" }));
    expect(callbacks.onCreateDesktop).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("button", { name: "Switch to Research" }));
    expect(callbacks.onSwitchDesktop).toHaveBeenCalledWith("research");
    expect(callbacks.onOpenChange).toHaveBeenCalledWith(false);
  });

  it("US-014.AC-3: Should rename inline; Escape leaves the rename and keeps the overview", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview();

    await user.click(within(card("Build")).getByRole("button", { name: "Rename" }));
    const name = screen.getByLabelText("Desktop name");
    await user.keyboard("{Escape}");
    expect(name).not.toBeInTheDocument();
    expect(callbacks.onOpenChange).not.toHaveBeenCalled();

    await user.click(within(card("Build")).getByRole("button", { name: "Rename" }));
    await user.clear(screen.getByLabelText("Desktop name"));
    await user.type(screen.getByLabelText("Desktop name"), "Implementation{Enter}");
    expect(callbacks.onRenameDesktop).toHaveBeenCalledWith("build", "Implementation");
  });

  it("US-014.AC-3: Should keep Move left / Move right in the card menu, absent at either end", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview();

    await openCardMenu(user, "Current desktop Control");
    expect(await screen.findByRole("menuitem", { name: "Move right" })).toBeVisible();
    expect(screen.queryByRole("menuitem", { name: "Move left" })).not.toBeInTheDocument();
    await user.keyboard("{Escape}");

    fireEvent.contextMenu(card("Research"));
    expect(screen.queryByRole("menuitem", { name: "Move right" })).not.toBeInTheDocument();
    await user.click(await screen.findByRole("menuitem", { name: "Move left" }));
    expect(callbacks.onReorderDesktop).toHaveBeenCalledWith("research", 1);
  });

  it("US-016.AC-2: Should move a tab to another desktop from the card menu", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview();

    fireEvent.contextMenu(card("Build"));
    await user.click(await screen.findByRole("menuitem", { name: "Move a tab" }));
    await user.click(await screen.findByRole("menuitem", { name: /^Agents/ }));
    await user.click(await screen.findByRole("menuitem", { name: "Research" }));
    expect(callbacks.onMoveTab).toHaveBeenCalledWith("agents", "research");
  });

  it("US-014.EC-1: Should ask for a destination before confirming the close of a desktop with tabs", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview();
    const build = card("Build");

    await user.click(within(build).getByRole("button", { name: "Close Build" }));
    const confirm = within(build).getByRole("button", { name: "Confirm: Close Build" });
    expect(confirm).toBeDisabled();
    await user.selectOptions(within(build).getByRole("combobox"), "Research");
    await user.click(confirm);
    expect(callbacks.onDeleteDesktop).toHaveBeenCalledWith("build", "research");
  });

  it("US-014.AC-4: Should cancel the close with Escape and keep the overview open", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview();
    const build = card("Build");

    await user.click(within(build).getByRole("button", { name: "Close Build" }));
    expect(within(build).getByRole("combobox")).toBeInTheDocument();
    await user.keyboard("{Escape}");
    expect(within(build).queryByRole("combobox")).not.toBeInTheDocument();
    expect(within(build).getByRole("button", { name: "Close Build" })).toHaveFocus();
    expect(callbacks.onDeleteDesktop).not.toHaveBeenCalled();
    expect(callbacks.onOpenChange).not.toHaveBeenCalled();
  });

  it("US-015.EC-2: Should open on a desktop's destination-and-confirm close from its indicator menu", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview({ desktopAction: { desktopId: "build", action: "close" } });
    const build = card("Build");

    await waitFor(() =>
      expect(within(build).getByRole("button", { name: "Cancel" })).toHaveFocus()
    );
    const confirm = within(build).getByRole("button", { name: "Confirm: Close Build" });
    expect(confirm).toBeDisabled();
    // Only the asked desktop opens its confirm.
    expect(within(card("Control")).getByRole("button", { name: "Close Control" })).toBeVisible();
    await user.selectOptions(within(build).getByRole("combobox"), "Control");
    await user.click(confirm);
    expect(callbacks.onDeleteDesktop).toHaveBeenCalledWith("build", "control");
  });

  it("US-015.AC-3: Should open on a desktop's rename field from its indicator menu", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview({
      desktopAction: { desktopId: "research", action: "rename" },
    });

    const name = within(card("Research")).getByLabelText("Desktop name");
    await waitFor(() => expect(name).toHaveFocus());
    await user.clear(name);
    await user.type(name, "Notes{Enter}");
    expect(callbacks.onRenameDesktop).toHaveBeenCalledWith("research", "Notes");
  });

  it("Should close a desktop without tabs with no destination", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview();
    const research = card("Research");

    await user.click(within(research).getByRole("button", { name: "Close Research" }));
    expect(within(research).queryByRole("combobox")).not.toBeInTheDocument();
    await user.click(within(research).getByRole("button", { name: "Confirm: Close Research" }));
    expect(callbacks.onDeleteDesktop).toHaveBeenCalledWith("research", null);
  });

  it("US-014.EC-2: Should offer no close on the last desktop", () => {
    renderOverview({
      state: { status: "ready", desktops: [DESKTOPS[0]!], activeDesktopId: "control" },
    });

    expect(screen.queryByRole("button", { name: "Close Control" })).not.toBeInTheDocument();
  });

  it("US-016.AC-1: Should mark the card a dragged tab would move to", () => {
    renderOverview({ dropTargetDesktopId: "research" });

    expect(card("Research")).toHaveAttribute("data-drop-target", "true");
    expect(within(card("Research")).getByText("Move tab here")).toBeInTheDocument();
    expect(card("Build")).not.toHaveAttribute("data-drop-target");
    expect(card("Build")).toHaveAttribute("data-desktop-drop-target", "build");
  });

  it("Should hand a press on a tab row to the tab drag", () => {
    const onTabPointerDown = vi.fn();
    renderOverview({ onTabPointerDown });

    fireEvent.pointerDown(screen.getByRole("button", { name: "Go to Agents and 1 more" }), {
      button: 0,
    });
    expect(onTabPointerDown).toHaveBeenCalledWith("tab-agents", expect.anything());
  });

  it("Should move between cards with the arrow keys and close with Done", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview();
    const current = screen.getByRole("button", { name: "Current desktop Control" });
    await waitFor(() => expect(current).toHaveFocus());

    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("button", { name: "Switch to Build — needs you" })).toHaveFocus();
    await user.keyboard("{End}");
    expect(screen.getByRole("button", { name: "Switch to Research" })).toHaveFocus();
    await user.keyboard("{Home}");
    expect(current).toHaveFocus();
    await user.keyboard("{ArrowRight}{ArrowRight}{Enter}");
    expect(callbacks.onSwitchDesktop).toHaveBeenCalledWith("research");

    await user.click(screen.getByRole("button", { name: "Done" }));
    expect(callbacks.onOpenChange).toHaveBeenLastCalledWith(false);
  });

  it("US-014.AC-1: Should close on Escape", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview();
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Current desktop Control" })).toHaveFocus()
    );

    await user.keyboard("{Escape}");
    expect(callbacks.onOpenChange).toHaveBeenCalledWith(false);
    expect(callbacks.onSwitchDesktop).not.toHaveBeenCalled();
  });

  it("Should expose loading semantics without rendering plausible desktop data", () => {
    renderOverview({ state: { status: "loading" } });

    expect(screen.getByRole("status")).toHaveTextContent("Loading desktops");
    expect(screen.queryByRole("button", { name: /Switch to/ })).not.toBeInTheDocument();
  });

  it("Should make every desktop mutation unavailable when no registered client is ready", () => {
    renderOverview({ canMutate: false });

    expect(screen.getByRole("button", { name: "New desktop" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Switch to Research" })).toBeDisabled();
    expect(within(card("Build")).getByRole("button", { name: "Rename" })).toBeDisabled();
    expect(within(card("Build")).getByRole("button", { name: "Close Build" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Go to Agents and 1 more" })).toBeDisabled();
  });

  it("Should retry an explicit load failure", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview({
      state: { status: "error", message: "Desktop snapshot unavailable." },
      canMutate: false,
    });

    expect(screen.getByRole("alert")).toHaveTextContent("Desktop snapshot unavailable.");
    await user.click(screen.getByRole("button", { name: "Retry loading" }));
    expect(callbacks.onRetry).toHaveBeenCalledTimes(1);
  });

  it("Should resolve a revision conflict before accepting another snapshot", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview({
      state: { status: "conflict", message: "Desktop order changed remotely." },
    });

    expect(screen.getByRole("alert")).toHaveTextContent("Desktop order changed remotely.");
    await user.click(screen.getByRole("button", { name: "Reload desktops" }));
    expect(callbacks.onResolveConflict).toHaveBeenCalledTimes(1);
  });

  it("Should offer creation when the authoritative desktop list is empty", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview({
      state: { status: "ready", desktops: [], activeDesktopId: null },
    });

    expect(screen.getByText("No desktops")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "New desktop" }));
    expect(callbacks.onCreateDesktop).toHaveBeenCalledTimes(1);
  });
});

describe("DesktopLayoutThumbnail", () => {
  it("Should draw each window of the active tab's tree once, sized by its weight", () => {
    const apps: Record<string, OsWindow["app"]> = { tasks: "tasks", s1: "session", s2: "terminal" };
    const { container } = render(
      <DesktopLayoutThumbnail
        activeTabId="tab-main"
        appOf={windowId => apps[windowId]}
        root={{
          id: "root",
          kind: "split",
          axis: "horizontal",
          weights: [0.6, 0.4],
          children: [
            { id: "n-tasks", kind: "leaf", windowId: "tasks" },
            {
              id: "side",
              kind: "split",
              axis: "vertical",
              weights: [0.5, 0.5],
              children: [
                { id: "n-s1", kind: "leaf", windowId: "s1" },
                { id: "n-s2", kind: "leaf", windowId: "s2" },
              ],
            },
          ],
        }}
        tabIds={["tab-main", "tab-other"]}
      />
    );

    const tiles = [
      ...container.querySelectorAll<HTMLElement>('[data-slot="desktop-thumbnail-window"]'),
    ];
    expect(tiles.map(tile => tile.dataset.windowId)).toEqual(["tasks", "s1", "s2"]);
    expect(tiles[0]!.parentElement).toHaveStyle({ flex: "0.6 1 0" });
  });

  it("Should draw only the tab strip before the desktop has an active tab", () => {
    const { container } = render(
      <DesktopLayoutThumbnail activeTabId={null} appOf={() => undefined} root={null} tabIds={[]} />
    );
    expect(container.querySelector('[data-slot="desktop-thumbnail-window"]')).toBeNull();
  });
});
