// Suite: sessions-board
// Invariant: the board renders the daemon's five columns as labelled regions
// whose names carry their facet counts; a card names its parent ("↳") or its
// child rollup, flags a child that needs you, and opens its session on click;
// any column folds into a rail (mark, vertical name, count) that expands it
// again, one fold per column across every swimlane (#1075); only an initial read failure replaces
// a column, while a failed next page or background re-read keeps the loaded
// cards with a local retry; the Archive bar is absent only after a read
// confirms zero archived sessions — a pending or failed read keeps the bar with
// its state and a retry — and lists archived cards once expanded. The Later bar
// (#1076) is the same bar over the parked sessions: absent at zero, labelled
// Later with its count, and its cards say why they wait.
// Owning layer: component (systems/session/components). Card mapping rules live
// in lib/__tests__/session-board-card.test.ts; queries and toggle in E2E-005..007.
// #952: with the session list's lifecycle verbs, every card offers them on
// right-click and behind a `…` button that never opens the session; a grouped
// board renders one swimlane per group, each reading the five columns through
// the group's own scope.
// #958: a board or column that has not answered yet holds its place with
// card-shaped placeholders under one busy status region — never a "Loading…"
// sentence, never an empty-state claim — and loaded cards replace them. The
// standalone board skeleton needs no query client.
// #1053: inside `SessionsBoardSelection` the board reuses the list's bulk selection:
// Cmd/Ctrl-click toggles, Shift-click ranges within a column, an open selection turns
// every click into a toggle, column and lane checkboxes select what they show, Esc
// clears; the selection bar replaces the head's actions with the list's verbs and
// counts; a selected card's context menu acts on the selection, and a context click
// outside it retargets; selected cards survive a column move and drop out once gone;
// a folded column keeps them as hidden; the Archive bar's cards join only while it is open;
// a toggle re-renders only the cards it changes.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { workspaceBadgeIdentity, workspacesListOptions } from "@/systems/workspace";

import { fetchSessionCatalogPage, fetchSessionFacets } from "../../adapters/session-catalog-api";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import type { SessionsBoardColumnView, SessionsBoardView } from "../../hooks/use-sessions-board";
import type { SessionBoardColumnId } from "../../lib/session-board-contract";
import { primarySessionFixture, resetSessionBoardColumnFolds } from "../../testing";
import type { SessionPayload } from "../../types";
import { renderWithTopbar } from "@/test/render-with-topbar";

import { useSessionRowRuntimeLabel } from "../../hooks/use-session-row-runtime-label";
import { SessionsBoard } from "../session-board/sessions-board";
import { SessionsBoardSelection } from "../session-board/sessions-board-selection";
import { SessionsBoardSkeleton } from "../session-board/sessions-board-skeleton";

vi.mock("../../hooks/use-session-row-runtime-label", async importOriginal => {
  const actual = await importOriginal<typeof import("../../hooks/use-session-row-runtime-label")>();
  // Wrapped so a test can count how often each card renders.
  return { useSessionRowRuntimeLabel: vi.fn(actual.useSessionRowRuntimeLabel) };
});

vi.mock("../../adapters/session-catalog-api", () => ({
  fetchSessionCatalogPage: vi.fn(),
  fetchSessionFacets: vi.fn(),
}));

const LABELS: Record<SessionBoardColumnId, string> = {
  needs_you: "Needs you",
  working: "Working",
  waiting: "Waiting",
  in_review: "In review",
  done: "Done",
};

function card(id: string, overrides: Partial<SessionPayload> = {}): SessionPayload {
  return {
    ...primarySessionFixture,
    id,
    name: `Session ${id}`,
    badge: "running",
    board_column: "working",
    pull_request: null,
    pull_request_lookup: null,
    ...overrides,
  };
}

function page(sessions: SessionPayload[] = [], overrides: Partial<SessionsBoardColumnView> = {}) {
  return {
    sessions,
    loading: false,
    failure: null,
    retrying: false,
    fetching: false,
    hasMore: false,
    loadingMore: false,
    loadMore: vi.fn(),
    retry: vi.fn(),
    total: undefined,
    ...overrides,
  };
}

function board(
  columns: Partial<Record<SessionBoardColumnId, Partial<SessionsBoardColumnView>>> = {},
  archive: Partial<SessionsBoardView["archive"]> = {},
  later: Partial<SessionsBoardView["later"]> = {}
): SessionsBoardView {
  return {
    columns: (Object.keys(LABELS) as SessionBoardColumnId[]).map(id => ({
      ...page(),
      id,
      label: LABELS[id],
      count: 0,
      ...columns[id],
    })),
    lanes: null,
    lanesLoading: false,
    groupBy: "none",
    groupByOptions: ["none"],
    archive: { ...page(), count: 0, ...archive },
    later: { ...page(), count: 0, ...later },
    workspaceNameOf: () => null,
    selectionKey: "board",
  };
}

function boardWrapper() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
}

function renderBoard(
  view: SessionsBoardView,
  onOpen = vi.fn(),
  actions?: SessionLifecycleActionHandlers
) {
  render(<SessionsBoard actions={actions} board={view} onOpen={onOpen} />, {
    wrapper: boardWrapper(),
  });
  return onOpen;
}

function lifecycleActions(): SessionLifecycleActionHandlers {
  return {
    pendingAction: null,
    pendingSessionId: null,
    onStop: vi.fn(),
    onRename: vi.fn(),
    onArchive: vi.fn(),
    onUnarchive: vi.fn(),
    onDelete: vi.fn(),
  };
}

