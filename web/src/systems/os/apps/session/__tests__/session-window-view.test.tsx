// Suite: OS session-window view runtime selection
// Invariant: a User-scope session (no workspace, scope "user") is live work and binds the chat
// runtime to the `@user` selector; only a legacy session without any scope falls back to the
// read-only history. A window whose session has not loaded keeps the live sessions rail, marking
// that session current, beside the frames the loaded pane fills (transcript, composer) under one
// busy status, never a bare spinner. That rail offers no resize handle: the loaded window mounts
// its own resizable split, which would drop a drag begun on the loading one. The rail list resumes
// its window's scroll when a session switch remounts it (#1923). Only a
// not-found read says "Couldn't find this session"; any other failure says the read failed and
// offers Try again.
// Owning layer: the OS session-window view.
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { SessionApiError } from "@/systems/session/adapters/session-api-errors";

import { SessionWindowView } from "../session-window-view";

const instances = vi.hoisted(() => ({ current: [] as { id: string; main_session_id: string }[] }));
vi.mock("@/systems/agent", () => ({ useAgentInstances: () => instances.current }));

const runtimeProviderSpy = vi.hoisted(() => vi.fn());
const readOnlyTranscriptSpy = vi.hoisted(() => vi.fn());

// `listLoading` swaps the list for placeholder rows; a new `listLayout` replaces its container.
const rail = vi.hoisted(() => ({ listLoading: false, listLayout: "flat" }));

vi.mock("../use-session-window-sidebar", () => ({
  useSessionWindowSidebar: () => ({
    open: true,
    sessions: [],
    view: { catalog: { loading: rail.listLoading } },
    rowProcessDialog: { open: false },
    rowRenameDialog: { open: false },
    rowLaterDialog: { open: false },
  }),
}));

