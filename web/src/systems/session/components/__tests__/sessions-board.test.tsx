// Suite: sessions-board
// Invariant: the board renders the daemon's four columns as labelled regions
// whose names carry their facet counts; a card names its parent ("↳") or its
// child rollup, flags a child that needs you, and opens its session on click;
// Done folds behind a disclosure button; only an initial read failure replaces
// a column, while a failed next page or background re-read keeps the loaded
// cards with a local retry; the Archive bar is absent only after a read
// confirms zero archived sessions — a pending or failed read keeps the bar with
// its state and a retry — and lists archived cards once expanded.
// Owning layer: component (systems/session/components). Card mapping rules live
// in lib/__tests__/session-board-card.test.ts; queries and toggle in E2E-005..007.
// #952: with the session list's lifecycle verbs, every card offers them on
// right-click and behind a `…` button that never opens the session; a grouped
// board renders one swimlane per group, each reading the four columns through
// the group's own scope.
// #958: a board or column that has not answered yet holds its place with
// card-shaped placeholders under one busy status region — never a "Loading…"
// sentence, never an empty-state claim — and loaded cards replace them. The
// standalone board skeleton needs no query client.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { workspaceBadgeIdentity, workspacesListOptions } from "@/systems/workspace";

import { fetchSessionCatalogPage, fetchSessionFacets } from "../../adapters/session-catalog-api";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import type { SessionsBoardColumnView, SessionsBoardView } from "../../hooks/use-sessions-board";
import type { SessionBoardColumnId } from "../../lib/session-board-contract";
import { primarySessionFixture } from "../../testing";
import type { SessionPayload } from "../../types";
import { SessionsBoard } from "../session-board/sessions-board";
import { SessionsBoardSkeleton } from "../session-board/sessions-board-skeleton";

vi.mock("../../adapters/session-catalog-api", () => ({
  fetchSessionCatalogPage: vi.fn(),
  fetchSessionFacets: vi.fn(),
}));

const LABELS: Record<SessionBoardColumnId, string> = {
  needs_you: "Needs you",
  working: "Working",
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
  archive: Partial<SessionsBoardView["archive"]> = {}
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
    workspaceNameOf: () => null,
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
      children: { total: 3, needs_you: 1, working: 2, in_review: 0, done: 0 },
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

  it("Should name one child without an attention signal and omit absent family facts", () => {
    renderBoard(
      board({
        working: {
          sessions: [
            card("one", {
              children: { total: 1, needs_you: 0, working: 1, in_review: 0, done: 0 },
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

  it("Should fold Done behind a disclosure button", async () => {
    const user = userEvent.setup();
    renderBoard(board({ done: { sessions: [card("d", { board_column: "done" })], count: 1 } }));
    const disclosure = screen.getByRole("button", { name: "Collapse Done" });
    expect(disclosure).toHaveAttribute("aria-expanded", "true");
    await user.click(disclosure);
    expect(screen.getByRole("button", { name: "Expand Done" })).toHaveAttribute(
      "aria-expanded",
      "false"
    );
    expect(screen.getByTestId("sessions-board-column-done")).toHaveAttribute(
      "data-collapsed",
      "true"
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

  it("Should render one lane per group, each reading the four columns in its own scope", async () => {
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
        board: { needs_you: 0, working: 1, in_review: 0, done: 0 },
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
    expect(reads.filter(query => query?.workspace_id === "ws-beta")).toHaveLength(4);
    expect(reads.every(query => query?.sort === "created")).toBe(true);
  });

  // #1029: a project lane header is a project surface: right-click and Shift+F10 open the
  // project's own actions, the same flow every other project surface uses.
  it("Should open a project lane's actions from its header by mouse and keyboard", async () => {
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
    await user.keyboard("{Shift>}{F10}{/Shift}");
    await user.click(await screen.findByRole("menuitem", { name: "Remove project" }));
    expect(await screen.findByRole("dialog", { name: "Remove Alpha?" })).toBeVisible();
  });
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
    expect(region.querySelectorAll('[data-slot="sessions-board-column-skeleton"]')).toHaveLength(8);
    expect(region.querySelectorAll(PLACEHOLDER_CARD).length).toBeGreaterThan(8);
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
    // The four columns keep their real names, so only the cards are placeholders; the Archive
    // bar holds its place under them.
    for (const name of [...Object.values(LABELS), "Archive"]) {
      expect(region).toHaveTextContent(name);
    }
    expect(region.querySelectorAll('[data-slot="sessions-board-column-skeleton"]')).toHaveLength(4);
    expect(region.querySelectorAll(PLACEHOLDER_CARD).length).toBeGreaterThan(4);
    expect(region.querySelector("button, a, input, [tabindex]")).toBeNull();

    rerender(<SessionsBoardSkeleton layout="lanes" />);

    expect(screen.getAllByRole("status")).toHaveLength(1);
    expect(
      screen.getByRole("status").querySelectorAll('[data-slot="sessions-board-lane-skeleton"]')
    ).toHaveLength(2);
  });
});
