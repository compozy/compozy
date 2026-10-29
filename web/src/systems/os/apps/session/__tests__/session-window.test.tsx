// Suite: OS session-window live ownership and profile resolution
// Invariant: every visible session window on the active desktop owns a live tail, while operator
// presence additionally requires the shell window and browser document to hold focus; and the
// window resolves a session through the profile-enforced read before deciding it is gone.
// A window's data scope (its binding, else its partition: a project, or User) hides another
// scope's history without retiring the shared window; an All projects catalog window hides none.
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
const sessionWindowEmptySpy = vi.fn((_props: Record<string, unknown>) => null);
const queryOptionsSpy = vi.fn((_options: Record<string, unknown>) => undefined);
const userRetireSession = vi.fn(() => Promise.resolve(true));
const userClose = vi.fn(() => Promise.resolve(true));
const userOpen = vi.fn(() => Promise.resolve(null));
type TestDataScope = { kind: "project"; workspaceId: string } | { kind: "user" } | { kind: "all" };
const workspace: { dataScope: TestDataScope | null } = {
  dataScope: { kind: "project", workspaceId: "ws-1" },
};
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

const scopedDetailSpy = vi.fn((..._args: unknown[]) => ({}));
const foreignState: { current: Record<string, unknown> } = { current: { status: "disabled" } };
const profileScope = { aggregate: false };
let deletionPending = false;
let deletedByOperator = false;
const toastError = vi.hoisted(() => vi.fn());
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
    useSessionDeletionPending: () => deletionPending,
    useSessionDeletedByOperator: () => deletedByOperator,
  };
});

vi.mock("sonner", () => ({ toast: { error: toastError } }));

vi.mock("@/systems/session/hooks/use-foreign-profile-session", () => ({
  useForeignProfileSession: () => foreignState.current,
}));

vi.mock("@/systems/profiles/hooks/use-profile-read-scope", () => ({
  useProfileReadScope: () => ({
    key: "marketing",
    aggregate: profileScope.aggregate,
    params: { profile: "marketing" },
  }),
}));

vi.mock("../session-profile-owner-notice", () => ({
  SessionProfileOwnerNotice: (props: Record<string, unknown>) => ownerViewSpy(props),
}));

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => ({ activeWorkspaceId: "ws-1", ...workspace }),
}));

vi.mock("@/systems/workspace/hooks/use-window-data-workspace", () => ({
  // The window's own data scope: its binding, else its partition.
  useWindowDataWorkspace: () => workspace.dataScope,
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
      userRetireSession,
      userClose,
      userOpen,
    },
  }),
}));

vi.mock("../session-window-empty", () => ({
  SessionWindowEmpty: (props: Record<string, unknown>) => sessionWindowEmptySpy(props),
}));

vi.mock("../session-window-view", () => ({
  SessionWindowNotice: (props: Record<string, unknown>) => sessionWindowNoticeSpy(props),
  SessionWindowView: (props: Record<string, unknown>) => sessionWindowViewSpy(props),
}));

import { SessionWindow } from "../session-window";
import { SessionNotFoundError } from "@/systems/session";
import { SessionApiError } from "@/systems/session/adapters/session-api-errors";

