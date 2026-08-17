// Suite: attention jump coordination
// Invariant: a cross-workspace attention activation changes workspace first
// and opens the session only after that workspace's window manager is live.
// Owning layer: OS navigation hook. No prior suite owned this coordination.
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { ensureQueryData, notifyUser } = vi.hoisted(() => ({
  ensureQueryData: vi.fn(),
  notifyUser: vi.fn(),
}));
let runtimeWorkspaceId = "ws-alpha";
let commandWorkspaceId: string | null = "ws-alpha";
const setActiveWorkspaceId = vi.fn();
const userOpen = vi.fn(() => Promise.resolve("window-session"));

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

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({
    runtimeWorkspaceId,
    setActiveWorkspaceId,
  }),
}));

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
  useOsShell: () => ({ coordinator: { userOpen } }),
}));

import { useAttentionJump } from "../use-attention-jump";

describe("useAttentionJump", () => {
  beforeEach(() => {
    runtimeWorkspaceId = "ws-alpha";
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

    expect(setActiveWorkspaceId).toHaveBeenCalledExactlyOnceWith("ws-beta");
    expect(userOpen).not.toHaveBeenCalled();

    runtimeWorkspaceId = "ws-beta";
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
});
