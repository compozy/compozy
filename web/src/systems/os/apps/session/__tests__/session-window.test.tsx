// Suite: OS session-window live ownership and profile resolution
// Invariant: every visible session window on the active desktop owns a live tail, while operator
// presence additionally requires the shell window and browser document to hold focus; and the
// window resolves a session through the profile-enforced read before deciding it is gone.
// Owning layer: the OS session-window controller.
import { act, render, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const session = {
  id: "sess-1",
  agent_name: "qa-agent",
  workspace_id: "ws-1",
};
const desktop = {
  activeDesktopId: "desktop-1",
  focusedId: "session:sess-1",
  windows: {
    "session:sess-1": {
      desktopId: "desktop-1",
      minimized: false,
      stackActive: true,
      route: { pathname: "/agents/qa-agent/sessions/sess-1", search: {} },
    },
  },
};
const sessionWindowViewSpy = vi.fn((_props: Record<string, unknown>) => null);
const sessionWindowNoticeSpy = vi.fn((_props: Record<string, unknown>) => null);
const queryOptionsSpy = vi.fn((_options: Record<string, unknown>) => undefined);
const userClose = vi.fn(() => Promise.resolve(true));
const userOpen = vi.fn(() => Promise.resolve(null));
const workspace = { runtimeWorkspaceId: "ws-1" as string | null };
const documentActivity = vi.hoisted(() => ({ active: true }));
const useSessionPresenceSpy = vi.hoisted(() => vi.fn());
const queryState: {
  data: typeof session | undefined;
  error: Error | null;
  isLoading: boolean;
  isError: boolean;
} = { data: session, error: null, isLoading: false, isError: false };

vi.mock("@tanstack/react-query", () => ({
  useQuery: (options: Record<string, unknown>) => {
    queryOptionsSpy(options);
    return queryState;
  },
}));

vi.mock("@/systems/session/adapters/session-api", () => ({
  SessionNotFoundError: class SessionNotFoundError extends Error {
    readonly status = 404;

    constructor(sessionId: string) {
      super(`Session not found: ${sessionId}`);
    }
  },
}));

const scopedDetailSpy = vi.fn((..._args: unknown[]) => ({}));
const foreignState: { current: Record<string, unknown> } = { current: { status: "disabled" } };
const ownerViewSpy = vi.fn((_props: Record<string, unknown>) => null);

vi.mock("@/systems/session/lib/query-options", () => ({
  sessionScopedDetailOptions: (...args: unknown[]) => scopedDetailSpy(...args),
}));

vi.mock("@/systems/session", async importOriginal => {
  const actual = await importOriginal<Record<string, unknown>>();
  return {
    ...actual,
    sessionScopedDetailOptions: (...args: unknown[]) => scopedDetailSpy(...args),
    useForeignProfileSession: () => foreignState.current,
  };
});

vi.mock("@/systems/session/hooks/use-foreign-profile-session", () => ({
  useForeignProfileSession: () => foreignState.current,
}));

vi.mock("@/systems/profiles/hooks/use-profile-read-scope", () => ({
  useProfileReadScope: () => ({
    key: "marketing",
    aggregate: false,
    params: { profile: "marketing" },
  }),
}));

vi.mock("../session-profile-owner-notice", () => ({
  SessionProfileOwnerNotice: (props: Record<string, unknown>) => ownerViewSpy(props),
}));

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => ({ activeWorkspaceId: "ws-1", ...workspace }),
}));

vi.mock("@/hooks/use-document-active", () => ({
  useDocumentActive: () => documentActivity.active,
}));

vi.mock("@/systems/session/hooks/use-session-presence", () => ({
  useSessionPresence: (...args: unknown[]) => useSessionPresenceSpy(...args),
}));

vi.mock("../../../hooks/use-desktop", () => ({
  useDesktop: (selector: (state: typeof desktop) => unknown) => selector(desktop),
}));

vi.mock("../../../hooks/use-os-shell", () => ({
  useOsShell: () => ({
    coordinator: {
      userClose,
      userOpen,
    },
  }),
}));

vi.mock("../session-window-view", () => ({
  SessionWindowNotice: (props: Record<string, unknown>) => sessionWindowNoticeSpy(props),
  SessionWindowView: (props: Record<string, unknown>) => sessionWindowViewSpy(props),
}));