afterEach(() => resetSessionBoardColumnFolds());

describe("SessionsBoard", () => {
  it("Should name each column region with its facet count", () => {
    renderBoard(board({ working: { count: 9 }, done: { count: 1 } }));
    expect(screen.getByRole("region", { name: "Working, 9 sessions" })).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Done, 1 session" })).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Needs you, 0 sessions" })).toHaveTextContent(
      "Nothing needs you"
    );
  });

  it("UT-070: Should show the parent link on a child and a highlighted rollup on its parent", () => {
    const parent = card("parent", {
      name: "Coordinator",
      children: { total: 3, needs_you: 1, working: 2, waiting: 0, in_review: 0, done: 0 },
    });
    const child = card("child", {
      board_column: "needs_you",
      badge: "waiting-for-input",
      parent_title: "  Coordinator  ",
    });
    renderBoard(
      board({
        needs_you: { sessions: [child], count: 1 },
        working: { sessions: [parent], count: 1 },
      })
    );
    expect(screen.getByTestId("session-board-card-parent-child")).toHaveTextContent("Coordinator");
    expect(
      screen.getByRole("button", { name: /Session child, Input, Child of Coordinator,/ })
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Coordinator,.*3 child sessions, A child needs you,/ })
    ).toBeInTheDocument();
    const rollup = screen.getByTestId("session-board-card-children-parent");
    expect(rollup).toHaveTextContent("3");
    expect(rollup).toHaveAttribute("data-needs-you", "true");
    expect(screen.getByTestId("session-board-card-status-child")).toHaveTextContent("Input");
  });

  it("Should name a fork's source as provenance, not as its parent", () => {
    const fork = card("fork", {
      parent_title: "Coordinator",
      lineage: { ...card("fork").lineage!, parent_session_id: "parent", kind: "fork" },
    });
    renderBoard(board({ working: { sessions: [fork], count: 1 } }));
    const origin = screen.getByTestId("session-board-card-parent-fork");
    expect(origin).toHaveTextContent("Coordinator");
    expect(origin).toHaveAttribute("data-lineage", "fork");
    const named = screen.getByRole("button", { name: /Session fork,.*Forked from Coordinator,/ });
    expect(named.getAttribute("aria-label")).not.toContain("Child of");
  });

  it("Should name one child without an attention signal and omit absent family facts", () => {
    renderBoard(
      board({
        working: {
          sessions: [
            card("one", {
              children: { total: 1, needs_you: 0, working: 1, waiting: 0, in_review: 0, done: 0 },
            }),
            card("none", { parent_title: "  ", children: null }),
          ],
        },
      })
    );
    const one = screen.getByRole("button", { name: /Session one,.*1 child session,/ });
    expect(one).not.toHaveAccessibleName(/A child needs you/);
    expect(screen.getByTestId("session-board-card-none")).not.toHaveAccessibleName(
      /Child of|child session|child needs you/
    );
  });

  it("Should open the clicked session and offer the next page with its progress", async () => {
    const user = userEvent.setup();
    const loadMore = vi.fn();
    const done = card("done-1", { board_column: "done", badge: "idle" });
    const onOpen = renderBoard(
      board({ done: { sessions: [done], count: 60, hasMore: true, loadMore } })
    );
    await user.click(screen.getByTestId("session-board-card-done-1"));
    expect(onOpen).toHaveBeenCalledWith(done);
    const more = screen.getByTestId("sessions-board-load-more-done");
    expect(more).toHaveTextContent("Load more1 of 60");
    await user.click(more);
    expect(loadMore).toHaveBeenCalledOnce();
  });

  it("Should replace a column with its error only when nothing has loaded", async () => {
    const user = userEvent.setup();
    const retry = vi.fn();
    renderBoard(board({ in_review: { failure: "initial", retry } }));
    const error = screen.getByTestId("sessions-board-column-error-in_review");
    expect(error).toHaveTextContent("Couldn't load in review sessions.");
    await user.click(within(error).getByRole("button", { name: "Retry" }));
    expect(retry).toHaveBeenCalledOnce();
  });

  it("Should keep loaded cards when the next page fails and retry it in place", async () => {
    const user = userEvent.setup();
    const retry = vi.fn();
    const loaded = [card("d1", { board_column: "done" }), card("d2", { board_column: "done" })];
    renderBoard(
      board({ done: { sessions: loaded, count: 300, hasMore: true, failure: "more", retry } })
    );
    expect(screen.queryByTestId("sessions-board-column-error-done")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-board-card-d1")).toBeInTheDocument();
    expect(screen.getByTestId("session-board-card-d2")).toBeInTheDocument();
    expect(screen.queryByTestId("sessions-board-load-more-done")).not.toBeInTheDocument();
    const failure = screen.getByTestId("sessions-board-read-error-done");
    expect(failure).toHaveRole("alert");
    expect(failure).toHaveTextContent("Couldn't load more sessions.");
    await user.click(within(failure).getByRole("button", { name: "Retry" }));
    expect(retry).toHaveBeenCalledOnce();
  });

  it("Should keep loaded cards when a background re-read fails", () => {
    renderBoard(
      board({
        working: {
          sessions: [card("w1")],
          count: 1,
          failure: "refresh",
          retrying: true,
        },
      })
    );
    expect(screen.getByTestId("session-board-card-w1")).toBeInTheDocument();
    const failure = screen.getByTestId("sessions-board-read-error-working");
    expect(failure).toHaveTextContent("Couldn't refresh. Showing the last loaded sessions.");
    expect(within(failure).getByRole("button", { name: "Retry" })).toBeDisabled();
  });

  it("Should fold any column into a rail that names it and counts its sessions", async () => {
    const user = userEvent.setup();
    renderBoard(
      board({
        waiting: {
          sessions: [card("wt", { board_column: "waiting", badge: "waiting" })],
          count: 1,
        },
        done: { sessions: [card("d", { board_column: "done" })], count: 1 },
      })
    );
    for (const label of Object.values(LABELS)) {
      expect(screen.getByRole("button", { name: `Collapse ${label}` })).toHaveAttribute(
        "aria-expanded",
        "true"
      );
    }
    await user.click(screen.getByRole("button", { name: "Collapse Waiting" }));
    const rail = screen.getByRole("button", { name: "Expand Waiting" });
    expect(rail).toHaveAttribute("aria-expanded", "false");
    expect(rail).toHaveTextContent("Waiting1");
    expect(screen.getByTestId("sessions-board-column-waiting")).toHaveAttribute(
      "data-collapsed",
      "true"
    );
    // Done keeps its own state: one fold never folds a neighbour.
    expect(screen.getByTestId("sessions-board-column-done")).not.toHaveAttribute("data-collapsed");

    rail.focus();
    await user.keyboard("{Enter}");
    expect(screen.getByRole("button", { name: "Collapse Waiting" })).toHaveFocus();
    expect(screen.getByTestId("sessions-board-column-waiting")).not.toHaveAttribute(
      "data-collapsed"
    );
  });

  it("Should show the Waiting column's empty state", () => {
    renderBoard(board());
    expect(screen.getByTestId("sessions-board-column-empty-waiting")).toHaveTextContent(
      "Nothing waiting on a wake"
    );
  });

  it("UT-071: Should hide the Archive bar with no archived sessions", () => {
    renderBoard(board());
    expect(screen.queryByTestId("sessions-board-archive")).not.toBeInTheDocument();
  });

  it("Should keep the Archive bar while its first read is pending", () => {
    renderBoard(board({}, { count: undefined, loading: true }));
    const status = screen.getByTestId("sessions-board-archive-status");
    expect(status).toHaveTextContent("Archive");
    // The count's place is a text bar under one busy region, not a spinner.
    const loading = within(status).getByRole("status");
    expect(loading).toHaveTextContent("Loading archived sessions");
    expect(loading).toHaveAttribute("aria-busy", "true");
    expect(loading.querySelector('[data-slot="skeleton-text"]')).not.toBeNull();
  });

  it("Should keep the Archive bar with a retry when its first read fails", async () => {
    const user = userEvent.setup();
    const retry = vi.fn();
    renderBoard(board({}, { count: undefined, failure: "initial", retry }));
    const status = screen.getByTestId("sessions-board-archive-status");
    expect(status).toHaveRole("alert");
    expect(status).toHaveTextContent("Couldn't load archived sessions.");
    await user.click(within(status).getByRole("button", { name: "Retry" }));
    expect(retry).toHaveBeenCalledOnce();
  });

  it("Should keep archived cards when the next archive page fails", async () => {
    const user = userEvent.setup();
    const archived = card("a1", { archived_at: "2026-10-21T10:00:00Z", board_column: "done" });
    renderBoard(board({}, { sessions: [archived], count: 40, hasMore: true, failure: "more" }));
    await user.click(screen.getByTestId("sessions-board-archive-toggle"));
    const body = screen.getByTestId("sessions-board-archive-body");
    expect(within(body).getByTestId("session-board-card-a1")).toBeInTheDocument();
    expect(within(body).getByTestId("sessions-board-read-error-archive")).toHaveTextContent(
      "Couldn't load more sessions."
    );
    expect(screen.queryByTestId("sessions-board-load-more-archive")).not.toBeInTheDocument();
  });

  it("Should list archived cards once the Archive bar is expanded", async () => {
    const user = userEvent.setup();
    const archived = [1, 2, 3].map(index =>
      card(`a${index}`, { archived_at: "2026-10-21T10:00:00Z", board_column: "done" })
    );
    renderBoard(board({}, { sessions: archived, count: 3 }));
    const toggle = screen.getByTestId("sessions-board-archive-toggle");
    expect(toggle).toHaveTextContent("Archive3");
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    await user.click(toggle);
    const body = screen.getByTestId("sessions-board-archive-body");
    expect(within(body).getAllByRole("button")).toHaveLength(3);
  });

  it("Should hide the Later bar until a read finds parked sessions", () => {
    renderBoard(board({}, {}, { count: undefined, failure: "initial" }));
    expect(screen.getByTestId("sessions-board-later-status")).toHaveTextContent(
      "Couldn't load sessions in Later."
    );
    cleanup();
    renderBoard(board());
    expect(screen.queryByTestId("sessions-board-later")).not.toBeInTheDocument();
  });

  it("Should list parked cards with their reason and waiting messages in the Later bar", async () => {
    const user = userEvent.setup();
    const parked = card("l1", {
      board_column: "done",
      state: "stopped",
      later: {
        parked_at: "2026-10-30T09:00:00Z",
        reason: "Waiting on the vendor",
        return_at: "2026-11-04T09:00:00Z",
        pending_messages: 1,
      },
    });
    renderBoard(board({}, {}, { sessions: [parked], count: 1 }));
    const toggle = screen.getByTestId("sessions-board-later-toggle");
    expect(toggle).toHaveTextContent("Later1");
    expect(screen.queryByTestId("sessions-board-archive")).not.toBeInTheDocument();
    await user.click(toggle);
    const body = screen.getByTestId("sessions-board-later-body");
    const parkedCard = within(body).getByTestId("session-board-card-l1");
    expect(parkedCard).not.toHaveAttribute("data-archived");
    expect(parkedCard).toHaveAccessibleName(
      /In Later, Waiting on the vendor, Back .*, Parked .*, 1 message waiting/
    );
    expect(within(body).getByTestId("session-later-pending-l1")).toHaveAttribute(
      "title",
      "1 message waiting"
    );
  });
});

