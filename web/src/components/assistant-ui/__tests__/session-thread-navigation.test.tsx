import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { SessionChatRuntimeProvider } from "@/systems/session/components/session-chat-runtime-provider";
import { sessionKeys } from "@/systems/session/lib/query-keys";
import type { SessionTranscriptData } from "@/systems/session/lib/session-transcript-query";
import { primarySessionFixture } from "@/systems/session/mocks";
import type { SessionMessage, SessionTranscriptSearchResponse } from "@/systems/session/types";

import { SessionThread } from "../session-thread";

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn(), warning: vi.fn() },
}));

vi.mock("@tanstack/react-router", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({ to, children, ...props }: Record<string, unknown>) => (
      <a href={typeof to === "string" ? to : "#"} {...(props as Record<string, unknown>)}>
        {children as React.ReactNode}
      </a>
    ),
  };
});

// Suite: session thread navigation host (task_08 — S8 find, S9 trail jumps, US-028).
// Invariant: ⌘F opens find only for the focused session window and Esc returns focus to the
// composer; a jump to a match in unloaded history pulls older pages through the transcript
// query's own continuation and then lands the row with the reader owning the viewport; a hit
// the daemon located behind a settled fold (`part_index`/`field`) opens that fold ("opened for
// a match") and the collapsed body holding the field, each hold released by the reader's own
// click; a match without a source opens the turn only and claims no part; the closed fold names
// its located matches; the full-history reads refetch when durable history settles, never per
// token delta.
// Owning layer: assistant-ui viewport host + session navigation hooks. Canonical suite: this file.
// Boundary IN: transcript pages / search route (mocked fetch). Boundary OUT: DOM rows + scroll store.
const WORKSPACE_ID = primarySessionFixture.workspace_id!;
const SESSION_ID = primarySessionFixture.id;
const SESSION_PATH = `/api/workspaces/${WORKSPACE_ID}/sessions/${SESSION_ID}`;

type Page = SessionTranscriptData["pages"][number];
type Entry = Page["entries"][number];

function entry(sequence: number, message: SessionMessage): Entry {
  return { message, sequence, start_sequence: sequence };
}

function userMessage(id: string, text: string, turnId: string): SessionMessage {
  return {
    id,
    parts: [
      { state: "done", text, timestamp: "2026-09-06T12:00:00Z", turn_id: turnId, type: "text" },
    ] as unknown as SessionMessage["parts"],
    role: "user",
  } as SessionMessage;
}

function workedMessage(id: string, turnId: string): SessionMessage {
  return {
    id,
    parts: [
      {
        input: { command: "go test ./internal/lifecycle -run Lifecycle" },
        output: { raw: { content: "ok" }, title: "Bash", type: "tool_result" },
        state: "output-available",
        timestamp: "2026-09-06T12:00:03Z",
        toolCallId: `${id}-bash`,
        turn_id: turnId,
        type: "tool-Bash",
      },
      {
        input: { file_path: "notes/ação-café.md", query: "ação λ café" },
        output: { raw: { content: "# café\n" }, title: "Read", type: "tool_result" },
        state: "output-available",
        timestamp: "2026-09-06T12:00:05Z",
        toolCallId: `${id}-read`,
        turn_id: turnId,
        type: "tool-Read",
      },
      {
        state: "done",
        text: "Done — the suite is green.",
        timestamp: "2026-09-06T12:00:08Z",
        turn_id: turnId,
        type: "text",
      },
    ] as unknown as SessionMessage["parts"],
    role: "assistant",
  } as SessionMessage;
}

const OLDER_PAGE: Page = {
  cursor: 4,
  entries: [
    entry(1, userMessage("msg-1", "Start with the store package.", "turn-1")),
    entry(2, userMessage("msg-2", "Refactor the lifecycle tests first.", "turn-2")),
    entry(3, workedMessage("msg-3", "turn-2")),
    entry(4, userMessage("msg-4", "Thanks.", "turn-4")),
  ],
  epoch: 1,
  generation: 1,
  has_older: false,
  limit: 200,
  max_sequence: 4,
};

const HEAD_PAGE: Page = {
  cursor: 103,
  entries: [
    entry(100, userMessage("msg-100", "Now the same in the store package.", "turn-100")),
    entry(102, workedMessage("msg-102", "turn-102")),
    entry(103, userMessage("msg-103", "Ship it.", "turn-103")),
  ],
  epoch: 1,
  generation: 1,
  has_older: true,
  limit: 200,
  max_sequence: 103,
  next_before_sequence: 100,
};

