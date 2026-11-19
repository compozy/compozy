// Suite: the session window's side chat host (`_uiux.md` S3, `_tests.md` web cases).
// Invariant: the window's side chat host opens, switches, hides and closes side chats through one
// drawer store shared with the subagent drawer; `/side` submissions open a side chat through
// `POST …/sides` with one idempotency key per question; the head toggle and the main status chip
// read the side list and the main session truthfully.
// Owning layer: `use-session-window-side-chat` + `session-side-drawer` (the window composition).
// Boundary OUT: the side chat adapter, the archive and session-detail adapters, and the side
// thread (its transcript and composer belong to the session thread suite).
// Canonical suite: this file (no existing suite owns the side chat host).
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { UIProvider } from "@compozy/ui";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  type SessionPayload,
  type SessionSideItem,
  sessionWindowDrawerKind,
  sessionWindowDrawerLogic,
} from "@/systems/session";
import {
  sideChatAnsweringFixture,
  sideChatClarifyFixture,
  sideChatIdleFixture,
  sideChatListFixture,
  sideChatMainSession,
  sideChatMainSessionFixture,
  sideChatPermissionFixture,
} from "@/systems/session/mocks/side-chat-fixtures";

const mocks = vi.hoisted(() => ({
  openSessionSide: vi.fn(),
  fetchSessionSides: vi.fn(),
  archiveSession: vi.fn(),
  fetchSession: vi.fn(),
  toastError: vi.fn(),
}));

vi.mock("@/systems/session/adapters/session-side-api", () => ({
  openSessionSide: mocks.openSessionSide,
  fetchSessionSides: mocks.fetchSessionSides,
}));
vi.mock("@/systems/session/adapters/session-archive-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/session/adapters/session-archive-api")>()),
  archiveSession: mocks.archiveSession,
}));
vi.mock("@/systems/session/adapters/session-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/session/adapters/session-api")>()),
  fetchSession: mocks.fetchSession,
}));
// The side thread is the session thread suite's; here it only names the session it binds.
vi.mock("../session-side-thread", () => ({
  SessionSideThread: ({ side }: { side: SessionPayload }) => (
    <div data-session={side.id} data-testid="side-thread-stub" />
  ),
}));
vi.mock("sonner", () => ({ toast: { error: mocks.toastError, success: vi.fn() } }));

import { SessionSideDrawer } from "../session-side-drawer";
import {
  type SessionWindowSideChat,
  useSessionWindowSideChat,
} from "../use-session-window-side-chat";

const WORKSPACE_ID = "ws-side";
let chat: SessionWindowSideChat;

function MainPane({ value, permission }: { value: SessionWindowSideChat; permission: boolean }) {
  chat = value;
  return (
    <div>
      {value.control}
      <textarea aria-label="Session prompt" data-testid="composer-input" />
      {permission ? (
        <button data-testid="permission-allow-once" type="button">
          Allow once
        </button>
      ) : null}
    </div>
  );
}

function Window({ session, permission }: { session: SessionPayload; permission: boolean }) {
  const value = useSessionWindowSideChat({ windowId: "w-1", session, workspaceId: WORKSPACE_ID });
  return (
    <SessionSideDrawer chat={value} session={session} workspaceId={WORKSPACE_ID}>
      <MainPane permission={permission} value={value} />
    </SessionSideDrawer>
  );
}

function renderWindow(
  {
    session = sideChatMainSessionFixture,
    sides = [],
    permission = false,
  }: { session?: SessionPayload; sides?: SessionSideItem[]; permission?: boolean } = {},
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
) {
  mocks.fetchSessionSides.mockResolvedValue({ sides });
  const tree = (next: SessionPayload) => (
    <QueryClientProvider client={queryClient}>
      <UIProvider>
        <Window permission={permission} session={next} />
      </UIProvider>
    </QueryClientProvider>
  );
  const view = render(tree(session));
  return {
    ...view,
    queryClient,
    rerenderSession: (next: SessionPayload) => view.rerender(tree(next)),
  };
}

function sidePayload(id: string, overrides: Partial<SessionPayload> = {}): SessionPayload {
  return {
    ...sideChatMainSessionFixture,
    id,
    name: "Side chat",
    lineage: { kind: "side", parent_session_id: sideChatMainSessionFixture.id },
    ...overrides,
  } as SessionPayload;
}

function deriveResult(id: string) {
  return {
    session: sidePayload(id),
    derived: { kind: "side", child_session_id: id },
  };
}

