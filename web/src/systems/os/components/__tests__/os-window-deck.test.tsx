// Suite: OS window deck rendering
// Invariant: a grouped frame exposes accessible tabs and deck controls while preserving truthful
// labels, attention state, pin state, and close-scope affordances for every member.
// Boundary IN: OsWindowDeck and OsWindowTab rendering plus their deck view-model boundary.
// Boundary OUT: window-manager command transport and pointer-gesture state machines.
import { createTopbarSlotStore, TooltipProvider, type TopbarSlotStore } from "@compozy/ui";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useDesktop } from "../../hooks/use-desktop";
import { useOsWindowDeck, type OsWindowDeckModel } from "../../hooks/use-os-window-deck";
import type { OsWindowFrameModel } from "../../lib/group-projection";
import type { OsDesktopRuntimeStore, OsWindow } from "../../lib/os-types";
import { OsWindowDeck } from "../os-window-deck";
import type { SessionPayload } from "@/systems/session";

vi.mock("../../hooks/use-desktop", () => ({ useDesktop: vi.fn() }));
vi.mock("../../hooks/use-os-window-deck", () => ({ useOsWindowDeck: vi.fn() }));

function windowFixture(id: string, overrides: Partial<OsWindow> = {}): OsWindow {
  return {
    id,
    app: "tasks",
    instanceKey: null,
    route: { pathname: "/tasks", search: {} },
    navStack: [],
    pinned: false,
    desktopId: "desktop:main",
    placement: "floating",
    rect: { x: 40, y: 50, w: 720, h: 480 },
    layer: 2,
    minimized: false,
    groupId: null,
    nodeId: null,
    stackId: "stack:main",
    stackActive: true,
    parentAxis: null,
    ...overrides,
  };
}

function frameFixture(overrides: Partial<OsWindowFrameModel> = {}): OsWindowFrameModel {
  return {
    id: "stack:main",
    desktopId: "desktop:main",
    kind: "floating",
    rect: { x: 40, y: 50, w: 720, h: 480 },
    zone: null,
    resizableEdges: { left: true, right: true, top: true, bottom: true },
    members: ["window:tasks", "window:catalog"],
    activeWindowId: "window:tasks",
    stackId: "stack:main",
    minimized: false,
    adapted: false,
    layer: 2,
    ...overrides,
  };
}

function sessionFixture(overrides: Partial<SessionPayload> = {}): SessionPayload {
  return {
    id: "session:review",
    name: "Review runtime permissions",
    agent_name: "codex",
    runtime: {
      status: "ready",
      transition: "initial_bind",
      effective: { provider: "codex" },
      selection_revision: 0,
    },
    workspace_id: "workspace:main",
    workspace_path: "/workspace/compozy",
    state: "active",
    badge: "waiting-for-auth",
    attachable: true,
    available_commands: [],
    created_at: "2026-07-20T12:00:00Z",
    updated_at: "2026-07-20T12:01:00Z",
    ...overrides,
    archived_at: overrides.archived_at ?? null,
    pending_interactions: overrides.pending_interactions ?? [],
  };
}

function deckModel(overrides: Partial<OsWindowDeckModel> = {}): OsWindowDeckModel {
  return {
    sessionById: new Map(),
    tabDrag: null,
    dropIndex: null,
    otherWindowCount: 0,
    registerTabs: vi.fn(),
    registerTab: vi.fn(),
    handleTabPointerDown: vi.fn(),
    wasDragged: () => false,
    activateTab: vi.fn(),
    closeTab: vi.fn(),
    closeOthers: vi.fn(),
    closeRight: vi.fn(),
    pinTab: vi.fn(),
    moveToNewWindow: vi.fn(),
    mergeAllWindows: vi.fn(),
    openNewTab: vi.fn(),
    ...overrides,
  };
}

function slotStore(crumb: string, crumbs: readonly string[] = []): TopbarSlotStore {
  const store = createTopbarSlotStore();
  store.publishSlot(
    {},
    {
      crumb,
      crumbs: crumbs.map((label, index) => ({ id: `${index}:${label}`, label, onSelect: vi.fn() })),
    }
  );
  return store;
}

let windows: Record<string, OsWindow>;
let deck: OsWindowDeckModel;

beforeEach(() => {
  windows = {
    "window:tasks": windowFixture("window:tasks"),
    "window:catalog": windowFixture("window:catalog", { app: "jobs" }),
  };
  deck = deckModel();
  vi.mocked(useDesktop).mockImplementation(selector =>
    selector({ windows } as unknown as OsDesktopRuntimeStore)
  );
  vi.mocked(useOsWindowDeck).mockImplementation(() => deck);
});