vi.mock("@/systems/session", () => ({
  canPromptSession: () => true,
  useSessionWindowPresentation: () => "window",
  // The rail's list scroll container, as `SessionList` marks it, naming the row on screen.
  SessionSidebar: ({
    currentSessionId,
    view,
  }: {
    currentSessionId?: string;
    view: { catalog: { loading: boolean } };
  }) => (
    <aside data-testid="session-sidebar">
      {view.catalog.loading ? null : (
        <div
          key={rail.listLayout}
          data-session-list-scroll
          data-testid="session-list-scroll"
          data-current={currentSessionId}
        />
      )}
    </aside>
  ),
  useMarkSeenOnLeave: () => undefined,
  // The live split, handle included, as `SessionSidebarLayout` renders it.
  SessionSidebarLayout: ({ sidebar, children }: { sidebar: ReactNode; children: ReactNode }) => (
    <>
      {sidebar}
      <div role="separator" aria-label="Resize sessions sidebar" />
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

function renderView(
  session: Record<string, unknown> | undefined,
  isLoading = false,
  {
    error = null,
    onRetry,
    id = "sess-1",
    windowId = "session:sess-1",
  }: { error?: Error | null; onRetry?: () => void; id?: string; windowId?: string } = {}
) {
  return render(
    <SessionWindowView
      windowId={windowId}
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
    rail.listLoading = false;
    rail.listLayout = "flat";
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
    // The live rail stays beside the pane's placeholder, outside its busy region; the transcript
    // rows and the composer card are each in the place they will fill.
    expect(screen.getByTestId("session-sidebar")).toBeInTheDocument();
    expect(region).not.toContainElement(screen.getByTestId("session-sidebar"));
    expect(screen.getByTestId("session-list-scroll")).toHaveAttribute("data-current", "sess-1");
    // Resizing waits for the loaded window's split, the one that keeps the width.
    expect(screen.queryByRole("separator", { name: "Resize sessions sidebar" })).toBeNull();
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

  describe("rail scroll across a session switch (#1923)", () => {
    // jsdom has no scroll layout; keep each element's scrollTop as the browser would.
    const scrollTops = new WeakMap<Element, number>();
    const original = Object.getOwnPropertyDescriptor(Element.prototype, "scrollTop");
    beforeEach(() => {
      Object.defineProperty(Element.prototype, "scrollTop", {
        configurable: true,
        get(this: Element) {
          return scrollTops.get(this) ?? 0;
        },
        set(this: Element, value: number) {
          scrollTops.set(this, value);
        },
      });
    });
    afterEach(() => {
      if (original) Object.defineProperty(Element.prototype, "scrollTop", original);
      else Reflect.deleteProperty(Element.prototype, "scrollTop");
    });

    it("Should resume the window's rail scroll when a session switch remounts it", () => {
      const first = renderView(undefined, true, { windowId: "session:switch" });
      const list = screen.getByTestId("session-list-scroll");
      list.scrollTop = 480;
      fireEvent.scroll(list);
      first.unmount();

      renderView(undefined, true, { id: "sess-2", windowId: "session:switch" });
      const resumed = screen.getByTestId("session-list-scroll");
      expect(resumed).not.toBe(list);
      expect(resumed.scrollTop).toBe(480);
      expect(resumed).toHaveAttribute("data-current", "sess-2");
    });

    it("Should resume the rail scroll once its list replaces the placeholder rows", () => {
      const first = renderView(undefined, true, { windowId: "session:late-list" });
      const list = screen.getByTestId("session-list-scroll");
      list.scrollTop = 480;
      fireEvent.scroll(list);
      first.unmount();

      rail.listLoading = true;
      const next = renderView(undefined, true, { id: "sess-2", windowId: "session:late-list" });
      expect(screen.queryByTestId("session-list-scroll")).toBeNull();
      rail.listLoading = false;
      next.rerender(
        <SessionWindowView
          windowId="session:late-list"
          name="general"
          id="sess-2"
          workspaceId=""
          session={undefined}
          liveTailEnabled
          isLoading
          error={null}
        />
      );
      expect(screen.getByTestId("session-list-scroll").scrollTop).toBe(480);
    });

    function renderSettlingRail(windowId: string) {
      const first = renderView(undefined, true, { windowId });
      const list = screen.getByTestId("session-list-scroll");
      list.scrollTop = 480;
      fireEvent.scroll(list);
      first.unmount();

      const next = renderView(undefined, true, { id: "sess-2", windowId });
      return () =>
        next.rerender(
          <SessionWindowView
            windowId={windowId}
            name="general"
            id="sess-2"
            workspaceId=""
            session={undefined}
            liveTailEnabled
            isLoading
            error={null}
          />
        );
    }

    it("Should resume the rail scroll again when a later layout replaces the list", () => {
      const rerender = renderSettlingRail("session:folders");
      const flat = screen.getByTestId("session-list-scroll");
      expect(flat.scrollTop).toBe(480);

      // The folder layout arrives after the catalog and mounts its own container.
      rail.listLayout = "folders";
      rerender();
      const folders = screen.getByTestId("session-list-scroll");
      expect(folders).not.toBe(flat);
      expect(folders.scrollTop).toBe(480);
    });

    it("Should leave the operator's later scrolling alone once they handle the rail", () => {
      const rerender = renderSettlingRail("session:settled");
      const resumed = screen.getByTestId("session-list-scroll");
      expect(resumed.scrollTop).toBe(480);

      // The operator scrolls; a re-render keeps their position.
      fireEvent.wheel(resumed);
      resumed.scrollTop = 120;
      fireEvent.scroll(resumed);
      rerender();
      expect(screen.getByTestId("session-list-scroll").scrollTop).toBe(120);

      // A list they reload (a breadth or search change) starts at the top.
      rail.listLoading = true;
      rerender();
      rail.listLoading = false;
      rerender();
      const reloaded = screen.getByTestId("session-list-scroll");
      expect(reloaded).not.toBe(resumed);
      expect(reloaded.scrollTop).toBe(0);
    });

    it("Should start another window's rail at the top", () => {
      const first = renderView(undefined, true, { windowId: "session:scrolled" });
      const list = screen.getByTestId("session-list-scroll");
      list.scrollTop = 480;
      fireEvent.scroll(list);
      first.unmount();

      renderView(undefined, true, { windowId: "session:other" });
      expect(screen.getByTestId("session-list-scroll").scrollTop).toBe(0);
    });
  });
});