async function sidesLoaded() {
  await waitFor(() => expect(mocks.fetchSessionSides).toHaveBeenCalled());
  await act(async () => {
    await Promise.resolve();
  });
}

beforeEach(() => {
  vi.clearAllMocks();
  // Wide enough for the inline, non-modal panel beside the main pane.
  vi.stubGlobal("matchMedia", (query: string) => ({
    matches: query.includes("min-width"),
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  }));
  mocks.fetchSession.mockImplementation(async (_workspace: string, id: string) => sidePayload(id));
  mocks.archiveSession.mockImplementation(async (_workspace: string, id: string) =>
    sidePayload(id)
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("Side chat opening", () => {
  it("UT-050: Should open an empty, focused side chat for the bare `/side` chip and write nothing", async () => {
    renderWindow();
    await sidesLoaded();

    let handled = false;
    act(() => {
      handled = chat.onCommandAction("/side");
    });

    expect(handled).toBe(true);
    expect(await screen.findByTestId("side-chat-new-panel")).toBeVisible();
    await waitFor(() => expect(screen.getByTestId("side-chat-question-input")).toHaveFocus());
    expect(mocks.openSessionSide).not.toHaveBeenCalled();
    expect(chat.onCommandAction("/worktree")).toBe(false);
  });

  it("UT-087: Should open `/side <question>` through POST …/sides and show that side chat", async () => {
    mocks.openSessionSide.mockResolvedValue(deriveResult("sess-side-new"));
    renderWindow();
    await sidesLoaded();

    let cleared: boolean | undefined;
    await act(async () => {
      cleared = await chat.submitInterceptor.submit({
        text: "/side why does the refactor touch config?",
        attachmentCount: 0,
      });
    });

    expect(cleared).toBe(true);
    expect(mocks.openSessionSide).toHaveBeenCalledExactlyOnceWith(
      WORKSPACE_ID,
      sideChatMainSessionFixture.id,
      {
        message: "why does the refactor touch config?",
        idempotency_key: expect.stringMatching(/^web-side-/),
      }
    );
    expect(await screen.findByTestId("side-thread-stub")).toHaveAttribute(
      "data-session",
      "sess-side-new"
    );
  });

  it("Should refuse a `/side` question with files and keep it in the composer", async () => {
    renderWindow();
    await sidesLoaded();

    let cleared: boolean | undefined;
    await act(async () => {
      cleared = await chat.submitInterceptor.submit({ text: "/side look", attachmentCount: 1 });
    });

    expect(cleared).toBe(false);
    expect(mocks.openSessionSide).not.toHaveBeenCalled();
    expect(mocks.toastError).toHaveBeenCalledWith(
      "Side chat questions can't include files yet. Send the question alone."
    );
  });

  it("UT-051: Should show a failed open with Retry, and retry with the original key once", async () => {
    mocks.openSessionSide
      .mockRejectedValueOnce(new Error("network down"))
      .mockResolvedValueOnce(deriveResult("sess-side-retried"));
    renderWindow();
    await sidesLoaded();

    await act(async () => {
      await chat.submitInterceptor.submit({ text: "/side what now?", attachmentCount: 0 });
    });
    expect(await screen.findByTestId("side-chat-failed")).toHaveTextContent(
      "Couldn't open the side chat."
    );
    expect(screen.getByTestId("side-chat-question")).toHaveTextContent("what now?");

    fireEvent.click(screen.getByTestId("side-chat-retry"));

    expect(await screen.findByTestId("side-thread-stub")).toHaveAttribute(
      "data-session",
      "sess-side-retried"
    );
    const [first, second] = mocks.openSessionSide.mock.calls;
    expect(second?.[2]).toEqual(first?.[2]);
    expect(mocks.openSessionSide).toHaveBeenCalledTimes(2);
  });

  it("UT-052: Should show only the question while the side chat starts, then bind the side session", async () => {
    let settle!: (value: unknown) => void;
    mocks.openSessionSide.mockReturnValue(new Promise(resolve => (settle = resolve)));
    renderWindow();
    await sidesLoaded();

    act(() => {
      void chat.submitInterceptor.submit({ text: "/side what now?", attachmentCount: 0 });
    });
    const opening = await screen.findByTestId("side-chat-opening");
    expect(opening).toHaveTextContent("what now?");
    expect(screen.queryByTestId("side-thread-stub")).not.toBeInTheDocument();

    await act(async () => settle(deriveResult("sess-side-fresh")));

    const thread = await screen.findByTestId("side-thread-stub");
    expect(thread).toHaveAttribute("data-session", "sess-side-fresh");
    expect(thread).not.toHaveAttribute("data-session", sideChatMainSessionFixture.id);
  });

  it("Should hide the panel and toast the daemon's reason when the open is refused", async () => {
    const { SessionApiError } = await import("@/systems/session/adapters/session-api-errors");
    mocks.openSessionSide.mockRejectedValue(
      new SessionApiError(
        "Send a message first, then open a side chat.",
        409,
        sideChatMainSessionFixture.id,
        { code: "session_side_source_empty" }
      )
    );
    renderWindow();
    await sidesLoaded();

    let cleared: boolean | undefined;
    await act(async () => {
      cleared = await chat.submitInterceptor.submit({ text: "/side early?", attachmentCount: 0 });
    });

    expect(cleared).toBe(false);
    expect(mocks.toastError).toHaveBeenCalledWith("Send a message first, then open a side chat.");
    expect(screen.queryByTestId("side-chat-opening")).not.toBeInTheDocument();
  });
});

describe("Side chat head toggle", () => {
  it("UT-053: Should open a new side chat with none, and the most recent one with some", async () => {
    const empty = renderWindow();
    await sidesLoaded();
    expect(screen.getByTestId("session-side-toggle")).toHaveAccessibleName("Side chats");
    fireEvent.click(screen.getByTestId("session-side-toggle"));
    expect(await screen.findByTestId("side-chat-new-panel")).toBeVisible();
    empty.unmount();

    renderWindow({ sides: sideChatListFixture });
    await waitFor(() =>
      expect(screen.getByTestId("session-side-toggle-count")).toHaveTextContent(
        String(sideChatListFixture.length)
      )
    );
    fireEvent.click(screen.getByTestId("session-side-toggle"));
    expect(await screen.findByTestId("side-thread-stub")).toHaveAttribute(
      "data-session",
      sideChatListFixture[0]!.id
    );
    expect(screen.getByTestId("session-side-toggle")).toHaveAttribute("aria-pressed", "true");
  });

  it("UT-056: Should show the attention dot for a pending permission or answer, and clear it", async () => {
    const { queryClient } = renderWindow({ sides: [sideChatPermissionFixture] });
    expect(await screen.findByTestId("session-side-toggle-attention")).toBeInTheDocument();

    mocks.fetchSessionSides.mockResolvedValue({ sides: [sideChatClarifyFixture] });
    await act(async () => {
      await queryClient.invalidateQueries();
    });
    expect(screen.getByTestId("session-side-toggle-attention")).toBeInTheDocument();

    mocks.fetchSessionSides.mockResolvedValue({ sides: [sideChatIdleFixture] });
    await act(async () => {
      await queryClient.invalidateQueries();
    });
    await waitFor(() =>
      expect(screen.queryByTestId("session-side-toggle-attention")).not.toBeInTheDocument()
    );
  });

  it("Should offer no toggle on a session that can't hold side chats", async () => {
    renderWindow({ session: { ...sideChatMainSessionFixture, type: "spawned" } });
    expect(screen.queryByTestId("session-side-toggle")).not.toBeInTheDocument();
    expect(mocks.fetchSessionSides).not.toHaveBeenCalled();
  });

  it("UT-060: Should keep the new side chat's draft across hide and reopen", async () => {
    renderWindow();
    await sidesLoaded();
    fireEvent.click(screen.getByTestId("session-side-toggle"));
    fireEvent.change(await screen.findByTestId("side-chat-question-input"), {
      target: { value: "half a question" },
    });

    fireEvent.click(screen.getByTestId("side-chat-hide"));
    await waitFor(() =>
      expect(screen.queryByTestId("side-chat-new-panel")).not.toBeInTheDocument()
    );
    fireEvent.click(screen.getByTestId("session-side-toggle"));

    expect(await screen.findByTestId("side-chat-question-input")).toHaveValue("half a question");
  });
});

describe("Side chat drawer store", () => {
  it("UT-055: Should show one drawer at a time: the side panel and a subagent replace each other", () => {
    const store = sessionWindowDrawerLogic.createStore();
    const subagent = { id: "sub-1", parent_session_id: "sess-parent" } as Parameters<
      typeof store.trigger.opened
    >[0]["subagent"];

    store.trigger.sideShown({ view: { kind: "new", draft: "kept" } });
    expect(sessionWindowDrawerKind(store.getSnapshot().context)).toBe("side");

    store.trigger.opened({ subagent, workspaceId: WORKSPACE_ID, trigger: null });
    expect(sessionWindowDrawerKind(store.getSnapshot().context)).toBe("subagent");

    store.trigger.sideShown({});
    expect(sessionWindowDrawerKind(store.getSnapshot().context)).toBe("side");
    expect(store.getSnapshot().context.selected).toBeNull();
    expect(store.getSnapshot().context.side.view).toEqual({ kind: "new", draft: "kept" });
  });
});

describe("Side chat main status", () => {
  it.each([
    ["running", "active", "Main working"],
    ["waiting-for-auth", "active", "Main needs approval"],
    ["waiting-for-input", "active", "Main needs input"],
    ["done", "active", "Main finished"],
    ["failed", "active", "Main failed"],
    ["stopped", "stopped", "Main stopped"],
  ] as const)(
    "UT-057: Should read the main session's %s state as %s",
    async (badge, state, label) => {
      renderWindow({ session: sideChatMainSession(badge, state) });
      await sidesLoaded();
      fireEvent.click(screen.getByTestId("session-side-toggle"));
      expect(await screen.findByTestId("side-chat-main-status")).toHaveTextContent(label);
    }
  );

  it("UT-057: Should show no status while the main session's state is outside the vocabulary", async () => {
    renderWindow({ session: sideChatMainSession("hung") });
    await sidesLoaded();
    fireEvent.click(screen.getByTestId("session-side-toggle"));
    await screen.findByTestId("side-chat-new-panel");
    expect(screen.getByTestId("side-chat-main-status")).toBeEmptyDOMElement();
  });

  it("UT-063: Should focus the main permission prompt, or else the main composer, from the status", async () => {
    const view = renderWindow({
      session: sideChatMainSession("waiting-for-auth"),
      permission: true,
    });
    await sidesLoaded();
    fireEvent.click(screen.getByTestId("session-side-toggle"));
    fireEvent.click(await screen.findByRole("button", { name: /Main needs approval/ }));
    expect(screen.getByTestId("permission-allow-once")).toHaveFocus();

    view.rerenderSession(sideChatMainSession("running"));
    fireEvent.click(await screen.findByRole("button", { name: /Main working/ }));
    expect(screen.getByTestId("composer-input")).toHaveFocus();
  });
});

describe("Side chat close", () => {
  it("UT-061: Should confirm before closing an answering side chat, then archive it with stop", async () => {
    renderWindow({ sides: [sideChatAnsweringFixture, sideChatIdleFixture] });
    await waitFor(() =>
      expect(screen.getByTestId("session-side-toggle-count")).toHaveTextContent("2")
    );
    fireEvent.click(screen.getByTestId("session-side-toggle"));
    await screen.findByTestId("side-thread-stub");

    fireEvent.click(screen.getByTestId("side-chat-close"));
    expect(await screen.findByTestId("side-chat-close-dialog")).toHaveTextContent(
      "Stop this side chat and close it?"
    );
    expect(mocks.archiveSession).not.toHaveBeenCalled();
    fireEvent.click(screen.getByTestId("side-chat-close-confirm"));

    await waitFor(() =>
      expect(mocks.archiveSession).toHaveBeenCalledWith(
        WORKSPACE_ID,
        sideChatAnsweringFixture.id,
        sideChatMainSessionFixture.profile_name,
        undefined,
        true
      )
    );
    // The panel moves on to the next most recent side chat.
    await waitFor(() =>
      expect(screen.getByTestId("side-thread-stub")).toHaveAttribute(
        "data-session",
        sideChatIdleFixture.id
      )
    );
  });

  it("Should close an idle side chat without asking", async () => {
    renderWindow({ sides: [sideChatIdleFixture] });
    await waitFor(() =>
      expect(screen.getByTestId("session-side-toggle-count")).toHaveTextContent("1")
    );
    fireEvent.click(screen.getByTestId("session-side-toggle"));
    await screen.findByTestId("side-thread-stub");

    fireEvent.click(screen.getByTestId("side-chat-close"));

    await waitFor(() => expect(mocks.archiveSession).toHaveBeenCalledOnce());
    expect(screen.queryByTestId("side-chat-close-dialog")).not.toBeInTheDocument();
  });
});
