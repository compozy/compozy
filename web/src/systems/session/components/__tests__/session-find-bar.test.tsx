import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useSessionFind } from "../../hooks/use-session-navigation";
import type { SessionFindJumpHandlers } from "../../lib/session-navigation-find-store";
import { primarySessionFixture } from "../../mocks/fixtures";
import type { SessionTranscriptSearchResponse } from "../../types";
import { SessionFindBar, type SessionFindBarProps } from "../session-find-bar";

// Suite: SessionFindBar (S8, US-028).
// Invariant: the bar searches the daemon's full history (never the DOM), states loading /
// no-matches / truncated / error explicitly, steps with wrap and jumps through the host's
// handlers (loading older history first when the target is unloaded, blocking steps until
// it lands), keeps focus and the active match when a refresh appends matches, and Esc closes.
// Owning layer: session navigation component. Canonical suite: this file.
// Boundary IN: search route (mocked fetch). Boundary OUT: host jump handlers.
const WORKSPACE_ID = primarySessionFixture.workspace_id!;
const SESSION_ID = primarySessionFixture.id;
const SEARCH_PATH = `/api/workspaces/${WORKSPACE_ID}/sessions/${SESSION_ID}/transcript/search`;

function matchesFor(query: string, sequences: number[]): SessionTranscriptSearchResponse {
  return {
    matches: sequences.map(sequence => ({
      role: sequence % 2 === 0 ? "assistant" : "user",
      sequence,
      snippet: `…the ${query} channel at ${sequence}…`,
      turn_id: `turn-${sequence}`,
    })),
    truncated: false,
  };
}

let searchResponses: Array<(url: URL) => Response | Promise<Response>> = [];
let searchCalls: URL[] = [];

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    headers: { "Content-Type": "application/json" },
    status,
  });
}

// The bar renders the host's `useSessionFind` model; this harness stands in for
// the viewport host with the same scope/handlers/refresh-key inputs.
type FindBarHarnessProps = SessionFindJumpHandlers &
  Omit<SessionFindBarProps, "find"> & {
    workspaceId: string;
    sessionId: string;
    initialQuery?: string;
    refreshKey?: string;
  };

function FindBarHarness({
  workspaceId,
  sessionId,
  initialQuery,
  refreshKey,
  isSequenceLoaded,
  loadOlderUntil,
  jumpToSequence,
  ...bar
}: FindBarHarnessProps) {
  const find = useSessionFind({
    handlers: { isSequenceLoaded, jumpToSequence, loadOlderUntil },
    initialQuery,
    open: true,
    refreshKey,
    sessionId,
    workspaceId,
  });
  return <SessionFindBar {...bar} find={find} />;
}

function renderFindBar(overrides: Partial<FindBarHarnessProps> = {}) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const props: FindBarHarnessProps = {
    agentName: "Claude Code",
    isSequenceLoaded: () => true,
    jumpToSequence: vi.fn(),
    loadOlderUntil: vi.fn(async () => true),
    onClose: vi.fn(),
    sessionId: SESSION_ID,
    workspaceId: WORKSPACE_ID,
    ...overrides,
  };
  const view = render(
    <QueryClientProvider client={queryClient}>
      <FindBarHarness {...props} />
    </QueryClientProvider>
  );
  return { ...view, props, queryClient };
}

