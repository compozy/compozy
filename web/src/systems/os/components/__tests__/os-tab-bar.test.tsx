// Suite: OS tab bar
// Invariant: the bar renders the active desktop's tabs in order with the client's active tab
// selected, activates a tab through its lead window, closes it by button, middle click, and menu
// (never a pinned tab by middle click), skips pinned tabs in bulk closes, and opens the overflow
// list only when tabs at the floor no longer fit. The tab menu offers a menu equivalent for every
// drag (Move left/right within the pin range, Split with ▸, Arrange ▸) and no retired control.
// Owning layer: OsTabBar + useOsTabBar over the shell projection. Width rules: lib/tab-width.
// Boundary OUT: runtime command execution and daemon reducers.
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { TooltipProvider } from "@compozy/ui";

import { OsShellContext, type OsShellHandle } from "../../contexts/os-shell-context";
import type { WindowManagerCommandOutcome, WindowManagerController } from "../../lib/os-types";
import { createWindowManagerProjectionAtom } from "../../lib/window-manager-projection";
import type {
  LayoutTab,
  WindowManagerConfig,
  WindowManagerSnapshot,
  WindowManagerWindow,
} from "../../lib/window-manager-types";
import { buildOsDesktopRuntimeView } from "../../lib/window-manager-view";
import { windowManagerStore } from "../../stores/window-manager-store";
import { OsTabBar } from "../os-tab-bar";

vi.mock("@/systems/workspace", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/workspace")>()),
  useActiveWorkspace: () => ({ runtimeWorkspaceId: null }),
}));

vi.mock("@/systems/session", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/session")>()),
  useSessions: () => ({ data: [] }),
}));

const CONFIG: WindowManagerConfig = {
  newWindowPolicy: "tab",
  smallViewportPolicy: "stack",
  focusPolicy: "click_directional",
  focusWrap: true,
  focusFollowsPointer: false,
  swapModifier: "shift",
  historyLimit: 50,
  navStackLimit: 50,
  closedEntryLimit: 20,
  desktopTransition: "slide",
  gaps: { inner: 0, top: 0, right: 0, bottom: 0, left: 0 },
  shortcuts: {},
  globalShortcuts: {},
  shortcutDefaults: {},
  effectiveShortcuts: {},
};

function win(id: string, app: string, tabId: string): WindowManagerWindow {
  return {
    id,
    app,
    instanceKey: null,
    route: { pathname: `/${app}`, search: {} },
    navStack: [],
    desktopId: "desktop:main",
    tabId,
  };
}

function solo(id: string, windowId: string, pinned = false): LayoutTab {
  return { id, pinned, root: { id: `leaf:${windowId}`, kind: "leaf", windowId } };
}

const TABS: LayoutTab[] = [
  solo("tab:dash", "w-dash", true),
  solo("tab:tasks", "w-tasks"),
  {
    id: "tab:split",
    pinned: false,
    root: {
      id: "split:1",
      kind: "split",
      axis: "horizontal",
      weights: [0.5, 0.5],
      children: [
        { id: "leaf:w-agents", kind: "leaf", windowId: "w-agents" },
        { id: "leaf:w-loops", kind: "leaf", windowId: "w-loops" },
      ],
    },
  },
  solo("tab:vault", "w-vault"),
];

function snapshot(tabs: LayoutTab[] = TABS): WindowManagerSnapshot {
  return {
    version: 6,
    workspaceId: "workspace:test",
    revision: 3,
    desktops: [{ id: "desktop:main", name: "Main", order: 0, tabs, activeTabId: "tab:tasks" }],
    windows: {
      "w-dash": win("w-dash", "dashboard", "tab:dash"),
      "w-tasks": win("w-tasks", "tasks", "tab:tasks"),
      "w-agents": win("w-agents", "agents", "tab:split"),
      "w-loops": win("w-loops", "loops", "tab:split"),
      "w-vault": win("w-vault", "vault", "tab:vault"),
    },
    closedEntryCount: 1,
    overrides: {},
    migration: null,
    updatedAt: "2026-11-12T00:00:00Z",
  };
}

function outcome(): WindowManagerCommandOutcome {
  return { accepted: true, completion: Promise.resolve(true) };
}

let stripWidth = 1200;

class ResizeObserverMock {
  constructor(private readonly callback: ResizeObserverCallback) {}
  observe(target: Element) {
    this.callback(
      [{ target, contentRect: new DOMRectReadOnly(0, 0, stripWidth, 40) } as ResizeObserverEntry],
      this as unknown as ResizeObserver
    );
  }
  unobserve() {}
  disconnect() {}
}