const LIFECYCLE_MATCHES: SessionTranscriptSearchResponse = {
  matches: [
    {
      field: "text",
      part_index: 0,
      role: "user",
      sequence: 2,
      snippet: "Refactor the lifecycle tests first.",
      turn_id: "turn-2",
    },
    {
      field: "input",
      part_index: 0,
      role: "assistant",
      sequence: 102,
      snippet: "go test ./internal/lifecycle -run Lifecycle",
      turn_id: "turn-102",
    },
  ],
  truncated: false,
};

/** The Unicode hit the daemon located in a specialized Read tool's raw input (BUG-20260906-find-specialized-tool-field). */
const CAFE_MATCHES: SessionTranscriptSearchResponse = {
  matches: [
    {
      field: "input",
      part_index: 1,
      role: "assistant",
      sequence: 102,
      snippet: "ação λ café",
      turn_id: "turn-102",
    },
  ],
  truncated: false,
};

/** The same hits as an older daemon reports them: the entry only, no part or field. */
const LEGACY_MATCHES: SessionTranscriptSearchResponse = {
  matches: LIFECYCLE_MATCHES.matches.map(
    ({ field: _field, part_index: _index, ...match }) => match
  ),
  truncated: false,
};

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    headers: { "Content-Type": "application/json" },
    status,
  });
}

interface FetchLog {
  olderPageRequests: number;
  searchRequests: string[];
}

function createFetchMock(log: FetchLog, search: () => SessionTranscriptSearchResponse) {
  return vi.fn(async (input: RequestInfo | URL) => {
    const url = new URL(
      typeof input === "string" ? input : input instanceof URL ? input : input.url,
      "http://localhost"
    );
    const { pathname } = url;
    if (pathname === "/api/sessions") return jsonResponse({ sessions: [primarySessionFixture] });
    if (pathname === SESSION_PATH) return jsonResponse({ session: primarySessionFixture });
    if (pathname === `${SESSION_PATH}/transcript`) {
      const before = url.searchParams.get("before_sequence");
      if (before === "100") {
        log.olderPageRequests += 1;
        return jsonResponse(OLDER_PAGE);
      }
      return jsonResponse(HEAD_PAGE);
    }
    if (pathname === `${SESSION_PATH}/transcript/search`) {
      log.searchRequests.push(url.searchParams.get("q") ?? "");
      return jsonResponse(search());
    }
    if (pathname === `${SESSION_PATH}/transcript/outline`) return jsonResponse({ entries: [] });
    if (pathname === `${SESSION_PATH}/clarifications`) return jsonResponse({ clarifications: [] });
    if (pathname === `${SESSION_PATH}/interactions`) return jsonResponse({ interactions: [] });
    if (pathname === `${SESSION_PATH}/prompt/queue`) return jsonResponse({ inputs: [] });
    throw new Error(`Unhandled fetch in navigation test: ${pathname}`);
  });
}

function renderNavigationThread({
  isSessionRunning = false,
  frame,
}: { isSessionRunning?: boolean; frame?: { focused: boolean } } = {}) {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });
  queryClient.setQueryData<SessionTranscriptData>(
    sessionKeys.transcript(WORKSPACE_ID, SESSION_ID),
    {
      pageParams: [undefined],
      pages: [HEAD_PAGE],
    }
  );
  const thread = (running: boolean) => (
    <QueryClientProvider client={queryClient}>
      <SessionChatRuntimeProvider
        liveTailEnabled={false}
        sessionId={SESSION_ID}
        workspaceId={WORKSPACE_ID}
      >
        <SessionThread
          agentName={primarySessionFixture.agent_name}
          canPrompt
          isSessionRunning={running}
          onCancelPrompt={() => {}}
          sessionId={SESSION_ID}
          statusSession={primarySessionFixture}
        />
      </SessionChatRuntimeProvider>
    </QueryClientProvider>
  );
  const wrap = (running: boolean) =>
    frame ? (
      <section data-slot="os-window-frame" data-focused={frame.focused ? "" : undefined}>
        {thread(running)}
      </section>
    ) : (
      thread(running)
    );
  const view = render(wrap(isSessionRunning));
  return {
    ...view,
    queryClient,
    setRunning: (running: boolean) => view.rerender(wrap(running)),
  };
}

function pressFindShortcut() {
  fireEvent.keyDown(document.body, { ctrlKey: true, key: "f" });
}

