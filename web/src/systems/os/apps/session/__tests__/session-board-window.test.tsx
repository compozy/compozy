// Suite: the Kanban board's session drawer (#1052)
// Invariant: a session window on Kanban shows its own session in a drawer beside the board,
// opened and closed through the window route (`drawer=open`) so Back/Forward, reload and window
// restore follow it. A card opens its session there; Escape, the close button and a click on the
// empty board close it — the click never while the composer holds unsent input — and focus
// returns to the session's card. Previous/Next step through the cards in board order, and
// archiving the drawer's session closes it and lands focus on the next card in its column.
// Selection gestures (#1053) — Cmd/Ctrl-click, Shift-click, any click while a selection is
// open — never open the drawer; a plain click does. A bulk archive or delete holding the
// drawer's session closes it and lands focus on a card outside the batch.
// The drawer renders the session view's own head (published into a drawer-local slot) without
// replacing the board window's.
// Owning layer: the OS session app's board window. The drawer primitive's layout, resize and
// focus mechanics are owned by `resizable-drawer.test.tsx`; route sync by the routing coordinator.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createTopbarSlotStore, TopbarSlotProvider, useTopbarSlot } from "@compozy/ui";

import { primarySessionFixture, resetSessionBoardColumnFolds } from "@/systems/session/testing";
import type { SessionPayload } from "@/systems/session";
import { sessionStore } from "@/systems/session/stores/session-store";
import { sessionsBoardViewFixture } from "@/systems/session/mocks/sessions-board-view-fixtures";
import { SESSION_LINK_SETS } from "@/systems/session/mocks/session-links-fixtures";
import { sessionDetailOptions } from "@/systems/session/lib/query-options";
import { ownerDataScope, workspaceSelectorForScope } from "@/systems/workspace";

const WINDOW_ID = "w-board";

const harness = vi.hoisted(() => ({
  desktop: {
    focusedId: "w-board",
    windowManagerConfig: {
      effectiveShortcuts: {
        "session.cycle.next": ["control+shift+ArrowDown"],
        "session.cycle.previous": ["control+shift+ArrowUp"],
      },
    },
    windows: {} as Record<
      string,
      {
        id: string;
        app: string;
        instanceKey: string;
        route: { pathname: string; search: Record<string, unknown> };
      }
    >,
  },
  userRetarget: vi.fn(),
  setView: vi.fn(),
  lifecycleOptions: {} as {
    onRemoved?: (session: { id: string }, action: "archive" | "later" | "delete") => void;
  },
  onArchive: vi.fn(),
  onArchiveMany: vi.fn(),
  onDeleteMany: vi.fn(),
  board: null as unknown,
}));