describe("SessionFindBar", () => {
  beforeEach(() => {
    searchResponses = [];
    searchCalls = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = new URL(
          typeof input === "string" ? input : input instanceof URL ? input : input.url,
          "http://localhost"
        );
        if (url.pathname !== SEARCH_PATH) {
          return jsonResponse({ error: "unexpected" }, 404);
        }
        searchCalls.push(url);
        const next = searchResponses.shift();
        if (!next) return jsonResponse(matchesFor(url.searchParams.get("q") ?? "", []));
        return next(url);
      })
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("Should open empty with the key hints, focus the field, and search the daemon for the committed query", async () => {
    const user = userEvent.setup();
    searchResponses.push(url =>
      jsonResponse(matchesFor(url.searchParams.get("q") ?? "", [12, 40, 95]))
    );
    renderFindBar();

    const bar = screen.getByTestId("session-find-bar");
    expect(bar).toHaveAttribute("data-state", "empty");
    expect(screen.getByTestId("find-keys")).toBeInTheDocument();
    const input = screen.getByRole("searchbox", { name: "Find in conversation" });
    expect(input).toHaveFocus();
    expect(screen.queryByTestId("session-find-next")).not.toBeInTheDocument();

    await user.type(input, "lifecycle");
    await waitFor(() => expect(bar).toHaveAttribute("data-state", "matches"));
    expect(searchCalls.at(-1)?.searchParams.get("q")).toBe("lifecycle");
    expect(searchCalls.at(-1)?.searchParams.get("limit")).toBe("200");
    expect(screen.getByTestId("session-find-count")).toHaveTextContent("3 matches");
    const rows = screen.getAllByTestId("session-find-match");
    expect(rows).toHaveLength(3);
    expect(within(rows[0]!).getByText("Claude Code")).toBeInTheDocument();
    expect(within(rows[1]!).getByTestId("session-find-mark")).toHaveTextContent("lifecycle");
    expect(input).toHaveFocus();
  });

  it("Should state no matches plainly with the query and without step controls", async () => {
    const user = userEvent.setup();
    renderFindBar();
    await user.type(screen.getByRole("searchbox"), "sqlite");
    await waitFor(() =>
      expect(screen.getByTestId("session-find-bar")).toHaveAttribute("data-state", "no-matches")
    );
    expect(screen.getByTestId("session-find-count")).toHaveTextContent("No matches");
    expect(screen.getByTestId("session-find-empty")).toHaveTextContent(
      "No matches for “sqlite” in this conversation."
    );
    expect(screen.queryByTestId("session-find-next")).not.toBeInTheDocument();
  });

  it("Should step with Enter and Shift+Enter, jump through the host, and load older history first when the target is unloaded", async () => {
    const user = userEvent.setup();
    searchResponses.push(url =>
      jsonResponse(matchesFor(url.searchParams.get("q") ?? "", [12, 40, 95]))
    );
    let releaseOlder!: (reached: boolean) => void;
    const loadOlderUntil = vi.fn(
      () =>
        new Promise<boolean>(resolve => {
          releaseOlder = resolve;
        })
    );
    const jumpToSequence = vi.fn();
    renderFindBar({
      isSequenceLoaded: sequence => sequence >= 40,
      jumpToSequence,
      loadOlderUntil,
    });
    const input = screen.getByRole("searchbox");
    await user.type(input, "lifecycle");
    await waitFor(() => expect(screen.getAllByTestId("session-find-match")).toHaveLength(3));

    // Enter → first match (12) is not loaded: older pages first, steps blocked meanwhile.
    await user.keyboard("{Enter}");
    await waitFor(() =>
      expect(screen.getByTestId("session-find-bar")).toHaveAttribute("data-state", "loading-older")
    );
    expect(screen.getByTestId("session-find-count")).toHaveTextContent("Loading older…");
    expect(screen.getByTestId("session-find-count")).toHaveTextContent("1 of 3");
    expect(loadOlderUntil).toHaveBeenCalledWith(12);
    expect(jumpToSequence).not.toHaveBeenCalled();
    expect(screen.getByTestId("session-find-next")).toBeDisabled();
    await user.keyboard("{Enter}");
    expect(loadOlderUntil).toHaveBeenCalledTimes(1);

    await act(async () => {
      releaseOlder(true);
    });
    await waitFor(() => expect(jumpToSequence).toHaveBeenCalledWith(12));
    await waitFor(() =>
      expect(screen.getByTestId("session-find-bar")).toHaveAttribute("data-state", "matches")
    );
    const rows = screen.getAllByTestId("session-find-match");
    expect(rows[0]).toHaveAttribute("aria-selected", "true");

    // Shift+Enter wraps back to the last match, which is loaded: straight to the host.
    await user.keyboard("{Shift>}{Enter}{/Shift}");
    await waitFor(() => expect(jumpToSequence).toHaveBeenLastCalledWith(95));
    expect(loadOlderUntil).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("session-find-count")).toHaveTextContent("3 of 3");
    expect(input).toHaveFocus();
  });

  it("Should move the selection with the arrows without jumping and jump on click", async () => {
    const user = userEvent.setup();
    searchResponses.push(url =>
      jsonResponse(matchesFor(url.searchParams.get("q") ?? "", [12, 40]))
    );
    const jumpToSequence = vi.fn();
    renderFindBar({ jumpToSequence });
    await user.type(screen.getByRole("searchbox"), "lifecycle");
    await waitFor(() => expect(screen.getAllByTestId("session-find-match")).toHaveLength(2));

    await user.keyboard("{ArrowDown}");
    expect(screen.getAllByTestId("session-find-match")[0]).toHaveAttribute("aria-selected", "true");
    expect(jumpToSequence).not.toHaveBeenCalled();
    await user.keyboard("{ArrowDown}");
    expect(screen.getByTestId("session-find-count")).toHaveTextContent("2 of 2");
    await user.click(screen.getAllByTestId("session-find-match")[0]!);
    await waitFor(() => expect(jumpToSequence).toHaveBeenCalledWith(12));
  });

  it("Should keep the active match and the reader's focus when a refresh appends matches (US-028.EC-2)", async () => {
    const user = userEvent.setup();
    searchResponses.push(url =>
      jsonResponse(matchesFor(url.searchParams.get("q") ?? "", [12, 40]))
    );
    const { rerender, props, queryClient } = renderFindBar();
    const input = screen.getByRole("searchbox");
    await user.type(input, "lifecycle");
    await waitFor(() => expect(screen.getAllByTestId("session-find-match")).toHaveLength(2));
    await user.keyboard("{ArrowDown}{ArrowDown}");
    expect(screen.getByTestId("session-find-count")).toHaveTextContent("2 of 2");

    // Durable history changed: the host bumps the refresh key; the daemon now has one more hit.
    searchResponses.push(url =>
      jsonResponse(matchesFor(url.searchParams.get("q") ?? "", [12, 40, 77]))
    );
    rerender(
      <QueryClientProvider client={queryClient}>
        <FindBarHarness {...props} refreshKey="1:1:80" />
      </QueryClientProvider>
    );
    await waitFor(() => expect(screen.getAllByTestId("session-find-match")).toHaveLength(3));
    expect(screen.getByTestId("session-find-count")).toHaveTextContent("2 of 3");
    expect(screen.getByTestId("session-find-appended")).toHaveTextContent("+1 new");
    expect(screen.getAllByTestId("session-find-match")[1]).toHaveAttribute("aria-selected", "true");
    expect(input).toHaveFocus();
  });

  it("Should mark folded hits, show the truncation note, surface a search error with Try again, and close on Esc", async () => {
    const user = userEvent.setup();
    searchResponses.push(url =>
      jsonResponse({ ...matchesFor(url.searchParams.get("q") ?? "", [12, 40]), truncated: true })
    );
    const onClose = vi.fn();
    renderFindBar({ isSequenceFolded: sequence => sequence === 40, onClose });
    const input = screen.getByRole("searchbox");
    await user.type(input, "lifecycle");
    await waitFor(() => expect(screen.getAllByTestId("session-find-match")).toHaveLength(2));
    expect(screen.getByTestId("session-find-count")).toHaveTextContent("2+ matches");
    expect(screen.getByTestId("session-find-truncated")).toHaveTextContent("first 2 matches");
    expect(
      within(screen.getAllByTestId("session-find-match")[1]!).getByTestId("session-find-folded")
    ).toBeInTheDocument();

    searchResponses.push(() => jsonResponse({ error: "search unavailable" }, 503));
    await user.clear(input);
    await user.type(input, "retry me");
    await waitFor(() =>
      expect(screen.getByTestId("session-find-bar")).toHaveAttribute("data-state", "error")
    );
    expect(screen.getByTestId("session-find-count")).toHaveTextContent("Couldn't search");
    searchResponses.push(url => jsonResponse(matchesFor(url.searchParams.get("q") ?? "", [5])));
    await user.click(screen.getByTestId("session-find-retry"));
    await waitFor(() =>
      expect(screen.getByTestId("session-find-bar")).toHaveAttribute("data-state", "matches")
    );

    await user.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalledOnce();
  });
});
