// Suite: OS session-window live ownership
// Invariant: only the focused, non-minimized session window on the active desktop owns a live tail.
// Owning layer: the OS session-window controller.
import { render } from "@testing-library/react";
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
      route: { pathname: "/agents/qa-agent/sessions/sess-1", search: {} },
    },
  },
};
const sessionWindowViewSpy = vi.fn((_props: Record<string, unknown>) => null);

vi.mock("@tanstack/react-query", () => ({
  useQuery: () => ({ data: session, error: null, isLoading: false }),
}));

vi.mock("@/systems/session", () => ({
  SessionNotFoundError: class SessionNotFoundError extends Error {},
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
      userClose: vi.fn(),
      userOpen: vi.fn(),
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
    sessionWindowViewSpy.mockClear();
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
});