function renderBar(options: { tabs?: LayoutTab[]; focusOrder?: string[] } = {}) {
  const state = buildOsDesktopRuntimeView({
    viewportWidth: 1440,
    snapshot: snapshot(options.tabs),
    globalConfig: CONFIG,
    client: {
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:main",
      focusedWindowId: "w-tasks",
      focusOrder: options.focusOrder ?? ["w-tasks"],
      activeTab: {},
      connectedAt: "2026-11-12T00:00:00Z",
    },
    workArea: { x: 0, y: 0, w: 1440, h: 800 },
    workAreaOrigin: { x: 0, y: 0 },
    seamPreview: null,
    routeIntents: {},
    connectionStatus: "connected",
    loadError: null,
    wallpaper: "flat",
    reduceMotion: false,
  });
  const manager = {
    getState: () => state,
    closeWindowScoped: vi.fn(async () => true),
    pinWindow: vi.fn(() => outcome()),
    reopenWindow: vi.fn(() => outcome()),
    moveWindowToDesktop: vi.fn(),
    moveTab: vi.fn(),
    splitWithTab: vi.fn(),
    arrangeLayout: vi.fn(),
    balanceLayout: vi.fn(),
  } as unknown as WindowManagerController;
  const coordinator = {
    userFocus: vi.fn(async () => true),
    userClose: vi.fn(async () => true),
    userOpen: vi.fn(async () => "w-new"),
  };
  const shell = {
    projection: createWindowManagerProjectionAtom(state),
    manager,
    coordinator,
  } as unknown as OsShellHandle;
  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <OsShellContext.Provider value={shell}>
      <TooltipProvider delay={0}>{children}</TooltipProvider>
    </OsShellContext.Provider>
  );
  render(<OsTabBar />, { wrapper });
  return { manager, coordinator };
}