describe("SessionsBoard card actions", () => {
  it("Should open the session list's verbs from a card's context menu", async () => {
    const user = userEvent.setup();
    const actions = lifecycleActions();
    const working = card("ctx");
    const onOpen = renderBoard(
      board({ working: { sessions: [working], count: 1 } }),
      vi.fn(),
      actions
    );

    await user.pointer({
      keys: "[MouseRight]",
      target: screen.getByTestId("session-board-card-ctx"),
    });
    const menu = await screen.findByRole("menu");
    expect(within(menu).getByRole("menuitem", { name: "Rename session" })).toBeInTheDocument();
    await user.click(within(menu).getByRole("menuitem", { name: "Delete session" }));
    expect(actions.onDelete).toHaveBeenCalledExactlyOnceWith(working);
    expect(onOpen).not.toHaveBeenCalled();
  });

  it("Should offer the same verbs behind the card's `…` without opening the session", async () => {
    const user = userEvent.setup();
    const actions = lifecycleActions();
    const working = card("dots");
    const onOpen = renderBoard(
      board({ working: { sessions: [working], count: 1 } }),
      vi.fn(),
      actions
    );

    const trigger = screen.getByRole("button", { name: "More actions for Session dots" });
    await user.click(trigger);
    await user.click(await screen.findByTestId("session-row-archive-dots"));
    expect(actions.onArchive).toHaveBeenCalledExactlyOnceWith(working);
    expect(onOpen).not.toHaveBeenCalled();
  });

  it("Should reach the card's `…` from the keyboard right after its card", async () => {
    const user = userEvent.setup();
    renderBoard(
      board({ working: { sessions: [card("keys")], count: 1 } }),
      vi.fn(),
      lifecycleActions()
    );
    screen.getByTestId("session-board-card-keys").focus();
    await user.tab();
    expect(screen.getByRole("button", { name: "More actions for Session keys" })).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(await screen.findByRole("menuitem", { name: "Rename session" })).toBeInTheDocument();
  });

  it("Should carry no menus when the host offers no verbs", () => {
    renderBoard(board({ working: { sessions: [card("plain")], count: 1 } }));
    expect(screen.queryByRole("button", { name: /More actions for/ })).not.toBeInTheDocument();
  });
});