vi.mock("../../../hooks/use-desktop", () => ({
  useDesktop: (selector: (state: typeof harness.desktop) => unknown) => selector(harness.desktop),
}));
vi.mock("../../../hooks/use-os-shell", () => ({
  useOsShell: () => ({
    coordinator: { userRetarget: harness.userRetarget, userRetireSession: vi.fn() },
    manager: { getState: () => ({ ...harness.desktop, client: { focusOrder: [WINDOW_ID] } }) },
  }),
}));
vi.mock("../../../hooks/use-attention-jump", () => ({ useAttentionJump: () => vi.fn() }));
vi.mock("../../../hooks/use-window-live-data-enabled", () => ({
  useWindowLiveDataEnabled: () => true,
}));
vi.mock("@/hooks/use-document-active", () => ({ useDocumentActive: () => true }));
vi.mock("../../../components/os-sessions-derive-host", () => ({
  OsSessionsDeriveHost: ({ children }: { children: (state: object) => ReactNode }) =>
    children({ deriveOpen: false }),
}));
vi.mock("../../../hooks/use-session-list-lifecycle-actions", () => ({
  useSessionListLifecycleActions: (options: typeof harness.lifecycleOptions) => {
    harness.lifecycleOptions = options;
    return {
      actions: {
        pendingAction: null,
        pendingSessionIds: [],
        onStop: vi.fn(),
        onRename: vi.fn(),
        onArchive: harness.onArchive,
        onUnarchive: vi.fn(),
        onDelete: vi.fn(),
        onStopMany: vi.fn(),
        onArchiveMany: harness.onArchiveMany,
        onUnarchiveMany: vi.fn(),
        onDeleteMany: harness.onDeleteMany,
      },
      batchResults: [],
      deleteDialog: { open: false },
      renameDialog: { open: false },
      laterDialog: { open: false },
      processDialog: { open: false },
    };
  },
}));
vi.mock("@/systems/session/hooks/use-session-entry-preload", () => ({
  useSessionEntryPreload: () => ({}),
}));
vi.mock("@/systems/workspace", async importOriginal => ({
  ...(await importOriginal<Record<string, unknown>>()),
  useDataScope: () => ({ kind: "project", workspaceId: "ws-1" }),
  useWindowDataWorkspace: () => ({ kind: "project", workspaceId: "ws-1" }),
}));
vi.mock("@/systems/session", async importOriginal => ({
  ...(await importOriginal<Record<string, unknown>>()),
  useSessionsBoard: () => harness.board,
  useSessionWindowViewState: () => ({
    view: "kanban",
    setView: harness.setView,
    hideChildren: false,
    setHideChildren: vi.fn(),
    drawerWidth: 640,
    setDrawerWidth: vi.fn(),
  }),
  useSessionWindowOrder: () => ({ sort: "created", groupBy: "none", setSort: vi.fn() }),
  useSessionCreateActions: () => ({ openForAgent: vi.fn() }),
}));

import { SessionBoardWindow } from "../session-board-window";

function card(id: string, column: SessionPayload["board_column"]): SessionPayload {
  return {
    ...primarySessionFixture,
    id,
    name: `Session ${id}`,
    agent_name: "claude",
    workspace_id: "ws-1",
    board_column: column,
    archived_at: null,
    pull_request: null,
    pull_request_lookup: null,
  };
}

const empty = { sessions: [], count: 0, hasMore: false };

function setRoute(sessionId: string | null, drawer: boolean) {
  harness.desktop.windows = {
    [WINDOW_ID]: {
      id: WINDOW_ID,
      app: "session",
      instanceKey: sessionId ?? "",
      route: {
        pathname: sessionId ? `/agents/claude/sessions/${sessionId}` : "/sessions",
        search: drawer ? { drawer: "open" } : {},
      },
    },
  };
}

/** The session view the drawer shows: it publishes its head the way the real one does. */
function FakeSessionView({ title }: { title: string }) {
  useTopbarSlot({ crumb: title });
  return <textarea aria-label="Composer" />;
}

function renderBoard(seed?: (client: QueryClient) => void) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  seed?.(client);
  const windowSlot = createTopbarSlotStore();
  const ui = () => (
    <QueryClientProvider client={client}>
      <TopbarSlotProvider store={windowSlot}>
        <SessionBoardWindow session={<FakeSessionView title="Session w2" />} windowId={WINDOW_ID} />
      </TopbarSlotProvider>
    </QueryClientProvider>
  );
  const view = render(ui());
  return { ...view, windowSlot, rerenderBoard: () => view.rerender(ui()) };
}

/** Applies the route the last gesture asked for, as the window manager would. */
function applyLastRetarget() {
  const [, target] = harness.userRetarget.mock.calls.at(-1)!;
  harness.desktop.windows[WINDOW_ID] = {
    id: WINDOW_ID,
    app: "session",
    instanceKey: target.instanceKey,
    route: target.route,
  };
}

const drawerRoute = (id: string) => ({
  app: "session",
  instanceKey: id,
  route: { pathname: `/agents/claude/sessions/${id}`, search: { drawer: "open" } },
});
const closedRoute = (id: string) => ({
  app: "session",
  instanceKey: id,
  route: { pathname: `/agents/claude/sessions/${id}`, search: {} },
});