function renderDeck(
  frame: OsWindowFrameModel = frameFixture(),
  slotStores: ReadonlyMap<string, TopbarSlotStore> = new Map()
) {
  return render(
    <TooltipProvider delay={0}>
      <OsWindowDeck
        frame={frame}
        slotStores={slotStores}
        onTrafficLight={vi.fn()}
        dragHandleClassName="deck-drag-handle"
      />
    </TooltipProvider>
  );
}

describe("OsWindowDeck", () => {
  it("Should expose two member tabs, traffic lights, and the new-tab control in its deck row (UT-070)", () => {
    renderDeck();

    const deckRow = screen.getByTestId("os-window-deck-stack:main");
    expect(within(deckRow).getByRole("tablist", { name: "Open tabs" })).toBeInTheDocument();
    expect(within(deckRow).getAllByRole("tab")).toHaveLength(2);
    expect(within(deckRow).getByRole("button", { name: "Close window" })).toBeInTheDocument();
    expect(within(deckRow).getByRole("button", { name: "New tab (⌘T)" })).toBeInTheDocument();
  });

  it("Should render every tab in an overflow-capable tablist with uniform unpinned slots at volume (UT-077)", () => {
    const members = Array.from({ length: 30 }, (_, index) => `window:task-${index}`);
    windows = Object.fromEntries(members.map(id => [id, windowFixture(id)]));
    const pinnedId = members[0]!;
    windows[pinnedId] = windowFixture(pinnedId, { pinned: true });
    const frame = frameFixture({ members, activeWindowId: pinnedId });

    renderDeck(frame);

    const tablist = screen.getByRole("tablist", { name: "Open tabs" });
    expect(within(tablist).getAllByRole("tab")).toHaveLength(30);
    expect(tablist).toHaveClass("overflow-x-auto");
    for (const member of members.slice(1)) {
      expect(screen.getByTestId(`os-window-tab-menu-${member}`).parentElement).toHaveClass(
        "w-deck-tab-max",
        "min-w-deck-tab"
      );
    }
    const pinnedSlot = screen.getByTestId(`os-window-tab-menu-${pinnedId}`).parentElement;
    expect(pinnedSlot).not.toHaveClass("w-deck-tab-max");
    expect(pinnedSlot).toHaveClass("shrink-0");
  });

  it("Should surface complete, distinct leaf paths through the tab tooltip (UT-078)", async () => {
    const user = userEvent.setup();
    const title = "🧭 تقرير التقدم ".repeat(24);
    const slotStores = new Map([
      ["window:tasks", slotStore(title, ["Tasks", "Alpha"])],
      ["window:catalog", slotStore(title, ["Tasks", "Beta"])],
    ]);

    renderDeck(frameFixture(), slotStores);
    await user.hover(screen.getByTestId("os-window-tab-window:tasks"));

    const tooltipLeaf = await screen.findByText("Alpha");
    expect(tooltipLeaf.parentElement).toHaveTextContent(title.trim());
  });

  it("Should label a new-tab member truthfully without inventing a document title (UT-079)", () => {
    windows = {
      "window:new": windowFixture("window:new", { app: "new-tab" }),
      "window:tasks": windowFixture("window:tasks"),
    };
    renderDeck(
      frameFixture({ members: ["window:new", "window:tasks"], activeWindowId: "window:new" })
    );

    expect(screen.getByRole("tab", { name: /New tab/ })).toBeInTheDocument();
  });

  it("Should route close-other, close-right, and alternate close through their scoped deck actions (UT-088)", async () => {
    const user = userEvent.setup();
    deck = deckModel({ otherWindowCount: 1 });
    renderDeck();

    const tab = screen.getByTestId("os-window-tab-window:tasks");
    fireEvent.contextMenu(screen.getByTestId("os-window-tab-menu-window:tasks"));
    await user.click(await screen.findByRole("menuitem", { name: "Close other tabs" }));
    expect(deck.closeOthers).toHaveBeenCalledExactlyOnceWith("window:tasks");

    fireEvent.contextMenu(screen.getByTestId("os-window-tab-menu-window:tasks"));
    await user.click(await screen.findByRole("menuitem", { name: "Close tabs to the right" }));
    expect(deck.closeRight).toHaveBeenCalledExactlyOnceWith("window:tasks");

    fireEvent.click(within(tab).getByRole("button", { name: "Close Tasks" }), { altKey: true });
    expect(deck.closeOthers).toHaveBeenCalledTimes(2);
  });

  it("Should keep bulk-close controls disabled when every eligible peer is pinned (UT-088)", async () => {
    windows["window:catalog"] = windowFixture("window:catalog", { app: "jobs", pinned: true });
    renderDeck();

    fireEvent.contextMenu(screen.getByTestId("os-window-tab-menu-window:tasks"));
    expect(await screen.findByRole("menuitem", { name: "Close other tabs" })).toHaveAttribute(
      "data-disabled"
    );
    expect(screen.getByRole("menuitem", { name: "Close tabs to the right" })).toHaveAttribute(
      "data-disabled"
    );
  });

  it("Should keep live tab labels synchronized with each member slot (UT-091)", async () => {
    const tasksSlot = slotStore("Tasks");
    const slotStores = new Map([["window:tasks", tasksSlot]]);
    renderDeck(frameFixture(), slotStores);

    expect(screen.getByRole("tab", { name: /Tasks/ })).toBeInTheDocument();
    await act(async () => {
      tasksSlot.publishSlot({}, { crumb: "incident-42", crumbs: [] });
    });
    expect(screen.getByRole("tab", { name: /incident-42/ })).toBeInTheDocument();
  });

  it("Should expose no rename affordance in the tab or its context menu (UT-092)", () => {
    renderDeck();
    fireEvent.contextMenu(screen.getByTestId("os-window-tab-menu-window:tasks"));

    expect(screen.queryByRole("menuitem", { name: /rename/i })).toBeNull();
    expect(screen.queryByRole("button", { name: /rename/i })).toBeNull();
  });

  it("Should render latest session state without selection borrowing attention color (UT-097, UT-098)", () => {
    const sessionWindow = windowFixture("window:session", {
      app: "session",
      instanceKey: "session:review",
    });
    windows = { "window:session": sessionWindow, "window:tasks": windowFixture("window:tasks") };
    const frame = frameFixture({
      members: ["window:session", "window:tasks"],
      activeWindowId: "window:tasks",
    });
    deck = deckModel({ sessionById: new Map([["session:review", sessionFixture()]]) });
    const view = renderDeck(frame);

    const sessionTab = screen.getByTestId("os-window-tab-window:session");
    expect(within(sessionTab).getByRole("img", { name: "Session needs input" })).toHaveAttribute(
      "data-state",
      "needs-input"
    );
    expect(within(sessionTab).getByText("1")).toHaveAttribute("data-slot", "os-window-tab-badge");
    expect(screen.getByTestId("os-window-tab-window:tasks")).not.toHaveClass("text-accent");

    deck = deckModel({
      sessionById: new Map([["session:review", sessionFixture({ badge: "running" })]]),
    });
    vi.mocked(useOsWindowDeck).mockReturnValue(deck);
    view.rerender(
      <TooltipProvider delay={0}>
        <OsWindowDeck
          frame={frameFixture({
            members: ["window:session", "window:tasks"],
            activeWindowId: "window:tasks",
          })}
          slotStores={new Map()}
          onTrafficLight={vi.fn()}
          dragHandleClassName="deck-drag-handle"
        />
      </TooltipProvider>
    );
    expect(
      within(screen.getByTestId("os-window-tab-window:session")).getByRole("img", {
        name: "Session running",
      })
    ).toHaveAttribute("data-state", "running");
    expect(within(screen.getByTestId("os-window-tab-window:session")).queryByText("1")).toBeNull();
  });

  it("Should render independently addressable deck rows for separate tiled panes (UT-102)", () => {
    const secondaryMembers = ["window:catalog", "window:jobs"];
    windows["window:jobs"] = windowFixture("window:jobs", {
      app: "jobs",
      stackId: "stack:secondary",
    });
    render(
      <>
        <OsWindowDeck
          frame={frameFixture({ kind: "tiled" })}
          slotStores={new Map()}
          onTrafficLight={vi.fn()}
          dragHandleClassName="deck-drag-handle"
        />
        <OsWindowDeck
          frame={frameFixture({
            id: "stack:secondary",
            kind: "tiled",
            members: secondaryMembers,
            activeWindowId: "window:jobs",
          })}
          slotStores={new Map()}
          onTrafficLight={vi.fn()}
          dragHandleClassName="deck-drag-handle"
        />
      </>
    );

    expect(screen.getByTestId("os-window-deck-stack:main")).toBeInTheDocument();
    expect(screen.getByTestId("os-window-deck-stack:secondary")).toBeInTheDocument();
  });
});