describe("SessionsBoard swimlanes", () => {
  const names = new Map([
    ["ws-alpha", "Alpha"],
    ["ws-beta", "Beta"],
  ]);
  const lane = (workspaceId: string, total: number) => ({
    group: {
      key: `project:${workspaceId}`,
      kind: "project" as const,
      badge: workspaceBadgeIdentity(workspaceId, names),
      total,
      working: total,
      needsYou: 0,
      scope: { kind: "project" as const, workspaceId },
      lens: { profile: "default" },
      pair: null,
    },
    filters: { workspace_id: workspaceId, sort: "created" as const },
  });

  it("Should render one lane per group, each reading the five columns in its own scope", async () => {
    vi.mocked(fetchSessionCatalogPage).mockImplementation(async query => ({
      sessions:
        query?.workspace_id === "ws-alpha" && query.board_column?.includes("working")
          ? [card("alpha-1", { workspace_id: "ws-alpha" })]
          : [],
      page: { has_more: false, limit: 50 },
    }));
    vi.mocked(fetchSessionFacets).mockResolvedValue({
      facets: {
        terminal_approvals: 0,
        all: 1,
        needs_you: 0,
        working: 1,
        finished: 0,
        idle: 0,
        board: { needs_you: 0, working: 1, waiting: 0, in_review: 0, done: 0 },
      },
      by_workspace: [],
      by_profile: [],
    });
    renderBoard({
      ...board(),
      columns: [],
      lanes: [lane("ws-alpha", 1), lane("ws-beta", 0)],
      workspaceNameOf: session => names.get(session.workspace_id ?? "") ?? null,
    });

    const alpha = screen.getByRole("region", { name: "Alpha" });
    expect(screen.getByRole("region", { name: "Beta" })).toBeInTheDocument();
    expect(await within(alpha).findByTestId("session-board-card-alpha-1")).toBeInTheDocument();
    // The lane header names the project; its cards do not repeat it.
    expect(within(alpha).getByTestId("session-board-card-alpha-1")).not.toHaveTextContent("Alpha");
    expect(
      within(alpha).getByRole("region", { name: "Alpha, Working, 1 session" })
    ).toBeInTheDocument();
    const reads = vi.mocked(fetchSessionCatalogPage).mock.calls.map(([query]) => query);
    expect(reads.filter(query => query?.workspace_id === "ws-beta")).toHaveLength(5);
    expect(reads.every(query => query?.sort === "created")).toBe(true);

    // One fold per column: folding Working in Alpha folds it in Beta too, so lanes stay aligned.
    await userEvent.click(within(alpha).getByRole("button", { name: "Collapse Working" }));
    const beta = screen.getByRole("region", { name: "Beta" });
    expect(within(beta).getByRole("button", { name: "Expand Working" })).toBeInTheDocument();
    expect(within(alpha).getByRole("button", { name: "Expand Working" })).toHaveFocus();
  });

  // #1029: a project lane header is a project surface: right-click and Shift+F10 open the
  // project's own actions, the same flow every other project surface uses.
  it.each(["{Shift>}{F10}{/Shift}", "{ContextMenu}"])(
    "Should open a project lane's actions by mouse and %s",
    async key => {
      const user = userEvent.setup();
      vi.mocked(fetchSessionCatalogPage).mockResolvedValue({
        sessions: [],
        page: { has_more: false, limit: 50 },
      });
      const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
      client.setQueryData(workspacesListOptions().queryKey, [
        {
          id: "ws-alpha",
          name: "Alpha",
          root_dir: "/projects/alpha",
          add_dirs: [],
          created_at: "2026-10-29T00:00:00Z",
          updated_at: "2026-10-29T00:00:00Z",
        },
      ]);
      render(
        <SessionsBoard
          board={{ ...board(), columns: [], lanes: [lane("ws-alpha", 0)] }}
          onOpen={vi.fn()}
        />,
        {
          wrapper: ({ children }: { children: ReactNode }) => (
            <QueryClientProvider client={client}>{children}</QueryClientProvider>
          ),
        }
      );

      const header = screen.getByTestId("sessions-board-lane-project:ws-alpha-header");
      fireEvent.contextMenu(header);
      expect(await screen.findByRole("menuitem", { name: "Archive project" })).toBeVisible();
      await user.keyboard("{Escape}");
      await waitFor(() =>
        expect(screen.queryByRole("menuitem", { name: "Archive project" })).toBeNull()
      );
      header.focus();
      await user.keyboard(key);
      await user.click(await screen.findByRole("menuitem", { name: "Remove project" }));
      expect(await screen.findByRole("dialog", { name: "Remove Alpha?" })).toBeVisible();
    }
  );
});

