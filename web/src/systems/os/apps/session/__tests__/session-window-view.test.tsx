// Suite: OS session-window view runtime selection
// Invariant: a User-scope session (no workspace, scope "user") is live work and binds the chat
// runtime to the `@user` selector; only a legacy session without any scope falls back to the
// read-only history. A window whose session has not loaded holds the frames the loaded pane
// fills (transcript, composer) under one busy status, never a bare spinner. The sessions rail is
// the window's (#1923): a session switch, through the next session's loading state, keeps the
// same rail and list mounted with their scroll, and marks the picked row current. In the Kanban
// board's drawer (#1052) there is no rail: the board beside it is the list. Only a
// not-found read says "Couldn't find this session"; any other failure says the read failed and
// offers Try again.
// Owning layer: the OS session-window view.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SessionApiError } from "@/systems/session/adapters/session-api-errors";

import { SessionWindowView } from "../session-window-view";

const instances = vi.hoisted(() => ({ current: [] as { id: string; main_session_id: string }[] }));
vi.mock("@/systems/agent", () => ({ useAgentInstances: () => instances.current }));

const runtimeProviderSpy = vi.hoisted(() => vi.fn());
const presentation = vi.hoisted(() => ({ current: "window" as "window" | "board-drawer" }));
const readOnlyTranscriptSpy = vi.hoisted(() => vi.fn());

vi.mock("../use-session-window-sidebar", () => ({
  useSessionWindowSidebar: () => ({
    open: true,
    sessions: [],
    rowProcessDialog: { open: false },
    rowRenameDialog: { open: false },
    rowLaterDialog: { open: false },
  }),
}));

vi.mock("@/systems/session", () => ({
  canPromptSession: () => true,
  useSessionWindowPresentation: () => presentation.current,
  // The list's scroll container, as the real rail hosts it, marking the row on screen.
  SessionSidebar: ({ currentSessionId }: { currentSessionId?: string }) => (
    <aside data-testid="session-sidebar">
      <div data-testid="session-sidebar-scroll" data-current={currentSessionId} />
    </aside>
  ),
  useMarkSeenOnLeave: () => undefined,
  SessionSidebarLayout: ({ sidebar, children }: { sidebar: ReactNode; children: ReactNode }) => (
    <>
      {sidebar}
      {children}
    </>
  ),
  SessionPromptRuntimeProvider: ({ children }: { children: ReactNode }) => children,
  SessionChatRuntimeProvider: (props: { workspaceId: string; children: ReactNode }) => {
    runtimeProviderSpy(props.workspaceId);
    return null;
  },
}));

vi.mock("../session-read-only-transcript", () => ({
  SessionReadOnlyTranscript: () => {
    readOnlyTranscriptSpy();
    return null;
  },
}));

vi.mock("../session-window-module-loader", () => ({
  loadSessionWindowContent: () => new Promise(() => {}),
}));

function viewElement(
  session: Record<string, unknown> | undefined,
  isLoading = false,
  {
    error = null,
    onRetry,
    id = "sess-1",
  }: { error?: Error | null; onRetry?: () => void; id?: string } = {}
) {
  return (
    <SessionWindowView
      windowId="session:sess-1"
      name="general"
      id={id}
      workspaceId=""
      session={session as never}
      liveTailEnabled
      isLoading={isLoading}
      error={error}
      onRetry={onRetry}
    />
  );
}

function renderView(...args: Parameters<typeof viewElement>) {
  return render(viewElement(...args));
}