describe("SessionWindow", () => {
  beforeEach(() => {
    desktop.activeDesktopId = "desktop-1";
    desktop.focusedId = "session:sess-1";
    desktop.windows["session:sess-1"].desktopId = "desktop-1";
    desktop.windows["session:sess-1"].minimized = false;
    desktop.windows["session:sess-1"].stackActive = true;
    desktop.windows["session:sess-1"].route.pathname = "/agents/qa-agent/sessions/sess-1";
    queryState.data = session;
    queryState.error = null;
    queryState.isLoading = false;
    queryState.isError = false;
    workspace.dataScope = { kind: "project", workspaceId: "ws-1" };
    documentActivity.active = true;
    sessionWindowViewSpy.mockClear();
    sessionWindowNoticeSpy.mockClear();
    sessionWindowEmptySpy.mockClear();
    queryOptionsSpy.mockClear();
    scopedDetailSpy.mockClear();
    ownerViewSpy.mockClear();
    foreignState.current = { status: "disabled" };
    profileScope.aggregate = false;
    deletionPending = false;
    deletedByOperator = false;
    toastError.mockClear();
    userRetireSession.mockClear();
    userRetireSession.mockResolvedValue(true);
    userClose.mockClear();
    userClose.mockResolvedValue(true);
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

  // Invariant: an in-place retarget reaches the view one deferred render behind the shell's
  // route, so it is a transition the keyed boundary can animate. The view never sees a mix of
  // two sessions: it keeps the current one until the next is the only thing it is given.
  it("Should hand the view the retargeted session only on the deferred render", () => {
    const view = render(<SessionWindow windowId="session:sess-1" />);
    sessionWindowViewSpy.mockClear();

    desktop.windows["session:sess-1"].route.pathname = "/agents/qa-agent/sessions/sess-2";
    view.rerender(<SessionWindow windowId="session:sess-1" />);

    const viewedIds = sessionWindowViewSpy.mock.calls.map(([props]) => props.id);
    expect(viewedIds[0]).toBe("sess-1");
    expect(viewedIds.at(-1)).toBe("sess-2");
    expect(viewedIds).not.toContain(undefined);
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

  it("Should render a session from its authoritative workspace under All projects", () => {
    workspace.dataScope = { kind: "all" };

    render(<SessionWindow windowId="session:sess-1" />);

    expect(sessionWindowViewSpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ session, workspaceId: "ws-1" })
    );
    expect(useSessionPresenceSpy).toHaveBeenLastCalledWith("ws-1", "sess-1", true);
    expect(sessionWindowNoticeSpy).not.toHaveBeenCalled();
  });

  it("Should keep User history in the shared layout while this document views a project", async () => {
    queryState.data = { ...session, workspace_id: "" };
    documentActivity.active = false;
    const view = render(<SessionWindow windowId="session:sess-1" />);

    await waitFor(() => expect(sessionWindowNoticeSpy).toHaveBeenCalled());
    expect(useSessionPresenceSpy).toHaveBeenLastCalledWith("", "sess-1", false);
    expect(sessionWindowViewSpy).not.toHaveBeenCalled();
    expect(userRetireSession).not.toHaveBeenCalled();

    workspace.dataScope = { kind: "user" };
    view.rerender(<SessionWindow windowId="session:sess-1" />);
    expect(sessionWindowViewSpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ session: queryState.data, workspaceId: "" })
    );
    expect(userRetireSession).not.toHaveBeenCalled();
  });

  it("Should hide a project's session on the User desktop without retiring the window", async () => {
    workspace.dataScope = { kind: "user" };
    render(<SessionWindow windowId="session:sess-1" />);

    await waitFor(() => expect(sessionWindowNoticeSpy).toHaveBeenCalled());
    expect(sessionWindowViewSpy).not.toHaveBeenCalled();
    expect(userRetireSession).not.toHaveBeenCalled();
  });

  it.each([404, 410])(
    "Should retire a restored missing session after HTTP %s (UT-087)",
    async status => {
      queryState.data = undefined;
      queryState.error = new SessionApiError("Session unavailable", status, "sess-1");
      queryState.isError = true;
      foreignState.current = { status: "missing" };
      render(<SessionWindow windowId="session:sess-1" />);

      await waitFor(() =>
        expect(userRetireSession).toHaveBeenCalledExactlyOnceWith("session:sess-1")
      );
      expect(userClose).not.toHaveBeenCalled();
      expect(userOpen).not.toHaveBeenCalled();
      expect(sessionWindowEmptySpy).toHaveBeenLastCalledWith(
        expect.objectContaining({ windowId: "session:sess-1" })
      );
    }
  );

  it.each([404, 410])(
    "Should retire an aggregate HTTP %s without rendering the missing-session notice",
    async status => {
      profileScope.aggregate = true;
      queryState.data = undefined;
      queryState.error = new SessionApiError("Session unavailable", status, "sess-1");
      queryState.isError = true;
      render(<SessionWindow windowId="session:sess-1" />);
      await waitFor(() =>
        expect(userRetireSession).toHaveBeenCalledExactlyOnceWith("session:sess-1")
      );
      expect(sessionWindowEmptySpy).toHaveBeenCalled();
      expect(sessionWindowNoticeSpy).not.toHaveBeenCalled();
    }
  );

  it.each([false, true])("Should preserve transport errors with aggregate=%s", aggregate => {
    profileScope.aggregate = aggregate;
    queryState.data = undefined;
    queryState.error = new Error("Network unavailable");
    queryState.isError = true;
    render(<SessionWindow windowId="session:sess-1" />);
    expect(userRetireSession).not.toHaveBeenCalled();
    expect(sessionWindowEmptySpy).not.toHaveBeenCalled();
  });

  it("Should suspend window reads during deletion and restore them after rollback", () => {
    deletionPending = true;
    const { rerender } = render(<SessionWindow windowId="session:sess-1" />);
    expect(scopedDetailSpy).toHaveBeenLastCalledWith(
      "sess-1",
      { profile: "marketing" },
      expect.objectContaining({ enabled: false, liveTail: false })
    );
    expect(useSessionPresenceSpy).toHaveBeenLastCalledWith("ws-1", "sess-1", false);
    deletionPending = false;
    rerender(<SessionWindow windowId="session:sess-1" />);
    expect(scopedDetailSpy).toHaveBeenLastCalledWith(
      "sess-1",
      { profile: "marketing" },
      expect.objectContaining({ enabled: true, liveTail: true })
    );
    expect(useSessionPresenceSpy).toHaveBeenLastCalledWith("ws-1", "sess-1", true);
  });

  it("Should defer remote retirement while an operator deletion owns the window", async () => {
    profileScope.aggregate = true;
    deletionPending = true;
    queryState.data = undefined;
    queryState.error = new SessionNotFoundError("sess-1");
    queryState.isError = true;
    const { rerender } = render(<SessionWindow windowId="session:sess-1" />);
    expect(userRetireSession).not.toHaveBeenCalled();
    expect(sessionWindowEmptySpy).toHaveBeenCalled();
    deletionPending = false;
    rerender(<SessionWindow windowId="session:sess-1" />);
    await waitFor(() => expect(userRetireSession).toHaveBeenCalledOnce());
  });

  // #952: rename/delete run through the session list's row lifecycle, which retires the window
  // itself. If that retarget was refused, the window still stops reading the session and retires
  // quietly: an operator-deleted session is never announced as missing.
  it("Should retire a window whose session this client deleted without reporting it missing", async () => {
    deletedByOperator = true;
    render(<SessionWindow windowId="session:sess-1" />);
    await waitFor(() =>
      expect(userRetireSession).toHaveBeenCalledExactlyOnceWith("session:sess-1")
    );
    expect(scopedDetailSpy).toHaveBeenLastCalledWith(
      "sess-1",
      { profile: "marketing" },
      expect.objectContaining({ liveTail: false })
    );
    expect(sessionWindowEmptySpy).toHaveBeenCalled();
    expect(toastError).not.toHaveBeenCalled();
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
    expect(userRetireSession).not.toHaveBeenCalled();
    expect(userClose).not.toHaveBeenCalled();
    expect(userOpen).not.toHaveBeenCalled();
    expect(useSessionPresenceSpy).toHaveBeenLastCalledWith("ws-1", "sess-1", false);
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
    expect(userRetireSession).not.toHaveBeenCalled();
    expect(userClose).not.toHaveBeenCalled();
    expect(sessionWindowNoticeSpy).not.toHaveBeenCalled();
  });

  it("Should hide a foreign session from another workspace without retiring the shared window", async () => {
    queryState.data = undefined;
    queryState.error = new SessionNotFoundError("sess-1");
    queryState.isError = true;
    foreignState.current = {
      status: "found",
      session: { ...session, workspace_id: "ws-2" },
      owner: { id: "01J9CONSULTING000000000000", name: "consulting", archived: false },
    };

    render(<SessionWindow windowId="session:sess-1" />);

    expect(useSessionPresenceSpy).toHaveBeenLastCalledWith("ws-2", "sess-1", false);
    expect(ownerViewSpy).not.toHaveBeenCalled();
    await waitFor(() => expect(sessionWindowNoticeSpy).toHaveBeenCalled());
    expect(userRetireSession).not.toHaveBeenCalled();
    expect(userClose).not.toHaveBeenCalled();
    expect(sessionWindowViewSpy).not.toHaveBeenCalled();
  });

  it("Should surface a failed owner lookup instead of claiming the session is gone", async () => {
    queryState.data = undefined;
    queryState.error = new SessionNotFoundError("sess-1");
    queryState.isError = true;
    foreignState.current = { status: "error", error: new Error("Owner lookup failed") };

    render(<SessionWindow windowId="session:sess-1" />);

    await waitFor(() =>
      expect(sessionWindowNoticeSpy).toHaveBeenLastCalledWith({
        title: "Couldn't open this session",
        detail: "Owner lookup failed",
      })
    );
    expect(userRetireSession).not.toHaveBeenCalled();
    expect(userClose).not.toHaveBeenCalled();
  });
});
