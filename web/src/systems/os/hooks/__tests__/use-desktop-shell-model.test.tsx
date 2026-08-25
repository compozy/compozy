// Suite: desktop shell model boundary
// Invariant: the shell model mounts before OsShellContext and independently budgets optional
// worktree traffic and continuity-critical session events.
// Boundary IN: useDesktopShellModel's dependency graph.
// Boundary OUT: query transports, window-manager projection, and rendered desktop chrome.
import { renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  sessionCreate: { store: {} },
  sessionCatalogStreams: vi.fn((_options: unknown) => "connected"),
  agentCreate: { open: false },
  worktreeCatalogStream: vi.fn((_workspaces: unknown, _options: unknown) => "connected"),
}));

vi.mock("@/systems/agent", () => ({
  useAgentCreateDialog: () => mocks.agentCreate,
  useAgents: () => ({ data: [], isLoading: false, error: null }),
}));

vi.mock("@/systems/session", () => ({
  useSessionCatalogStreams: (options: unknown) => mocks.sessionCatalogStreams(options),
  useSessionCreateDialogController: () => mocks.sessionCreate,
}));

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({
    workspaces: [],
    registeredWorkspaces: [],
    hasWorkspaces: false,
    activeWorkspace: null,
    activeWorkspaceId: null,
    setActiveWorkspaceId: vi.fn(),
    isLoading: false,
    isError: false,
  }),
  useUserHomeDir: () => "/operator",
  useWorkspace: () => ({ data: undefined, isLoading: false, error: null }),
  useWorktreeCatalogStream: (workspaces: unknown, options: unknown) =>
    mocks.worktreeCatalogStream(workspaces, options),
  useWorktrees: () => ({ data: undefined }),
}));

import { useDesktopShellModel } from "../use-desktop-shell-model";
import { useActiveWorkspace } from "@/systems/workspace";

describe("useDesktopShellModel", () => {
  it("Should mount before the desktop shell context exists", () => {
    const { result } = renderHook(() => useDesktopShellModel(useActiveWorkspace()));

    expect(result.current.activeWorkspaceId).toBeNull();
    expect(result.current.worktreeListing).toBeUndefined();
  });

  it("Should budget optional and continuity streams independently", () => {
    renderHook(() =>
      useDesktopShellModel(useActiveWorkspace(), {
        backgroundStreamsEnabled: false,
        continuityStreamsEnabled: true,
      })
    );

    expect(mocks.worktreeCatalogStream).toHaveBeenLastCalledWith([], { enabled: false });
    expect(mocks.sessionCatalogStreams).toHaveBeenLastCalledWith(
      expect.objectContaining({ enabled: true })
    );
  });
});