describe("SessionWindowView", () => {
  it("Should keep a superseded instance session read-only and link to its current main", () => {
    instances.current = [{ id: "instance", main_session_id: "sess-current" }];
    renderView({ id: "sess-1", agent_name: "general", scope: "user", instance_id: "instance" });
    expect(readOnlyTranscriptSpy).toHaveBeenCalled();
    expect(runtimeProviderSpy).not.toHaveBeenCalled();
    expect(screen.getByRole("link", { name: "Open the current one" })).toHaveAttribute(
      "href",
      "/session/sess-current"
    );
  });

  beforeEach(() => {
    instances.current = [];
    presentation.current = "window";
    runtimeProviderSpy.mockClear();
    readOnlyTranscriptSpy.mockClear();
  });

  it("Should bind a User-scope session to the live runtime through @user", () => {
    renderView({ id: "sess-1", agent_name: "general", scope: "user" });

    expect(runtimeProviderSpy).toHaveBeenCalledWith("@user");
    expect(readOnlyTranscriptSpy).not.toHaveBeenCalled();
  });

  it("Should keep a legacy session without a scope in the read-only history", () => {
    renderView({ id: "sess-1", agent_name: "general" });

    expect(readOnlyTranscriptSpy).toHaveBeenCalled();
    expect(runtimeProviderSpy).not.toHaveBeenCalled();
  });

  it("Should hold the window's frames under one busy status while its session loads", () => {
    renderView(undefined, true);

    const region = screen.getByTestId("session-route-loading");
    expect(region).toHaveAttribute("role", "status");
    expect(region).toHaveAttribute("aria-busy", "true");
    expect(region).toHaveTextContent("Loading session");
    // The window's live rail frames the pane; the transcript rows and the composer card are each
    // in the place they will fill.
    expect(screen.getByTestId("session-sidebar")).toBeInTheDocument();
    expect(region).not.toContainElement(screen.getByTestId("session-sidebar"));
    expect(region.querySelector('[data-slot="skeleton-text"]')).not.toBeNull();
    expect(screen.getByTestId("composer-skeleton")).toBeInTheDocument();
    // One announcement for the whole window: the placeholders are never statuses of their own.
    expect(screen.getAllByRole("status")).toHaveLength(1);
    expect(runtimeProviderSpy).not.toHaveBeenCalled();
  });

  it("Should offer Try again when the read failed for a reason other than not-found", async () => {
    const onRetry = vi.fn();
    renderView(undefined, false, {
      error: new SessionApiError("daemon unavailable", 503, "sess-1"),
      onRetry,
    });

    expect(screen.getByRole("alert")).toHaveTextContent("Couldn't load this session");
    expect(screen.queryByText("Couldn't find this session")).not.toBeInTheDocument();
    await userEvent.click(screen.getByTestId("session-window-retry"));
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it("Should say the session is missing only for a not-found read", () => {
    renderView(undefined, false, {
      error: new SessionApiError("session not found", 404, "sess-1"),
      onRetry: vi.fn(),
    });

    expect(screen.getByRole("alert")).toHaveTextContent("Couldn't find this session");
    expect(screen.queryByTestId("session-window-retry")).not.toBeInTheDocument();
  });

  it("Should keep the rail and its list scroll mounted across a session switch (#1923)", () => {
    const sessionFor = (id: string) => ({ id, agent_name: "general", scope: "user" });
    const { rerender } = renderView(sessionFor("sess-1"));
    const scroll = screen.getByTestId("session-sidebar-scroll");
    scroll.scrollTop = 480;

    // The picked session loads, then shows: the pane swaps, the rail never remounts.
    rerender(viewElement(undefined, true, { id: "sess-2" }));
    expect(screen.getByTestId("session-route-loading")).toBeInTheDocument();
    expect(screen.getByTestId("session-sidebar-scroll")).toBe(scroll);
    expect(scroll).toHaveAttribute("data-current", "sess-2");

    rerender(viewElement(sessionFor("sess-2"), false, { id: "sess-2" }));
    expect(screen.getByTestId("session-sidebar-scroll")).toBe(scroll);
    expect(scroll.scrollTop).toBe(480);
    expect(scroll).toHaveAttribute("data-current", "sess-2");
  });

  it("Should lay the rail beside the session in a window but never in the board drawer", () => {
    const loaded = { id: "sess-1", agent_name: "general", scope: "user" };
    const { unmount } = renderView(loaded);
    expect(screen.getByTestId("session-sidebar")).toBeInTheDocument();
    unmount();

    presentation.current = "board-drawer";
    renderView(loaded);
    expect(screen.queryByTestId("session-sidebar")).toBeNull();
    expect(runtimeProviderSpy).toHaveBeenCalledWith("@user");
  });
});