describe("SessionsBoard loading", () => {
  const PLACEHOLDER_CARD = '[data-slot="session-board-card-skeleton"]';

  function groupedBoard(overrides: Partial<SessionsBoardView> = {}): SessionsBoardView {
    return {
      ...board(),
      columns: [],
      lanes: [],
      groupBy: "project",
      groupByOptions: ["none", "project"],
      ...overrides,
    };
  }

  it("Should hold a grouped board's place with lane and card placeholders until its lanes land", () => {
    const wrapper = boardWrapper();
    const { rerender } = render(
      <SessionsBoard board={groupedBoard({ lanesLoading: true })} onOpen={vi.fn()} />,
      { wrapper }
    );

    const region = screen.getByRole("status");
    expect(screen.getAllByRole("status")).toHaveLength(1);
    expect(region).toHaveAttribute("aria-busy", "true");
    expect(region).toHaveTextContent("Loading sessions");
    expect(region.querySelectorAll('[data-slot="sessions-board-lane-skeleton"]')).toHaveLength(2);
    expect(region.querySelectorAll('[data-slot="sessions-board-column-skeleton"]')).toHaveLength(
      10
    );
    expect(region.querySelectorAll(PLACEHOLDER_CARD).length).toBeGreaterThan(10);
    // A pending read is not an answer: no sentence, and no claim that there is nothing.
    expect(screen.queryByText("Loading sessions…")).not.toBeInTheDocument();
    expect(screen.queryByTestId("sessions-board-lanes-empty")).not.toBeInTheDocument();

    rerender(<SessionsBoard board={groupedBoard()} onOpen={vi.fn()} />);

    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    expect(screen.getByTestId("sessions-board-lanes-empty")).toHaveTextContent("No sessions yet.");
  });

  it("Should show cards where a column is loading and keep every column that has answered", () => {
    const wrapper = boardWrapper();
    const working = { sessions: [card("w1")], count: 1 };
    const { rerender } = render(
      <SessionsBoard
        board={board({ needs_you: { loading: true, count: undefined }, working })}
        onOpen={vi.fn()}
      />,
      { wrapper }
    );

    const needsYou = screen.getByTestId("sessions-board-column-needs_you");
    const region = within(needsYou).getByRole("status");
    expect(region).toHaveTextContent("Loading sessions");
    expect(region.querySelectorAll(PLACEHOLDER_CARD).length).toBeGreaterThan(0);
    expect(within(needsYou).queryByTestId("sessions-board-column-empty-needs_you")).toBeNull();
    expect(within(needsYou).queryByTestId("sessions-board-column-count-needs_you")).toBeNull();
    // The columns that already answered keep their cards, with no placeholder beside them.
    const loaded = screen.getByTestId("sessions-board-column-working");
    expect(within(loaded).getByTestId("session-board-card-w1")).toBeInTheDocument();
    expect(within(loaded).queryByRole("status")).not.toBeInTheDocument();

    rerender(
      <SessionsBoard
        board={board({ needs_you: { sessions: [], count: 0 }, working })}
        onOpen={vi.fn()}
      />
    );

    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    expect(screen.getByTestId("sessions-board-column-empty-needs_you")).toHaveTextContent(
      "Nothing needs you"
    );
  });

  it("Should render the whole board's skeleton as one busy region without a query client", () => {
    const { rerender } = render(<SessionsBoardSkeleton />);

    const region = screen.getByRole("status");
    expect(region).toHaveAttribute("aria-busy", "true");
    expect(region).toHaveTextContent("Loading sessions");
    // The five columns keep their real names, so only the cards are placeholders; the Archive
    // bar holds its place under them.
    for (const name of [...Object.values(LABELS), "Archive"]) {
      expect(region).toHaveTextContent(name);
    }
    expect(region.querySelectorAll('[data-slot="sessions-board-column-skeleton"]')).toHaveLength(5);
    expect(region.querySelectorAll(PLACEHOLDER_CARD).length).toBeGreaterThan(5);
    expect(region.querySelector("button, a, input, [tabindex]")).toBeNull();

    rerender(<SessionsBoardSkeleton layout="lanes" />);

    expect(screen.getAllByRole("status")).toHaveLength(1);
    expect(
      screen.getByRole("status").querySelectorAll('[data-slot="sessions-board-lane-skeleton"]')
    ).toHaveLength(2);
  });
});