beforeEach(() => {
  harness.userRetarget.mockReset();
  harness.onArchive.mockReset();
  harness.onArchiveMany.mockReset();
  harness.onDeleteMany.mockReset();
  harness.setView.mockReset();
  harness.board = sessionsBoardViewFixture(
    {
      needs_you: empty,
      working: { sessions: [card("w1", "working"), card("w2", "working"), card("w3", "working")] },
      waiting: empty,
      in_review: empty,
      done: { sessions: [card("d1", "done")], count: 1, hasMore: false },
    },
    { sessions: [], count: 0, hasMore: false }
  );
  // A wide board: the drawer sits beside it rather than covering it.
  vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockReturnValue(1600);
  vi.spyOn(HTMLElement.prototype, "clientHeight", "get").mockReturnValue(900);
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => new Response(null, { status: 404 }))
  );
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  resetSessionBoardColumnFolds();
  sessionStore.trigger.composerDraftChanged({ sessionId: "w2", text: "" });
});

describe("SessionBoardWindow drawer", () => {
  it("Should open a card's session in the drawer beside the live board", async () => {
    setRoute(null, false);
    const { rerenderBoard, windowSlot } = renderBoard();
    expect(screen.queryByRole("dialog")).toBeNull();

    await userEvent.click(screen.getByTestId("session-board-card-w2"));
    expect(harness.userRetarget).toHaveBeenCalledExactlyOnceWith(WINDOW_ID, drawerRoute("w2"));

    applyLastRetarget();
    rerenderBoard();
    const drawer = await screen.findByRole("dialog", { name: "Session w2" });
    expect(drawer).toHaveAttribute("data-mode", "inline");
    expect(within(drawer).getByLabelText("Composer")).toBeVisible();
    expect(screen.getByTestId("sessions-board")).toBeVisible();
    expect(screen.getByTestId("session-board-card-w2")).toHaveAttribute("aria-current", "true");
    // The session's head renders in the drawer; the window keeps the board's.
    expect(within(drawer).getByTestId("session-board-drawer-head")).toHaveTextContent("Session w2");
    expect(windowSlot.getSnapshot()?.crumb).toBe("Sessions");
  });

  // Invariant: a rolled-up linked-work item names only its child session; Open session from a
  // card's menu reads the child's agent and opens it in this drawer, not another window.
  it("Should open a rolled-up item's child session in the drawer from a card's menu", async () => {
    const lead = SESSION_LINK_SETS.rolledUp[0]!;
    const childId = lead.source_session_id!;
    const parent = {
      ...card("w2", "working"),
      linked_work: { lead, total: 1, needs_you: 0, pull_requests: 1 },
    };
    harness.board = sessionsBoardViewFixture(
      {
        needs_you: empty,
        working: { sessions: [card("w1", "working"), parent], count: 2, hasMore: false },
        waiting: empty,
        in_review: empty,
        done: empty,
      },
      empty
    );
    setRoute(null, false);
    renderBoard(client =>
      client.setQueryData(
        sessionDetailOptions(workspaceSelectorForScope(ownerDataScope("ws-1")), childId).queryKey,
        { ...card(childId, "working"), agent_name: "codex" }
      )
    );
    const user = userEvent.setup({ pointerEventsCheck: 0 });

    await user.click(screen.getByTestId("session-row-actions-w2"));
    await user.click(
      await screen.findByRole("menuitem", { name: `Pull request #${lead.pull_request!.number}` })
    );
    await user.click(await screen.findByRole("menuitem", { name: "Open session" }));

    await waitFor(() =>
      expect(harness.userRetarget).toHaveBeenCalledExactlyOnceWith(WINDOW_ID, {
        app: "session",
        instanceKey: childId,
        route: { pathname: `/agents/codex/sessions/${childId}`, search: { drawer: "open" } },
      })
    );
  });

  // Invariant: the latest open wins; a slower child lookup never replaces a later choice.
  it("Should keep the later child when an earlier child's lookup finishes last", async () => {
    const [slowLead, , fastLead] = SESSION_LINK_SETS.rolledUp;
    const slowId = slowLead!.source_session_id!;
    const fastId = fastLead!.source_session_id!;
    harness.board = sessionsBoardViewFixture(
      {
        needs_you: empty,
        working: {
          sessions: [
            {
              ...card("w1", "working"),
              linked_work: { lead: slowLead!, total: 1, needs_you: 0, pull_requests: 1 },
            },
            {
              ...card("w2", "working"),
              linked_work: { lead: fastLead!, total: 1, needs_you: 0, pull_requests: 1 },
            },
          ],
          count: 2,
          hasMore: false,
        },
        waiting: empty,
        in_review: empty,
        done: empty,
      },
      empty
    );
    let release: () => void = () => undefined;
    vi.stubGlobal(
      "fetch",
      vi.fn(
        (request: Request) =>
          new Promise<Response>(resolve => {
            if (!String(request.url ?? request).includes(slowId)) {
              resolve(new Response(null, { status: 404 }));
              return;
            }
            release = () =>
              resolve(
                Response.json({ session: { ...card(slowId, "working"), agent_name: "slow" } })
              );
          })
      )
    );
    setRoute(null, false);
    renderBoard(client =>
      client.setQueryData(
        sessionDetailOptions(workspaceSelectorForScope(ownerDataScope("ws-1")), fastId).queryKey,
        { ...card(fastId, "working"), agent_name: "codex" }
      )
    );
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    // Submenus are driven by keys, as the session list suite does: a pointer click on a submenu
    // item can land before jsdom settles its pointer guard.
    const openChild = async (cardId: string, number: number) => {
      await user.click(screen.getByTestId(`session-row-actions-${cardId}`));
      const trigger = await screen.findByRole("menuitem", { name: `Pull request #${number}` });
      trigger.focus();
      await user.keyboard("{ArrowRight}");
      const submenu = await waitFor(() => {
        const menu = document.activeElement?.closest<HTMLElement>('[role="menu"]');
        expect(menu).toBeTruthy();
        expect(menu).not.toContainElement(trigger);
        return menu!;
      });
      within(submenu).getByRole("menuitem", { name: "Open session" }).focus();
      await user.keyboard("{Enter}");
      // The next card's menu must not find this closing menu's Open session.
      await waitFor(() => expect(screen.queryByRole("menu")).toBeNull());
    };

    await openChild("w1", slowLead!.pull_request!.number);
    await openChild("w2", fastLead!.pull_request!.number);
    await waitFor(() => expect(harness.userRetarget).toHaveBeenCalledTimes(1));
    await act(async () => release());

    expect(harness.userRetarget).toHaveBeenCalledExactlyOnceWith(WINDOW_ID, {
      app: "session",
      instanceKey: fastId,
      route: { pathname: `/agents/codex/sessions/${fastId}`, search: { drawer: "open" } },
    });
  });

  it("Should close on Escape through the route and return focus to the session's card", async () => {
    setRoute("w2", true);
    const { rerenderBoard } = renderBoard();
    const drawer = await screen.findByRole("dialog", { name: "Session w2" });
    within(drawer).getByLabelText("Composer").focus();

    await userEvent.keyboard("{Escape}");
    expect(harness.userRetarget).toHaveBeenCalledExactlyOnceWith(WINDOW_ID, closedRoute("w2"));

    applyLastRetarget();
    rerenderBoard();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await waitFor(() => expect(screen.getByTestId("session-board-card-w2")).toHaveFocus());
  });

  it("Should close on an empty-board click only while the composer holds no unsent input", async () => {
    setRoute("w2", true);
    renderBoard();
    await screen.findByRole("dialog", { name: "Session w2" });
    const background = screen.getByText("No open pull requests");
    const click = () => {
      fireEvent.pointerDown(background, { button: 0, clientX: 10, clientY: 10 });
      fireEvent.pointerUp(background, { button: 0, clientX: 10, clientY: 10 });
    };

    act(() => sessionStore.trigger.composerDraftChanged({ sessionId: "w2", text: "half done" }));
    click();
    expect(harness.userRetarget).not.toHaveBeenCalled();
    expect(screen.getByTestId("session-board-card-draft-w2")).toHaveTextContent("Draft");

    // A drag that ends on the board is not a click.
    act(() => sessionStore.trigger.composerDraftChanged({ sessionId: "w2", text: "" }));
    fireEvent.pointerDown(background, { button: 0, clientX: 10, clientY: 10 });
    fireEvent.pointerUp(background, { button: 0, clientX: 80, clientY: 10 });
    expect(harness.userRetarget).not.toHaveBeenCalled();

    click();
    expect(harness.userRetarget).toHaveBeenCalledExactlyOnceWith(WINDOW_ID, closedRoute("w2"));
  });

  it("Should step through the cards in board order from the head and the session shortcut", async () => {
    setRoute("w2", true);
    const { rerenderBoard } = renderBoard();
    const drawer = await screen.findByRole("dialog", { name: "Session w2" });
    // Each step lands like the window manager would before the next one reads the drawer.
    const land = () => {
      applyLastRetarget();
      rerenderBoard();
    };

    await userEvent.click(within(drawer).getByRole("button", { name: "Next session" }));
    expect(harness.userRetarget).toHaveBeenLastCalledWith(WINDOW_ID, drawerRoute("w3"));
    land();
    await userEvent.click(within(drawer).getByRole("button", { name: "Previous session" }));
    expect(harness.userRetarget).toHaveBeenLastCalledWith(WINDOW_ID, drawerRoute("w2"));
    land();
    harness.userRetarget.mockClear();

    // The shortcut works from the drawer's controls, never from inside text entry.
    within(drawer).getByRole("button", { name: "Next session" }).focus();
    await userEvent.keyboard("{Control>}{Shift>}{ArrowDown}{/Shift}{/Control}");
    expect(harness.userRetarget).toHaveBeenCalledExactlyOnceWith(WINDOW_ID, drawerRoute("w3"));
    land();
    within(drawer).getByLabelText("Composer").focus();
    await userEvent.keyboard("{Control>}{Shift>}{ArrowDown}{/Shift}{/Control}");
    expect(harness.userRetarget).toHaveBeenCalledTimes(1);
  });

  it("Should skip the cards of a folded column when stepping", async () => {
    setRoute("w3", true);
    renderBoard();
    const drawer = await screen.findByRole("dialog", { name: "Session w2" });
    await userEvent.click(within(drawer).getByRole("button", { name: "Next session" }));
    expect(harness.userRetarget).toHaveBeenLastCalledWith(WINDOW_ID, drawerRoute("d1"));

    await userEvent.click(screen.getByRole("button", { name: "Collapse Done" }));
    await userEvent.click(within(drawer).getByRole("button", { name: "Next session" }));
    expect(harness.userRetarget).toHaveBeenLastCalledWith(WINDOW_ID, drawerRoute("w1"));
  });

  it("Should step from the board's ends and return focus to the fold when the session is out of sight", async () => {
    setRoute("d1", true);
    const { rerenderBoard } = renderBoard();
    const drawer = await screen.findByRole("dialog", { name: "Session w2" });
    await userEvent.click(screen.getByRole("button", { name: "Collapse Done" }));

    await userEvent.click(within(drawer).getByRole("button", { name: "Next session" }));
    expect(harness.userRetarget).toHaveBeenLastCalledWith(WINDOW_ID, drawerRoute("w1"));
    await userEvent.click(within(drawer).getByRole("button", { name: "Previous session" }));
    expect(harness.userRetarget).toHaveBeenLastCalledWith(WINDOW_ID, drawerRoute("w3"));

    // Once the drawer lands on a visible card, stepping continues from that card.
    setRoute("w1", true);
    rerenderBoard();
    await userEvent.click(within(drawer).getByRole("button", { name: "Next session" }));
    expect(harness.userRetarget).toHaveBeenLastCalledWith(WINDOW_ID, drawerRoute("w2"));

    setRoute("d1", true);
    rerenderBoard();
    within(drawer).getByLabelText("Composer").focus();
    await userEvent.keyboard("{Escape}");
    harness.desktop.windows[WINDOW_ID] = {
      id: WINDOW_ID,
      app: "session",
      instanceKey: "d1",
      route: { pathname: "/agents/claude/sessions/d1", search: {} },
    };
    rerenderBoard();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await waitFor(() => expect(screen.getByRole("button", { name: "Expand Done" })).toHaveFocus());
  });

  it("Should close when its session is archived and land focus on the next card in the column", async () => {
    setRoute("w2", true);
    const { rerenderBoard } = renderBoard();
    await screen.findByRole("dialog", { name: "Session w2" });

    const user = userEvent.setup();
    await user.pointer({
      keys: "[MouseRight]",
      target: screen.getByTestId("session-board-card-w2"),
    });
    const menu = await screen.findByRole("menu");
    await user.click(within(menu).getByRole("menuitem", { name: "Stop and archive session" }));
    await waitFor(() =>
      expect(harness.onArchive).toHaveBeenCalledWith(expect.objectContaining({ id: "w2" }))
    );

    harness.userRetarget.mockClear();
    act(() => harness.lifecycleOptions.onRemoved?.({ id: "w2" }, "archive"));
    expect(harness.userRetarget).not.toHaveBeenCalled();
    setRoute(null, false);
    rerenderBoard();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await waitFor(() => expect(screen.getByTestId("session-board-card-w3")).toHaveFocus());
  });

  it("Should take the drawer along when the window leaves the board for List", async () => {
    setRoute("w2", true);
    renderBoard();
    const drawer = await screen.findByRole("dialog", { name: "Session w2" });

    await userEvent.click(within(drawer).getByRole("button", { name: "Open full session" }));
    expect(harness.userRetarget).toHaveBeenCalledExactlyOnceWith(WINDOW_ID, closedRoute("w2"));
    expect(harness.setView).toHaveBeenCalledWith("list");
  });

  it("Should land focus on a visible card when the archived session sat in a folded column", async () => {
    setRoute("d1", true);
    const { rerenderBoard } = renderBoard();
    await screen.findByRole("dialog", { name: "Session w2" });
    await userEvent.click(screen.getByRole("button", { name: "Collapse Done" }));

    harness.userRetarget.mockClear();
    act(() => harness.lifecycleOptions.onRemoved?.({ id: "d1" }, "archive"));
    expect(harness.userRetarget).not.toHaveBeenCalled();
    setRoute(null, false);
    rerenderBoard();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await waitFor(() => expect(screen.getByTestId("session-board-card-w3")).toHaveFocus());
  });

  it("Should close the drawer itself when its session moves to Later", async () => {
    setRoute("w2", true);
    const { rerenderBoard } = renderBoard();
    await screen.findByRole("dialog", { name: "Session w2" });

    act(() => harness.lifecycleOptions.onRemoved?.({ id: "w2" }, "later"));
    expect(harness.userRetarget).toHaveBeenLastCalledWith(WINDOW_ID, closedRoute("w2"));
    applyLastRetarget();
    rerenderBoard();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await waitFor(() => expect(screen.getByTestId("session-board-card-w3")).toHaveFocus());
  });

  it("Should land focus below a deleted session once its window retires the drawer", async () => {
    setRoute("w2", true);
    const { rerenderBoard } = renderBoard();
    await screen.findByRole("dialog", { name: "Session w2" });
    const user = userEvent.setup();
    await user.pointer({
      keys: "[MouseRight]",
      target: screen.getByTestId("session-board-card-w2"),
    });
    await user.click(
      within(await screen.findByRole("menu")).getByRole("menuitem", { name: "Delete session" })
    );

    // The delete retires the window's session (closing the drawer) before it confirms.
    setRoute(null, false);
    rerenderBoard();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    act(() => harness.lifecycleOptions.onRemoved?.({ id: "w2" }, "delete"));
    await waitFor(() => expect(screen.getByTestId("session-board-card-w3")).toHaveFocus());
  });

  it("Should open the drawer from a plain click only, never from a selection gesture", async () => {
    setRoute(null, false);
    renderBoard();
    const user = userEvent.setup();

    await user.keyboard("{Control>}");
    await user.click(screen.getByTestId("session-board-card-w1"));
    await user.keyboard("{/Control}");
    await user.keyboard("{Shift>}");
    await user.click(screen.getByTestId("session-board-card-w3"));
    await user.keyboard("{/Shift}");
    // An open selection turns a plain click into a toggle as well.
    await user.click(screen.getByTestId("session-board-card-d1"));
    expect(screen.getByTestId("session-board-card-d1")).toHaveAttribute("aria-pressed", "true");
    expect(harness.userRetarget).not.toHaveBeenCalled();

    await user.keyboard("{Escape}");
    await user.click(screen.getByTestId("session-board-card-w2"));
    expect(harness.userRetarget).toHaveBeenCalledExactlyOnceWith(WINDOW_ID, drawerRoute("w2"));
  });

  it("Should close on a bulk archive holding its session and land focus past the batch", async () => {
    setRoute("w2", true);
    const { rerenderBoard } = renderBoard();
    await screen.findByRole("dialog", { name: "Session w2" });
    const user = userEvent.setup();
    await user.click(screen.getByTestId("session-board-card-select-w2"));
    await user.click(screen.getByTestId("session-board-card-w3"));

    await user.pointer({
      keys: "[MouseRight]",
      target: screen.getByTestId("session-board-card-w3"),
    });
    await user.click(await screen.findByTestId("sessions-board-selection-menu-archive"));
    expect(harness.onArchiveMany).toHaveBeenCalledExactlyOnceWith([
      expect.objectContaining({ id: "w2" }),
      expect.objectContaining({ id: "w3" }),
    ]);

    harness.userRetarget.mockClear();
    act(() => harness.lifecycleOptions.onRemoved?.({ id: "w2" }, "archive"));
    expect(harness.userRetarget).not.toHaveBeenCalled();
    setRoute(null, false);
    rerenderBoard();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    // w3 leaves with it, so focus lands on the card above.
    await waitFor(() => expect(screen.getByTestId("session-board-card-w1")).toHaveFocus());
  });

  it("Should land focus past the batch once a bulk delete retires the drawer", async () => {
    setRoute("w2", true);
    const { rerenderBoard } = renderBoard();
    await screen.findByRole("dialog", { name: "Session w2" });
    const user = userEvent.setup();
    await user.click(screen.getByTestId("session-board-card-select-w2"));
    await user.click(screen.getByTestId("session-board-card-w3"));

    await user.pointer({
      keys: "[MouseRight]",
      target: screen.getByTestId("session-board-card-w2"),
    });
    await user.click(await screen.findByTestId("sessions-board-selection-menu-delete-item"));
    expect(harness.onDeleteMany).toHaveBeenCalledExactlyOnceWith(
      [expect.objectContaining({ id: "w2" }), expect.objectContaining({ id: "w3" })],
      expect.any(Function)
    );

    // The delete retires the window's session (closing the drawer) before it confirms.
    setRoute(null, false);
    rerenderBoard();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    act(() => harness.lifecycleOptions.onRemoved?.({ id: "w2" }, "delete"));
    await waitFor(() => expect(screen.getByTestId("session-board-card-w1")).toHaveFocus());
  });

  it("Should land on a batch member the archive skipped when no other card remains", async () => {
    harness.board = sessionsBoardViewFixture(
      {
        needs_you: empty,
        working: { sessions: [card("w2", "working"), card("w3", "working")] },
        waiting: empty,
        in_review: empty,
        done: empty,
      },
      { sessions: [], count: 0, hasMore: false }
    );
    setRoute("w2", true);
    const { rerenderBoard } = renderBoard();
    await screen.findByRole("dialog", { name: "Session w2" });
    const user = userEvent.setup();
    await user.click(screen.getByTestId("session-board-card-select-w2"));
    await user.click(screen.getByTestId("session-board-card-w3"));
    await user.pointer({
      keys: "[MouseRight]",
      target: screen.getByTestId("session-board-card-w2"),
    });
    await user.click(await screen.findByTestId("sessions-board-selection-menu-archive"));

    // w3 was skipped (or failed) and stays on the board: it is still the best landing.
    act(() => harness.lifecycleOptions.onRemoved?.({ id: "w2" }, "archive"));
    setRoute(null, false);
    rerenderBoard();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await waitFor(() => expect(screen.getByTestId("session-board-card-w3")).toHaveFocus());
  });
});
