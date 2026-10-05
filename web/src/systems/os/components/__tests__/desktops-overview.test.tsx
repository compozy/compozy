// Suite: desktops overview
// Invariant: overview entry focuses the nearest requested hidden desktop (else the current one), each
// management action emits one semantic callback, the card grid is keyboard-navigable, and async states
// stay recoverable; each card's thumbnail draws every visible frame once (tiled stacks and floating decks
// as their active tab +N).
// Boundary IN: DesktopsOverview state rendering, local forms, accessibility, keyboard, callback payloads, and
// DesktopLayoutThumbnail tiles.
// Boundary OUT: TanStack Query snapshots, window-manager coordination, mutations, revisions, and persistence.
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { OsWindow } from "../../lib/os-types";
import type { LayoutProjection } from "../../lib/window-manager-types";
import { DesktopLayoutThumbnail } from "../desktop-layout-thumbnail";
import {
  DesktopsOverview,
  type DesktopOverviewItem,
  type DesktopsOverviewProps,
} from "../desktops-overview";

const DESKTOPS: DesktopOverviewItem[] = [
  {
    id: "control",
    name: "Control",
    thumbnail: <span />,
    windows: [{ id: "dashboard", title: "Dashboard", detail: "Workspace status" }],
  },
  {
    id: "build",
    name: "Build",
    thumbnail: <span />,
    windows: [{ id: "agents", title: "Agents", detail: "Window manager" }],
    needsYou: true,
    switchShortcut: "⌃2",
  },
  {
    id: "research",
    name: "Research",
    thumbnail: <span />,
    windows: [],
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
    onMoveWindow: vi.fn(),
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

  it("Should emit switch, create, reorder, rename, move, and transfer-on-delete actions", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview();

    const [headerCreate, cardCreate] = screen.getAllByRole("button", { name: "New desktop" });
    await user.click(headerCreate);
    await user.click(cardCreate);
    expect(callbacks.onCreateDesktop).toHaveBeenCalledTimes(2);

    await user.click(screen.getByRole("button", { name: "Switch to Research" }));
    expect(callbacks.onSwitchDesktop).toHaveBeenCalledWith("research");
    expect(callbacks.onOpenChange).toHaveBeenCalledWith(false);

    await user.click(screen.getByRole("button", { name: "Actions for Research" }));
    await user.click(await screen.findByRole("menuitem", { name: "Move left" }));
    expect(callbacks.onReorderDesktop).toHaveBeenCalledWith("research", 1);

    await user.click(screen.getByRole("button", { name: "Actions for Build" }));
    await user.click(await screen.findByRole("menuitem", { name: "Rename" }));
    const name = screen.getByLabelText("Desktop name");
    await user.clear(name);
    await user.type(name, "Implementation");
    await user.click(screen.getByRole("button", { name: "Save name" }));
    expect(callbacks.onRenameDesktop).toHaveBeenCalledWith("build", "Implementation");

    await user.click(screen.getByRole("button", { name: "Actions for Build" }));
    // Keyboard through the nested submenus: window, then destination.
    const moveWindow = await screen.findByRole("menuitem", { name: "Move a window" });
    act(() => moveWindow.focus());
    await user.keyboard("{ArrowRight}");
    await waitFor(() => expect(screen.getByRole("menuitem", { name: "Agents" })).toHaveFocus());
    await user.keyboard("{ArrowRight}");
    await waitFor(() => expect(screen.getByRole("menuitem", { name: "Control" })).toHaveFocus());
    await user.keyboard("{ArrowDown}{Enter}");
    expect(callbacks.onMoveWindow).toHaveBeenCalledWith("agents", "build", "research");

    await user.click(screen.getByRole("button", { name: "Actions for Build" }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete…" }));
    await user.click(screen.getByRole("button", { name: "Delete desktop" }));
    expect(callbacks.onDeleteDesktop).toHaveBeenCalledWith("build", "control");
  });

  it("Should mark an off-screen desktop that needs you and show its switch chord", () => {
    renderOverview();

    const build = screen.getByRole("button", { name: "Switch to Build — needs you" });
    expect(within(build).getByText("⌃2")).toBeInTheDocument();
    expect(within(build).getByLabelText("Needs you")).toBeInTheDocument();
  });

  it("Should move between cards with the arrow keys and close with Done", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview();
    const current = screen.getByRole("button", { name: "Current desktop Control" });
    await waitFor(() => expect(current).toHaveFocus());

    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("button", { name: "Switch to Build — needs you" })).toHaveFocus();
    await user.keyboard("{End}");
    expect(screen.getAllByRole("button", { name: "New desktop" }).at(-1)).toHaveFocus();
    await user.keyboard("{ArrowLeft}");
    expect(screen.getByRole("button", { name: "Switch to Research" })).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(callbacks.onSwitchDesktop).toHaveBeenCalledWith("research");

    await user.click(screen.getByRole("button", { name: "Done" }));
    expect(callbacks.onOpenChange).toHaveBeenLastCalledWith(false);
  });

  it("Should expose loading semantics without rendering plausible desktop data", () => {
    renderOverview({ state: { status: "loading" } });

    expect(screen.getByRole("dialog", { name: "Desktops" })).toHaveAttribute("aria-busy", "true");
    expect(screen.queryByRole("button", { name: /Switch to/ })).not.toBeInTheDocument();
  });

  it("Should make every desktop mutation unavailable when no registered client is ready", () => {
    renderOverview({ canMutate: false });

    for (const create of screen.getAllByRole("button", { name: "New desktop" })) {
      expect(create).toBeDisabled();
    }
    expect(screen.getByRole("button", { name: "Switch to Research" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Actions for Research" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Actions for Build" })).toBeDisabled();
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

function thumbnailWindow(
  id: string,
  app: OsWindow["app"],
  extra: Partial<OsWindow> = {}
): OsWindow {
  return {
    id,
    app,
    instanceKey: null,
    route: { pathname: `/${app}`, search: {} },
    navStack: [],
    pinned: false,
    desktopId: "control",
    placement: "tiled",
    rect: { x: 0, y: 0, w: 500, h: 500 },
    layer: 1,
    minimized: false,
    zoomed: false,
    groupId: null,
    nodeId: null,
    stackId: null,
    stackActive: true,
    parentAxis: null,
    ...extra,
  };
}

describe("DesktopLayoutThumbnail", () => {
  it("Should draw a floating tab deck as one floating tile titled by its active tab +N", () => {
    const zone = { x: 0, y: 0, w: 1, h: 1 };
    const rect = { x: 0, y: 0, w: 500, h: 1000 };
    const projection: LayoutProjection = {
      revision: 1,
      desktopId: "control",
      workArea: { x: 0, y: 0, w: 1000, h: 1000 },
      windows: [
        {
          windowId: "tasks",
          nodeId: "n-tasks",
          groupId: "g",
          rect,
          zone,
          stackId: null,
          active: true,
          adapted: false,
          parentAxis: null,
        },
        {
          windowId: "s1",
          nodeId: "n-s1",
          groupId: "g",
          rect,
          zone,
          stackId: "tiled-stack",
          active: true,
          adapted: false,
          parentAxis: null,
        },
        {
          windowId: "s2",
          nodeId: "n-s2",
          groupId: "g",
          rect,
          zone,
          stackId: "tiled-stack",
          active: false,
          adapted: false,
          parentAxis: null,
        },
      ],
      stacks: [
        {
          nodeId: "tiled-stack",
          groupId: "g",
          kind: "explicit",
          windowIds: ["s1", "s2"],
          activeWindowId: "s1",
          rect,
          zone,
        },
      ],
      seams: [],
      frameSeams: [],
      diagnostics: [],
    };
    const deckRect = { x: 600, y: 100, w: 200, h: 400 };
    const deckMember = (id: string, active: boolean) =>
      thumbnailWindow(id, "terminal", {
        placement: "stacked",
        rect: deckRect,
        stackId: "floating-deck",
        stackActive: active,
      });
    const windows = [
      thumbnailWindow("tasks", "tasks"),
      thumbnailWindow("s1", "session", { placement: "stacked", stackId: "tiled-stack" }),
      thumbnailWindow("s2", "session", {
        placement: "stacked",
        stackId: "tiled-stack",
        stackActive: false,
      }),
      deckMember("t1", false),
      deckMember("t2", true),
      deckMember("t3", false),
      thumbnailWindow("settings", "settings", { placement: "floating" }),
      thumbnailWindow("vault", "vault", {
        placement: "stacked",
        stackId: "hidden-deck",
        minimized: true,
      }),
    ];

    const { container } = render(
      <DesktopLayoutThumbnail projection={projection} windows={windows} />
    );

    const tiles = [
      ...container.querySelectorAll<HTMLElement>('[data-slot="desktop-thumbnail-window"]'),
    ];
    expect(tiles.map(tile => tile.textContent)).toEqual([
      "Session +1",
      "Tasks",
      "Terminal +2",
      "Settings",
    ]);
    expect(tiles[2]).toHaveClass("shadow-card");
    expect(tiles[2]).toHaveStyle({ left: "60%", top: "10%", width: "20%", height: "40%" });
  });
});
