// Suite: OS session-window live ownership
// Invariant: only the focused, non-minimized session window on the active desktop owns a live tail.
// Owning layer: the OS session-window controller.
import { render, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SessionNotFoundError } from "@/systems/session";

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
      route: { pathname: "/agents/qa-agent/sessions/sess-1", search: {} },
    },
  },
};
const sessionWindowViewSpy = vi.fn((_props: Record<string, unknown>) => null);
const userClose = vi.fn(() => Promise.resolve(true));
const userOpen = vi.fn(() => Promise.resolve(null));
const queryState: {
  data: typeof session | undefined;
  error: Error | null;
  isLoading: boolean;
} = { data: session, error: null, isLoading: false };

vi.mock("@tanstack/react-query", () => ({
  useQuery: () => queryState,
}));

vi.mock("@/systems/session", () => ({
  SessionNotFoundError: class SessionNotFoundError extends Error {
    constructor(sessionId: string) {
      super(`Session not found: ${sessionId}`);
    }
  },
  sessionDetailOptions: () => ({}),
}));

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({ activeWorkspaceId: "ws-1" }),
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
  SessionWindowNotice: () => null,
  SessionWindowView: (props: Record<string, unknown>) => sessionWindowViewSpy(props),
}));

import { SessionWindow } from "../session-window";

describe("SessionWindow", () => {
  beforeEach(() => {
    desktop.activeDesktopId = "desktop-1";
    desktop.focusedId = "session:sess-1";
    desktop.windows["session:sess-1"].desktopId = "desktop-1";
    desktop.windows["session:sess-1"].minimized = false;
    queryState.data = session;
    queryState.error = null;
    queryState.isLoading = false;
    sessionWindowViewSpy.mockClear();
    userClose.mockClear();
    userOpen.mockClear();
  });

  it.each([
    {
      name: "another window gains focus",
      arrange: () => {
        desktop.focusedId = "app:marketplace";
      },
    },
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