describe("SessionThread navigation host", () => {
  let log: FetchLog;
  let searchResponse: SessionTranscriptSearchResponse;

  beforeEach(() => {
    log = { olderPageRequests: 0, searchRequests: [] };
    searchResponse = LIFECYCLE_MATCHES;
    vi.stubGlobal(
      "fetch",
      createFetchMock(log, () => searchResponse)
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("Should open find for the focused window only and return focus to the composer on Esc", async () => {
    const user = userEvent.setup();
    const { unmount } = renderNavigationThread({ frame: { focused: false } });
    await screen.findByText("Ship it.");
    pressFindShortcut();
    expect(screen.queryByTestId("session-find-bar")).not.toBeInTheDocument();
    unmount();

    renderNavigationThread({ frame: { focused: true } });
    await screen.findByText("Ship it.");
    pressFindShortcut();
    const input = await screen.findByTestId("session-find-input");
    expect(input).toHaveFocus();
    expect(screen.getByTestId("session-find-bar")).toHaveAttribute("data-state", "empty");

    await user.keyboard("{Escape}");
    expect(screen.queryByTestId("session-find-bar")).not.toBeInTheDocument();
    expect(screen.getByTestId("composer-input")).toContainElement(
      document.activeElement as HTMLElement
    );
  });

  it("Should load older history for an unloaded match, land it with the reader owning the viewport, and open the fold behind a work hit", async () => {
    const user = userEvent.setup();
    renderNavigationThread();
    await screen.findByText("Ship it.");
    expect(screen.queryByText("Refactor the lifecycle tests first.")).not.toBeInTheDocument();

    pressFindShortcut();
    await user.type(await screen.findByTestId("session-find-input"), "lifecycle");
    const rows = await screen.findAllByTestId("session-find-match");
    expect(rows).toHaveLength(2);
    // The work hit sits behind the settled fold: the list says so, and so does the fold.
    expect(rows[1]).toHaveAttribute("data-folded", "true");
    const fold = screen.getByRole("button", { name: /Worked for/ });
    expect(fold).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByTestId("turn-fold-note")).toHaveTextContent("1 match inside");

    // Enter → the first match is in unloaded history: the continuation pulls the page, then lands.
    await user.keyboard("{Enter}");
    await waitFor(() => expect(log.olderPageRequests).toBe(1));
    expect(await screen.findByText("Refactor the lifecycle tests first.")).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByTestId("session-find-bar")).toHaveAttribute("data-state", "matches")
    );
    expect(screen.getByTestId("session-find-count")).toHaveTextContent("1 of 2");
    // The reader owns the viewport once the landing scroll runs: the pill offers the way back.
    await waitFor(() =>
      expect(screen.getByTestId("scroll-to-bottom-pill")).toHaveAttribute("data-visible", "true")
    );
    expect(screen.queryByTestId("session-find-jump-error")).not.toBeInTheDocument();

    // Enter → the work hit the daemon located in the tool's input: its fold opens for
    // the match and the tool row's body opens so the matched field is on screen.
    await user.keyboard("{Enter}");
    await waitFor(() => expect(fold).toHaveAttribute("aria-expanded", "true"));
    expect(screen.getByTestId("turn-fold-note")).toHaveTextContent("opened for a match");
    // The settled work group opens for the match and the matched tool's body with it.
    const toolRow = within(fold.parentElement!)
      .getAllByTestId("tool-call-row")
      .find(row => row.dataset.partIndex === "0")!;
    const toolTrigger = toolRow.querySelector<HTMLElement>('[data-slot="tool-call-row-trigger"]')!;
    expect(toolTrigger).toHaveAttribute("aria-expanded", "true");
    expect(within(toolRow).getByTestId("tool-matched-field")).toHaveAttribute(
      "data-field",
      "input"
    );
    expect(screen.getByTestId("session-find-count")).toHaveTextContent("2 of 2");
    expect(log.olderPageRequests).toBe(1);

    // The reader closes the tool body: only that hold drops; the fold stays open for the match.
    await user.click(toolTrigger);
    await waitFor(() => expect(toolTrigger).toHaveAttribute("aria-expanded", "false"));
    expect(fold).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByTestId("turn-fold-note")).toHaveTextContent("opened for a match");

    // Closing the fold by hand is one click, which releases the jump's hold; the
    // closed fold goes back to naming what find found inside it.
    await user.click(fold);
    await waitFor(() => expect(fold).toHaveAttribute("aria-expanded", "false"));
    expect(screen.getByTestId("turn-fold-note")).toHaveTextContent("1 match inside");
  });

  it("Should reveal the matched raw input of a specialized Read tool on the jump (BUG-20260906-find-specialized-tool-field)", async () => {
    const user = userEvent.setup();
    searchResponse = CAFE_MATCHES;
    renderNavigationThread();
    await screen.findByText("Ship it.");
    pressFindShortcut();
    await user.type(await screen.findByTestId("session-find-input"), "café");
    await screen.findAllByTestId("session-find-match");
    const fold = screen.getByRole("button", { name: /Worked for/ });
    expect(screen.getByTestId("turn-fold-note")).toHaveTextContent("1 match inside");

    await user.keyboard("{Enter}");
    await waitFor(() => expect(fold).toHaveAttribute("aria-expanded", "true"));
    const readRow = within(fold.parentElement!)
      .getAllByTestId("tool-call-row")
      .find(row => row.dataset.partIndex === "1")!;
    const trigger = readRow.querySelector<HTMLElement>('[data-slot="tool-call-row-trigger"]')!;
    await waitFor(() => expect(trigger).toHaveAttribute("aria-expanded", "true"));
    // The specialized Read display stays; the matched raw input renders in full beside it.
    expect(within(readRow).getByTestId("read-content")).toHaveTextContent("notes/ação-café.md");
    const matched = within(readRow).getByTestId("tool-matched-field");
    expect(matched).toHaveAttribute("data-field", "input");
    expect(matched).toHaveTextContent("ação λ café");
    // The other tool in the fold is untouched by the reveal.
    const bashRow = within(fold.parentElement!)
      .getAllByTestId("tool-call-row")
      .find(row => row.dataset.partIndex === "0")!;
    expect(within(bashRow).queryByTestId("tool-matched-field")).not.toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByTestId("session-find-bar")).toHaveAttribute("data-state", "matches")
    );
    expect(screen.queryByTestId("session-find-jump-error")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-find-input")).toHaveFocus();
  });

  it("Should open only the turn for a match without a source and never claim a part or a fold", async () => {
    const user = userEvent.setup();
    searchResponse = LEGACY_MATCHES;
    renderNavigationThread();
    await screen.findByText("Ship it.");
    pressFindShortcut();
    await user.type(await screen.findByTestId("session-find-input"), "lifecycle");
    const rows = await screen.findAllByTestId("session-find-match");
    // Nothing located: no "folded" tag on the list row, no count on the fold.
    expect(rows[1]).not.toHaveAttribute("data-folded");
    expect(screen.queryByTestId("turn-fold-note")).not.toBeInTheDocument();
    const fold = screen.getByRole("button", { name: /Worked for/ });

    await user.click(rows[1]!);
    await waitFor(() => expect(fold).toHaveAttribute("aria-expanded", "true"));
    expect(screen.getByTestId("turn-fold-note")).toHaveTextContent("opened for a match");
    await waitFor(() =>
      expect(screen.getByTestId("session-find-bar")).toHaveAttribute("data-state", "matches")
    );
    expect(screen.queryByTestId("session-find-jump-error")).not.toBeInTheDocument();
    // The turn opened conservatively; both tool bodies stay collapsed and no
    // field is claimed when the result has no source location.
    const toolRows = within(fold.parentElement!).getAllByTestId("tool-call-row");
    expect(toolRows).toHaveLength(2);
    for (const row of toolRows) {
      expect(row.querySelector('[data-slot="tool-call-row-trigger"]')).toHaveAttribute(
        "aria-expanded",
        "false"
      );
    }
    expect(screen.queryByTestId("tool-matched-field")).not.toBeInTheDocument();
    expect(log.olderPageRequests).toBe(0);
  });

  it("Should refetch the full-history search when the turn settles, not on token deltas", async () => {
    const user = userEvent.setup();
    const { queryClient, setRunning } = renderNavigationThread({ isSessionRunning: true });
    await screen.findByText("Ship it.");
    pressFindShortcut();
    await user.type(await screen.findByTestId("session-find-input"), "lifecycle");
    await screen.findAllByTestId("session-find-match");
    expect(log.searchRequests).toEqual(["lifecycle"]);

    // A token delta rewrites the streaming edge in the cache: same entries, no reread.
    await act(async () => {
      queryClient.setQueryData<SessionTranscriptData>(
        sessionKeys.transcript(WORKSPACE_ID, SESSION_ID),
        data => {
          if (!data) return data;
          const head = data.pages[0]!;
          const last = head.entries[head.entries.length - 1]!;
          return {
            ...data,
            pages: [
              {
                ...head,
                entries: [
                  ...head.entries.slice(0, -1),
                  { ...last, message: userMessage("msg-103", "Ship it. Now.", "turn-103") },
                ],
              },
              ...data.pages.slice(1),
            ],
          };
        }
      );
    });
    await screen.findByText("Ship it. Now.");
    expect(log.searchRequests).toEqual(["lifecycle"]);

    // The turn settles: durable history changed, the reads refresh once.
    setRunning(false);
    await waitFor(() => expect(log.searchRequests).toEqual(["lifecycle", "lifecycle"]));
    expect(screen.getByTestId("session-find-input")).toHaveFocus();
  });
});