import { SessionWindow } from "../session-window";
import { SessionNotFoundError } from "@/systems/session";

describe("SessionWindow", () => {
  beforeEach(() => {
    desktop.activeDesktopId = "desktop-1";
    desktop.focusedId = "session:sess-1";
    desktop.windows["session:sess-1"].desktopId = "desktop-1";
    desktop.windows["session:sess-1"].minimized = false;
    desktop.windows["session:sess-1"].stackActive = true;
    queryState.data = session;
    queryState.error = null;
    queryState.isLoading = false;
    queryState.isError = false;
    workspace.runtimeWorkspaceId = "ws-1";
    documentActivity.active = true;
    sessionWindowViewSpy.mockClear();
    sessionWindowNoticeSpy.mockClear();
    queryOptionsSpy.mockClear();
    scopedDetailSpy.mockClear();
    ownerViewSpy.mockClear();
    foreignState.current = { status: "disabled" };
    userClose.mockClear();
    userOpen.mockClear();
    useSessionPresenceSpy.mockClear();
  });

  it.each([
    {
      name: "the session is minimized",
      arrange: () => {
        desktop.windows["session:sess-1"].minimized = true;
      },
    },
    {
      name: "another desktop becomes active",
      arrange: () => {
        desktop.activeDesktopId = "desktop-2";
      },
    },
    {
      name: "another window covers its stack position",
      arrange: () => {
        desktop.windows["session:sess-1"].stackActive = false;
      },
    },
  ])("Should release live-tail ownership when $name", ({ arrange }) => {
    const view = render(<SessionWindow windowId="session:sess-1" />);
    expect(sessionWindowViewSpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ liveTailEnabled: true })
    );

    arrange();
    view.rerender(<SessionWindow windowId="session:sess-1" />);

    expect(sessionWindowViewSpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ liveTailEnabled: false })
    );
  });

  it("Should retain live-tail ownership when another visible window gains focus", () => {
    const view = render(<SessionWindow windowId="session:sess-1" />);

    desktop.focusedId = "app:marketplace";
    view.rerender(<SessionWindow windowId="session:sess-1" />);

    expect(sessionWindowViewSpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ liveTailEnabled: true })
    );
  });

  it("Should release live-tail ownership before closing a deleted session window", async () => {
    render(<SessionWindow windowId="session:sess-1" />);
    const props = sessionWindowViewSpy.mock.lastCall?.[0] as
      | { onDeleteSuccess?: () => void }
      | undefined;

    act(() => props?.onDeleteSuccess?.());

    expect(scopedDetailSpy).toHaveBeenLastCalledWith(
      "sess-1",
      { profile: "marketing" },
      expect.objectContaining({ enabled: true, liveTail: false })
    );
    await waitFor(() => expect(userClose).toHaveBeenCalledExactlyOnceWith("session:sess-1"));
    await waitFor(() =>
      expect(userOpen).toHaveBeenCalledExactlyOnceWith({
        app: "agents",
        route: { pathname: "/agents/qa-agent", search: {} },
      })
    );
  });

  it("Should release operator presence when the browser document loses focus", () => {
    const view = render(<SessionWindow windowId="session:sess-1" />);
    expect(useSessionPresenceSpy).toHaveBeenLastCalledWith("ws-1", "sess-1", true);

    documentActivity.active = false;
    view.rerender(<SessionWindow windowId="session:sess-1" />);

    expect(useSessionPresenceSpy).toHaveBeenLastCalledWith("ws-1", "sess-1", false);
    expect(sessionWindowViewSpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ liveTailEnabled: true })
    );
  });

  it("Should render a Global-scope session from its authoritative workspace", () => {
    workspace.runtimeWorkspaceId = "";

    render(<SessionWindow windowId="session:sess-1" />);

    expect(sessionWindowViewSpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ session, workspaceId: "ws-1" })
    );
    expect(useSessionPresenceSpy).toHaveBeenLastCalledWith("ws-1", "sess-1", true);
    expect(sessionWindowNoticeSpy).not.toHaveBeenCalled();
  });

  it("Should show the truthful gone notice and return a restored missing session to its agent list (UT-087)", async () => {
    queryState.data = undefined;
    queryState.error = new SessionNotFoundError("sess-1");
    queryState.isError = true;
    foreignState.current = { status: "missing" };
    render(<SessionWindow windowId="session:sess-1" />);

    expect(sessionWindowViewSpy).toHaveBeenLastCalledWith(
      expect.objectContaining({
        error: expect.objectContaining({ message: "Session not found: sess-1" }),
        session: undefined,
      })
    );
    await waitFor(() => expect(userClose).toHaveBeenCalledExactlyOnceWith("session:sess-1"));
    await waitFor(() =>
      expect(userOpen).toHaveBeenCalledExactlyOnceWith({
        app: "agents",
        route: { pathname: "/agents/qa-agent", search: {} },
      })
    );
  });

  it("Should retain the same gone-state recovery path when the active tab's drilled session disappears (UT-089)", async () => {
    queryState.data = undefined;
    queryState.error = new SessionNotFoundError("sess-1");
    queryState.isError = true;
    foreignState.current = { status: "missing" };
    desktop.focusedId = "session:sess-1";
    render(<SessionWindow windowId="session:sess-1" />);

    await waitFor(() => expect(userOpen).toHaveBeenCalledTimes(1));
    expect(sessionWindowViewSpy).toHaveBeenLastCalledWith(
      expect.objectContaining({
        error: expect.objectContaining({ message: "Session not found: sess-1" }),
        session: undefined,
      })
    );
    expect(userOpen).toHaveBeenLastCalledWith({
      app: "agents",
      route: { pathname: "/agents/qa-agent", search: {} },
    });
  });

  it("Should read the session through the profile-enforced route, keyed by the active lens", () => {
    render(<SessionWindow windowId="session:sess-1" />);

    // Not the workspace detail route: that one answers 200 for any profile, so a
    // boundary resting on it would never see a foreign session as foreign.
    expect(scopedDetailSpy).toHaveBeenLastCalledWith(
      "sess-1",
      { profile: "marketing" },
      expect.objectContaining({ enabled: true })
    );
  });

  it("Should hold the window open while the aggregate lookup is still in flight", async () => {
    queryState.data = undefined;
    queryState.error = new SessionNotFoundError("sess-1");
    queryState.isError = true;
    foreignState.current = { status: "loading" };

    render(<SessionWindow windowId="session:sess-1" />);
    await waitFor(() => expect(sessionWindowViewSpy).toHaveBeenCalled());

    // Closing here is what made a visible session look deleted: the scoped miss
    // alone does not mean gone, only "not in this profile".
    expect(userClose).not.toHaveBeenCalled();
    expect(userOpen).not.toHaveBeenCalled();
    expect(sessionWindowViewSpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ isLoading: true })
    );
  });

  it("Should render a foreign session under its owner banner instead of bouncing", async () => {
    queryState.data = undefined;
    queryState.error = new SessionNotFoundError("sess-1");
    queryState.isError = true;
    foreignState.current = {
      status: "found",
      session,
      owner: { id: "01J9CONSULTING000000000000", name: "consulting", archived: false },
    };

    render(<SessionWindow windowId="session:sess-1" />);

    await waitFor(() => expect(ownerViewSpy).toHaveBeenCalled());
    expect(ownerViewSpy).toHaveBeenLastCalledWith(
      expect.objectContaining({
        owner: expect.objectContaining({ name: "consulting" }),
        session,
      })
    );
    expect(userClose).not.toHaveBeenCalled();
    expect(sessionWindowNoticeSpy).not.toHaveBeenCalled();
  });

  it("Should surface a failed owner lookup instead of claiming the session is gone", async () => {
    queryState.data = undefined;
    queryState.error = new SessionNotFoundError("sess-1");
    queryState.isError = true;
    foreignState.current = { status: "error", error: new Error("Owner lookup failed") };

    render(<SessionWindow windowId="session:sess-1" />);

    await waitFor(() =>
      expect(sessionWindowNoticeSpy).toHaveBeenLastCalledWith({ message: "Owner lookup failed" })
    );
    expect(userClose).not.toHaveBeenCalled();
  });
});