describe("SessionsBoard selection", () => {
  function bulkActions(): SessionLifecycleActionHandlers {
    return {
      ...lifecycleActions(),
      onStopMany: vi.fn(),
      onArchiveMany: vi.fn(),
      onUnarchiveMany: vi.fn(),
      onDeleteMany: vi.fn(),
    };
  }

  function renderSelectable(view: SessionsBoardView, actions = bulkActions()) {
    const onOpen = vi.fn();
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const ui = (next: SessionsBoardView) => (
      <QueryClientProvider client={client}>
        <SessionsBoardSelection
          selectionKey={next.selectionKey}
          paused={next.lanesLoading}
          sessionActions={actions}
          topbar={{ actions: <button type="button">New session</button> }}
        >
          <SessionsBoard actions={actions} board={next} onOpen={onOpen} />
        </SessionsBoardSelection>
      </QueryClientProvider>
    );
    const result = renderWithTopbar(ui(view));
    return { onOpen, actions, rerender: (next: SessionsBoardView) => result.rerender(ui(next)) };
  }

  const cardOf = (id: string) => screen.getByTestId(`session-board-card-${id}`);
  const selectedCount = () => screen.getByTestId("sessions-board-selection-count");

  it("Should toggle with Cmd/Ctrl, toggle every click while open, and clear on Esc", async () => {
    const user = userEvent.setup();
    const { onOpen } = renderSelectable(
      board({ working: { sessions: [card("a"), card("b")], count: 2 } })
    );

    await user.keyboard("{Meta>}");
    await user.click(cardOf("a"));
    await user.keyboard("{/Meta}");
    // The selection bar takes the head's actions, so the board itself never moves.
    expect(screen.queryByRole("button", { name: "New session" })).not.toBeInTheDocument();
    expect(selectedCount()).toHaveTextContent("1 selected");
    expect(screen.getByTestId("sessions-board-selection-status")).toHaveTextContent(
      "1 session selected"
    );
    expect(cardOf("a")).toHaveAttribute("aria-pressed", "true");
    expect(cardOf("b")).toHaveAttribute("aria-pressed", "false");

    await user.click(cardOf("b"));
    expect(selectedCount()).toHaveTextContent("2 selected");
    await user.click(cardOf("a"));
    expect(selectedCount()).toHaveTextContent("1 selected");
    expect(onOpen).not.toHaveBeenCalled();

    await user.keyboard("{Escape}");
    expect(screen.queryByTestId("sessions-board-selection-bar")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "New session" })).toBeInTheDocument();
    expect(cardOf("b")).not.toHaveAttribute("aria-pressed");
    await user.click(cardOf("b"));
    expect(onOpen).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({ id: "b" }));
  });

  it("Should range with Shift inside the clicked card's column", async () => {
    const user = userEvent.setup();
    renderSelectable(
      board({
        working: { sessions: [card("w1"), card("w2"), card("w3")], count: 3 },
        done: { sessions: [card("d1", { board_column: "done" })], count: 1 },
      })
    );

    await user.click(screen.getByTestId("session-board-card-select-w1"));
    await user.keyboard("{Shift>}");
    await user.click(cardOf("w3"));
    expect(selectedCount()).toHaveTextContent("3 selected");
    // An anchor in another column starts a new range at the clicked card.
    await user.click(cardOf("d1"));
    await user.keyboard("{/Shift}");
    expect(selectedCount()).toHaveTextContent("4 selected");
    expect(cardOf("d1")).toHaveAttribute("aria-pressed", "true");
  });

  it("Should select and clear a column, then a lane, through their header checkboxes", async () => {
    const user = userEvent.setup();
    renderSelectable(
      board({
        working: { sessions: [card("w1"), card("w2")], count: 2 },
        done: { sessions: [card("d1", { board_column: "done" })], count: 1 },
      })
    );

    const working = screen.getByTestId("sessions-board-column-select-working");
    expect(working).toHaveAccessibleName("Select all in Working");
    await user.click(working);
    expect(selectedCount()).toHaveTextContent("2 selected");
    await user.click(cardOf("d1"));
    expect(screen.getByTestId("sessions-board-column-select-working")).toHaveAccessibleName(
      "Deselect all in Working"
    );
    await user.click(screen.getByTestId("sessions-board-column-select-working"));
    // Clearing a column removes only its cards.
    expect(selectedCount()).toHaveTextContent("1 selected");
    expect(cardOf("d1")).toHaveAttribute("aria-pressed", "true");

    await user.keyboard("{Control>}a{/Control}");
    expect(selectedCount()).toHaveTextContent("3 selected");
  });

  it("Should act on the whole selection from a selected card and retarget from outside it", async () => {
    const user = userEvent.setup();
    const live = card("live", { state: "active" });
    const idle = card("idle", { state: "stopped", badge: "idle", board_column: "done" });
    const other = card("other");
    const { actions } = renderSelectable(
      board({
        working: { sessions: [live, other], count: 2 },
        done: { sessions: [idle], count: 1 },
      })
    );
    await user.click(screen.getByTestId("session-board-card-select-live"));
    await user.click(cardOf("idle"));

    await user.pointer({ keys: "[MouseRight]", target: cardOf("idle") });
    const menu = await screen.findByRole("menu");
    expect(within(menu).getByTestId("sessions-board-selection-menu-count")).toHaveTextContent(
      "2 sessions"
    );
    expect(within(menu).getByTestId("sessions-board-selection-menu-stop")).toHaveTextContent(
      "1 active"
    );
    expect(within(menu).getByTestId("sessions-board-selection-menu-archive")).toHaveTextContent(
      "Stop and archive2 sessions"
    );
    await user.click(within(menu).getByTestId("sessions-board-selection-menu-stop"));
    expect(actions.onStopMany).toHaveBeenCalledExactlyOnceWith([live, idle]);

    // Finder's rule: a context click outside the selection makes that card the selection.
    await user.pointer({ keys: "[MouseRight]", target: cardOf("other") });
    expect(await screen.findByRole("menuitem", { name: "Rename session" })).toBeInTheDocument();
    expect(selectedCount()).toHaveTextContent("1 selected");
    expect(cardOf("other")).toHaveAttribute("aria-pressed", "true");

    // Escape closes the menu onto its card, so the next Escape clears the selection.
    await user.keyboard("{Escape}");
    await waitFor(() => expect(cardOf("other")).toHaveFocus());
    await user.keyboard("{Escape}");
    expect(screen.queryByTestId("sessions-board-selection-bar")).not.toBeInTheDocument();
  });

  it("Should run the bar's verbs and confirm a delete through the list's dialog", async () => {
    const user = userEvent.setup();
    const a = card("a");
    const archived = card("arch", { archived_at: "2026-10-01T00:00:00Z", state: "stopped" });
    const { actions } = renderSelectable(
      board({ working: { sessions: [a], count: 1 } }, { sessions: [archived], count: 1 })
    );
    await user.click(screen.getByTestId("sessions-board-archive-toggle"));
    await user.click(screen.getByTestId("session-board-card-select-a"));
    await user.click(cardOf("arch"));

    await user.click(screen.getByTestId("sessions-board-selection-more"));
    expect(await screen.findByTestId("sessions-board-selection-unarchive")).toHaveTextContent(
      "1 archived"
    );
    await user.click(screen.getByTestId("sessions-board-selection-unarchive"));
    expect(actions.onUnarchiveMany).toHaveBeenCalledExactlyOnceWith([a, archived]);

    await user.click(screen.getByTestId("sessions-board-selection-delete"));
    expect(actions.onDeleteMany).toHaveBeenCalledExactlyOnceWith(
      [a, archived],
      expect.any(Function)
    );
  });

  it("Should keep a selected card that moves column and drop one gone from every read", async () => {
    const user = userEvent.setup();
    const moving = card("moving");
    const gone = card("gone");
    const { rerender } = renderSelectable(
      board({ working: { sessions: [moving, gone], count: 2 } })
    );
    await user.click(screen.getByTestId("sessions-board-column-select-working"));
    expect(selectedCount()).toHaveTextContent("2 selected");

    // Working re-read first: the card is in no column while Done is still reading. The bar
    // counts what the verbs would act on, but the card keeps its place in the selection.
    rerender(board({ working: { sessions: [gone], count: 1 }, done: { fetching: true } }));
    expect(selectedCount()).toHaveTextContent("1 selected");
    // Both reads settled without it (Done read before it finished): still held for a re-read.
    rerender(board({ working: { sessions: [gone], count: 1 } }));
    const moved = { ...moving, board_column: "done" as const, badge: "idle" as const };
    rerender(board({ working: { sessions: [gone], count: 1 }, done: { sessions: [moved] } }));
    expect(selectedCount()).toHaveTextContent("2 selected");
    expect(cardOf("moving")).toHaveAttribute("aria-pressed", "true");
    await new Promise(resolve => setTimeout(resolve, 400));
    expect(selectedCount()).toHaveTextContent("2 selected");

    // Gone from every read after the confirming re-read: it leaves the selection.
    rerender(board({ done: { sessions: [moved], count: 1 } }));
    await waitFor(() => expect(selectedCount()).toHaveTextContent("1 selected"));
    await new Promise(resolve => setTimeout(resolve, 400));
    expect(selectedCount()).toHaveTextContent("1 selected");
    await user.keyboard("{Control>}a{/Control}");
    expect(selectedCount()).toHaveTextContent("1 selected");
  });

  it("Should confirm a repeated absence with a fresh re-read, never an earlier one", async () => {
    const user = userEvent.setup();
    const flaky = card("flaky");
    const stay = card("stay");
    const present = board({ working: { sessions: [flaky, stay], count: 2 } });
    const absent = board({ working: { sessions: [stay], count: 1 } });
    const { rerender } = renderSelectable(present);
    await user.click(screen.getByTestId("sessions-board-column-select-working"));

    // Missing after a confirming re-read: it leaves the selection.
    rerender(absent);
    await waitFor(() => expect(selectedCount()).toHaveTextContent("1 selected"));
    await new Promise(resolve => setTimeout(resolve, 400));
    rerender(present);
    expect(cardOf("flaky")).toHaveAttribute("aria-pressed", "false");

    // Selected again and missing again: held until its own re-read, so a quick return keeps it.
    await user.click(cardOf("flaky"));
    rerender(absent);
    rerender(present);
    expect(cardOf("flaky")).toHaveAttribute("aria-pressed", "true");
    await new Promise(resolve => setTimeout(resolve, 400));
    expect(selectedCount()).toHaveTextContent("2 selected");
  });

  it("Should hold a missing card while a board read has failed", async () => {
    const user = userEvent.setup();
    const kept = card("kept");
    const { rerender } = renderSelectable(board({ working: { sessions: [kept], count: 1 } }));
    await user.click(screen.getByTestId("session-board-card-select-kept"));

    // The re-read returns, but a column failed: nothing confirms the card is gone.
    rerender(board({ working: { failure: "refresh" } }));
    await new Promise(resolve => setTimeout(resolve, 400));
    rerender(board({ working: { sessions: [kept], count: 1 } }));
    expect(cardOf("kept")).toHaveAttribute("aria-pressed", "true");
  });

  it("Should hold the selection while the lanes are off screen", async () => {
    const user = userEvent.setup();
    const kept = card("kept");
    const { rerender } = renderSelectable(board({ working: { sessions: [kept], count: 1 } }));
    await user.click(screen.getByTestId("session-board-card-select-kept"));

    rerender({ ...board(), columns: [], lanes: [], lanesLoading: true, groupBy: "project" });
    await new Promise(resolve => setTimeout(resolve, 400));
    rerender(board({ working: { sessions: [kept], count: 1 } }));
    expect(selectedCount()).toHaveTextContent("1 selected");
    expect(cardOf("kept")).toHaveAttribute("aria-pressed", "true");
  });

  it("Should move only a selection that sits in one project and profile", async () => {
    const user = userEvent.setup();
    renderSelectable(
      board({
        working: {
          sessions: [
            card("alpha", { workspace_id: "ws-alpha" }),
            card("beta", { workspace_id: "ws-beta" }),
          ],
          count: 2,
        },
      })
    );
    await user.click(screen.getByTestId("sessions-board-column-select-working"));
    await user.click(screen.getByTestId("sessions-board-selection-more"));
    await user.click(await screen.findByTestId("sessions-board-selection-move"));
    expect(await screen.findByTestId("sessions-board-selection-move-blocked")).toHaveTextContent(
      "Select sessions from one project and profile to move them."
    );
  });

  it("Should keep folded Done cards as hidden and drop archived cards once the bar closes", async () => {
    const user = userEvent.setup();
    const archived = card("arch", { archived_at: "2026-10-01T00:00:00Z", state: "stopped" });
    renderSelectable(
      board(
        { done: { sessions: [card("d1", { board_column: "done" })], count: 1 } },
        { sessions: [archived], count: 1 }
      )
    );
    await user.click(screen.getByTestId("session-board-card-select-d1"));
    await user.click(screen.getByTestId("sessions-board-column-toggle-done"));
    expect(selectedCount()).toHaveTextContent("1 selected· 1 hidden");

    await user.click(screen.getByTestId("sessions-board-archive-toggle"));
    await user.click(cardOf("arch"));
    expect(selectedCount()).toHaveTextContent("2 selected· 1 hidden");
    // Closing the bar drops its cards at once: reopening it right away finds them unselected.
    await user.click(screen.getByTestId("sessions-board-archive-toggle"));
    expect(selectedCount()).toHaveTextContent("1 selected· 1 hidden");
    await user.click(screen.getByTestId("sessions-board-archive-toggle"));
    expect(cardOf("arch")).toHaveAttribute("aria-pressed", "false");
  });

  it("Should keep a card's checkbox on a board that offers no verbs", async () => {
    const user = userEvent.setup();
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    renderWithTopbar(
      <QueryClientProvider client={client}>
        <SessionsBoardSelection selectionKey="board" sessionActions={bulkActions()} topbar={{}}>
          <SessionsBoard
            board={board({ working: { sessions: [card("bare")], count: 1 } })}
            onOpen={vi.fn()}
          />
        </SessionsBoardSelection>
      </QueryClientProvider>
    );
    await user.click(screen.getByTestId("session-board-card-select-bare"));
    expect(cardOf("bare")).toHaveAttribute("aria-pressed", "true");
  });

  it("Should re-render only the cards a toggle changes", async () => {
    const user = userEvent.setup();
    const ids = ["a", "b", "c", "d", "e"];
    renderSelectable(board({ working: { sessions: ids.map(id => card(id)), count: 5 } }));
    const renders = (id: string) =>
      vi.mocked(useSessionRowRuntimeLabel).mock.calls.filter(([session]) => session.id === id)
        .length;
    await user.click(screen.getByTestId("session-board-card-select-a"));
    await user.click(cardOf("b"));
    const before = Object.fromEntries(ids.map(id => [id, renders(id)]));

    await user.click(cardOf("c"));
    expect(selectedCount()).toHaveTextContent("3 selected");
    expect(renders("c")).toBeGreaterThan(before.c!);
    for (const id of ["a", "b", "d", "e"]) expect(renders(id)).toBe(before[id]);
  });
});
