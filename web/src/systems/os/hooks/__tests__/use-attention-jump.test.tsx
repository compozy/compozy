// Suite: attention jump coordination
// Invariant: a cross-destination attention activation focuses the session's destination
// first — its project, or User for a session without one — and opens the session only after
// that partition's window manager is live. A User session resolves through `@user`.
// Owning layer: OS navigation hook. No prior suite owned this coordination.
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { ensureQueryData, focusDestination, notifyUser } = vi.hoisted(() => ({
  ensureQueryData: vi.fn(),
  focusDestination: vi.fn(),
  notifyUser: vi.fn(),
}));
type TestDataScope = { kind: "project"; workspaceId: string } | { kind: "user" } | { kind: "all" };
let dataScope: TestDataScope = { kind: "project", workspaceId: "ws-alpha" };
let commandWorkspaceId: string | null = "ws-alpha";
const userOpen = vi.fn(() => Promise.resolve("window-session"));
const userOpenCreatedSession = vi.fn(() => Promise.resolve("window-session"));

vi.mock("@tanstack/react-query", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-query")>();
  return { ...actual, useQueryClient: () => ({ ensureQueryData }) };
});

vi.mock("@/lib/user-feedback", () => ({ notifyUser }));

vi.mock("@/systems/session", () => ({
  SessionNotFoundError: class SessionNotFoundError extends Error {},
  sessionDetailOptions: (workspaceId: string, sessionId: string) => ({
    queryKey: ["sessions", "workspace", workspaceId, "detail", sessionId],
  }),
}));

vi.mock("@/systems/workspace", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/workspace")>();
  return {
    ...actual,
    focusDestination,
    useDataScope: () => dataScope,
  };
});

vi.mock("../use-desktop", () => ({
  useDesktop: (selector: (state: unknown) => unknown) =>
    selector({
      snapshot: commandWorkspaceId === null ? null : { workspaceId: commandWorkspaceId },
      client: commandWorkspaceId === null ? null : {},
      hydration: commandWorkspaceId === null ? "pending" : "live",
      connectionStatus: commandWorkspaceId === null ? "disconnected" : "connected",
    }),
}));

vi.mock("../use-os-shell", () => ({
  useOsShell: () => ({ coordinator: { userOpen, userOpenCreatedSession } }),
}));

import { useAttentionJump } from "../use-attention-jump";

describe("useAttentionJump", () => {
  beforeEach(() => {
    dataScope = { kind: "project", workspaceId: "ws-alpha" };
    commandWorkspaceId = "ws-alpha";
    vi.clearAllMocks();
    ensureQueryData.mockResolvedValue({ agent_name: "claude" });
  });

  it("Should wait for the target workspace manager before opening the session", async () => {
    const { result, rerender } = renderHook(() => useAttentionJump());

    act(() => {
      result.current({
        sessionId: "sess-beta",
        agentName: "claude",
        workspaceId: "ws-beta",
      });
    });

    expect(focusDestination).toHaveBeenCalledExactlyOnceWith({
      kind: "project",
      workspaceId: "ws-beta",
    });
    expect(userOpen).not.toHaveBeenCalled();

    dataScope = { kind: "project", workspaceId: "ws-beta" };
    rerender();
    expect(userOpen).not.toHaveBeenCalled();

    commandWorkspaceId = "ws-beta";
    rerender();

    await waitFor(() => {
      expect(userOpen).toHaveBeenCalledExactlyOnceWith({
        app: "session",
        instanceKey: "sess-beta",
        route: {
          pathname: "/agents/claude/sessions/sess-beta",
          search: {},
        },
      });
    });
  });

  it("Should focus User before opening a User session from a project", async () => {
    const { result, rerender } = renderHook(() => useAttentionJump());

    act(() => {
      result.current({ sessionId: "sess-user", agentName: "claude", workspaceId: "" });
    });

    expect(focusDestination).toHaveBeenCalledExactlyOnceWith({ kind: "user" });
    expect(notifyUser).not.toHaveBeenCalled();

    dataScope = { kind: "user" };
    rerender();
    expect(userOpen).not.toHaveBeenCalled();

    commandWorkspaceId = "user";
    rerender();
    await waitFor(() => {
      expect(userOpen).toHaveBeenCalledExactlyOnceWith({
        app: "session",
        instanceKey: "sess-user",
        route: { pathname: "/agents/claude/sessions/sess-user", search: {} },
      });
    });
  });

  it("Should resolve a User session's agent through @user and open it on the User desktop", async () => {
    dataScope = { kind: "user" };
    commandWorkspaceId = "user";
    const { result } = renderHook(() => useAttentionJump());

    act(() => {
      result.current({ sessionId: "sess-user", workspaceId: "" });
    });

    await waitFor(() => expect(userOpen).toHaveBeenCalledOnce());
    expect(ensureQueryData).toHaveBeenCalledWith({
      queryKey: ["sessions", "workspace", "@user", "detail", "sess-user"],
    });
    expect(focusDestination).not.toHaveBeenCalled();
  });

  it("Should land a just-created session through the created-session policy", async () => {
    const { result } = renderHook(() => useAttentionJump());

    act(() => {
      result.current({
        sessionId: "sess-new",
        agentName: "claude",
        workspaceId: "ws-alpha",
        desktopId: "desktop-origin",
        created: true,
      });
    });

    await waitFor(() => {
      expect(userOpenCreatedSession).toHaveBeenCalledExactlyOnceWith({
        app: "session",
        instanceKey: "sess-new",
        route: { pathname: "/agents/claude/sessions/sess-new", search: {} },
        desktopId: "desktop-origin",
      });
    });
    expect(userOpen).not.toHaveBeenCalled();
  });

  it("Should resolve the sending session before opening an agent notification", async () => {
    const { result } = renderHook(() => useAttentionJump());

    act(() => {
      result.current({ sessionId: "sess-alpha", workspaceId: "ws-alpha" });
    });

    await waitFor(() => {
      expect(userOpen).toHaveBeenCalledExactlyOnceWith({
        app: "session",
        instanceKey: "sess-alpha",
        route: {
          pathname: "/agents/claude/sessions/sess-alpha",
          search: {},
        },
      });
    });
    expect(ensureQueryData).toHaveBeenCalledWith({
      queryKey: ["sessions", "workspace", "ws-alpha", "detail", "sess-alpha"],
    });
    expect(notifyUser).not.toHaveBeenCalled();
  });

  it("Should keep a requested placement through agent resolution", async () => {
    const { result } = renderHook(() => useAttentionJump());

    act(() => {
      result.current({ sessionId: "sess-alpha", workspaceId: "ws-alpha", placement: "split" });
    });

    await waitFor(() => {
      expect(userOpen).toHaveBeenCalledExactlyOnceWith({
        app: "session",
        instanceKey: "sess-alpha",
        route: {
          pathname: "/agents/claude/sessions/sess-alpha",
          search: {},
        },
        placement: "split",
      });
    });
  });

  it("Should open a new session on the desktop it was started from", async () => {
    const { result } = renderHook(() => useAttentionJump());

    act(() => {
      result.current({
        sessionId: "sess-alpha",
        agentName: "claude",
        workspaceId: "ws-alpha",
        desktopId: "desktop-origin",
      });
    });

    await waitFor(() => {
      expect(userOpen).toHaveBeenCalledExactlyOnceWith({
        app: "session",
        instanceKey: "sess-alpha",
        route: {
          pathname: "/agents/claude/sessions/sess-alpha",
          search: {},
        },
        desktopId: "desktop-origin",
      });
    });
  });
});
