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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import type { SessionsBoardColumnView, SessionsBoardView } from "../../hooks/use-sessions-board";
import type { SessionBoardColumnId } from "../../lib/session-board-contract";
import { primarySessionFixture } from "../../testing";
import type { SessionPayload } from "../../types";
import { SessionsBoard } from "../session-board/sessions-board";

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
    archive: { ...page(), count: 0, ...archive },
    workspaceNameOf: () => null,
  };
}

function renderBoard(view: SessionsBoardView, onOpen = vi.fn()) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  render(<SessionsBoard board={view} onOpen={onOpen} />, { wrapper });
  return onOpen;
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
    expect(within(status).getByRole("status", { name: "Loading archived sessions" })).toBeVisible();
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
