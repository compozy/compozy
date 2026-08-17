// Suite: OS session-window live ownership
// Invariant: every visible session window on the active desktop owns a live tail, while operator
// presence additionally requires the shell window and browser document to hold focus.
// Owning layer: the OS session-window controller.
import { render, waitFor } from "@testing-library/react";
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
} = { data: session, error: null, isLoading: false };

vi.mock("@tanstack/react-query", () => ({
  useQuery: (options: Record<string, unknown>) => {
    queryOptionsSpy(options);
    return queryState;
  },
}));

vi.mock("@/systems/session/adapters/session-api", () => ({
  SessionNotFoundError: class SessionNotFoundError extends Error {
    constructor(sessionId: string) {
      super(`Session not found: ${sessionId}`);
    }
  },
}));

vi.mock("@/systems/session/lib/query-options", () => ({
  sessionDetailOptions: () => ({}),
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
    workspace.runtimeWorkspaceId = "ws-1";
    documentActivity.active = true;
    sessionWindowViewSpy.mockClear();
    sessionWindowNoticeSpy.mockClear();
    queryOptionsSpy.mockClear();
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

  it("Should treat an empty runtime workspace as unavailable without querying or rendering", () => {
    workspace.runtimeWorkspaceId = "";

    render(<SessionWindow windowId="session:sess-1" />);

    expect(queryOptionsSpy).toHaveBeenLastCalledWith(expect.objectContaining({ enabled: false }));
    expect(sessionWindowNoticeSpy).toHaveBeenLastCalledWith({
      message: "Session workspace unavailable",
    });
    expect(sessionWindowViewSpy).not.toHaveBeenCalled();
  });

  it("Should show the truthful gone notice and return a restored missing session to its agent list (UT-087)", async () => {
    queryState.data = undefined;
    queryState.error = new SessionNotFoundError("sess-1");
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
});