beforeEach(() => {
  stripWidth = 1200;
  vi.stubGlobal("ResizeObserver", ResizeObserverMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
  act(() => windowManagerStore.trigger.paletteIntentCleared());
});

describe("OsTabBar", () => {
  it("Should render the desktop's tabs in bar order with the client's active tab selected", () => {
    renderBar();
    const tabs = within(screen.getByTestId("os-tab-bar")).getAllByRole("tab");

    expect(tabs.map(tab => tab.getAttribute("data-testid"))).toEqual([
      "os-tab-tab:dash",
      "os-tab-tab:tasks",
      "os-tab-tab:split",
      "os-tab-tab:vault",
    ]);
    expect(screen.getByTestId("os-tab-tab:tasks")).toHaveAttribute("aria-selected", "true");
    expect(screen.getByTestId("os-tab-tab:dash")).toHaveAttribute("data-pinned");
    expect(screen.getByTestId("os-tab-tab:split")).toHaveAccessibleName("Agents and 1 more");
  });

  it("Should name a split by the member this client focused last", () => {
    renderBar({ focusOrder: ["w-loops", "w-tasks"] });

    expect(screen.getByTestId("os-tab-tab:split")).toHaveAttribute(
      "data-lead-window-id",
      "w-loops"
    );
  });

  it("Should activate a tab through its lead window", () => {
    const { coordinator } = renderBar();

    fireEvent.click(screen.getByTestId("os-tab-tab:vault"));

    expect(coordinator.userFocus).toHaveBeenCalledWith("w-vault");
  });

  it("Should close a single-window tab through the operator guard and a split as one tab", () => {
    const { coordinator, manager } = renderBar();

    const closeOf = (tabId: string) =>
      screen
        .getByTestId(`os-tab-${tabId}`)
        .closest('[data-slot="os-tab"]')!
        .querySelector<HTMLButtonElement>('[data-slot="os-tab-close"]')!;
    fireEvent.click(closeOf("tab:tasks"));
    fireEvent.click(closeOf("tab:split"));

    expect(coordinator.userClose).toHaveBeenCalledWith("w-tasks");
    expect(manager.closeWindowScoped).toHaveBeenCalledWith("w-agents", "group");
  });

  it("Should close an unpinned tab on middle click and never a pinned one", () => {
    const { coordinator } = renderBar();

    fireEvent(
      screen.getByTestId("os-tab-tab:vault"),
      new MouseEvent("auxclick", { bubbles: true, button: 1 })
    );
    fireEvent(
      screen.getByTestId("os-tab-tab:dash"),
      new MouseEvent("auxclick", { bubbles: true, button: 1 })
    );

    expect(coordinator.userClose).toHaveBeenCalledExactlyOnceWith("w-vault");
    const pinnedPlate = screen.getByTestId("os-tab-tab:dash").closest('[data-slot="os-tab"]')!;
    expect(pinnedPlate.querySelector('[data-slot="os-tab-close"]')).toBeNull();
  });

  it("Should run bulk closes from the tab menu and offer Unpin instead of Close on a pinned tab", async () => {
    const user = userEvent.setup();
    const { manager } = renderBar();

    fireEvent.contextMenu(screen.getByTestId("os-tab-tab:split"));
    await user.click(await screen.findByRole("menuitem", { name: "Close tabs to the left" }));
    expect(manager.closeWindowScoped).toHaveBeenCalledWith("w-agents", "left");

    fireEvent.contextMenu(screen.getByTestId("os-tab-tab:dash"));
    expect(await screen.findByRole("menuitem", { name: "Unpin tab" })).toBeInTheDocument();
    // "Close tab" may carry its shortcut in the name; "Close tabs to the …" must not match.
    expect(screen.queryByRole("menuitem", { name: /^Close tab(?!s)/ })).toBeNull();
    // Every other tab is unpinned and to its right.
    expect(screen.getByRole("menuitem", { name: "Close tabs to the left" })).toHaveAttribute(
      "aria-disabled",
      "true"
    );
  });

  it("Should move a tab one step within its range and leave out moves past the range [US-012]", async () => {
    const user = userEvent.setup();
    const { manager } = renderBar();

    fireEvent.contextMenu(screen.getByTestId("os-tab-tab:tasks"));
    // The first regular tab cannot move among the pins.
    await user.click(await screen.findByRole("menuitem", { name: "Move right" }));
    expect(screen.queryByRole("menuitem", { name: "Move left" })).toBeNull();
    expect(manager.moveTab).toHaveBeenCalledExactlyOnceWith("w-tasks", "right");

    fireEvent.contextMenu(screen.getByTestId("os-tab-tab:vault"));
    await user.click(await screen.findByRole("menuitem", { name: "Move left" }));
    expect(screen.queryByRole("menuitem", { name: "Move right" })).toBeNull();
    expect(manager.moveTab).toHaveBeenLastCalledWith("w-vault", "left");
  });

  it("Should split with another tab and arrange or balance from the tab menu [US-012.AC-3, AC-4]", async () => {
    const user = userEvent.setup();
    const { manager } = renderBar();

    fireEvent.contextMenu(screen.getByTestId("os-tab-tab:tasks"));
    act(() => screen.getByTestId("os-tab-split-with-menu-tab:tasks").focus());
    await user.keyboard("{ArrowRight}");
    // Every other tab is offered, never the tab itself.
    expect(await screen.findByTestId("os-tab-split-with-tab:vault")).toBeInTheDocument();
    expect(screen.queryByTestId("os-tab-split-with-tab:tasks")).toBeNull();
    await user.click(screen.getByTestId("os-tab-split-with-tab:vault"));
    expect(manager.splitWithTab).toHaveBeenCalledExactlyOnceWith("tab:tasks", "tab:vault");

    fireEvent.contextMenu(screen.getByTestId("os-tab-tab:split"));
    act(() => screen.getByTestId("os-tab-arrange-menu-tab:split").focus());
    await user.keyboard("{ArrowRight}");
    await user.click(await screen.findByRole("menuitem", { name: "Grid" }));
    expect(manager.arrangeLayout).toHaveBeenCalledExactlyOnceWith("w-agents", "grid");

    fireEvent.contextMenu(screen.getByTestId("os-tab-tab:split"));
    act(() => screen.getByTestId("os-tab-arrange-menu-tab:split").focus());
    await user.keyboard("{ArrowRight}");
    await user.click(await screen.findByRole("menuitem", { name: "Balance sizes" }));
    expect(manager.balanceLayout).toHaveBeenCalledExactlyOnceWith({ tabId: "tab:split" });
  });

  it("Should offer no retired window control in the tab menu [US-018.AC-1]", async () => {
    renderBar();
    fireEvent.contextMenu(screen.getByTestId("os-tab-tab:split"));
    await screen.findByRole("menuitem", { name: "Pin tab" });
    for (const retired of [/minimi[sz]e/i, /zoom/i, /float/i, /maximi[sz]e/i, /corner|quarter/i]) {
      expect(screen.queryByRole("menuitem", { name: retired })).toBeNull();
    }
  });

  it("Should open a New Tab after the active tab and raise its destination picker", async () => {
    const { coordinator } = renderBar();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /^New tab/ }));
    });

    expect(coordinator.userOpen).toHaveBeenCalledWith({ app: "new-tab", afterWindowId: "w-tasks" });
    expect(windowManagerStore.getSnapshot().context.paletteIntent).toEqual({
      kind: "destination",
      windowId: "w-new",
    });
  });

  it("Should show the overflow list only once tabs at the floor no longer fit", () => {
    stripWidth = 1200;
    renderBar();
    expect(screen.queryByRole("button", { name: "All 4 tabs" })).toBeNull();
  });

  it("Should list every tab in the overflow menu when the strip is too narrow", async () => {
    const user = userEvent.setup();
    stripWidth = 120;
    const { coordinator } = renderBar();

    await user.click(screen.getByRole("button", { name: "All 4 tabs" }));
    await user.click(await screen.findByRole("menuitem", { name: "Vault" }));

    expect(coordinator.userFocus).toHaveBeenCalledWith("w-vault");
  });
});
