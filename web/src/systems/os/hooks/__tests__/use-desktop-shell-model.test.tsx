// Suite: desktop shell model boundary
// Invariant: the shell model mounts before OsShellContext because DesktopShell owns that provider.
// Boundary IN: useDesktopShellModel's dependency graph.
// Boundary OUT: query transports, window-manager projection, and rendered desktop chrome.
import { renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  sessionCreate: { store: {} },
  agentCreate: { open: false },
}));

vi.mock("@/systems/agent", () => ({
  useAgentCreateDialog: () => mocks.agentCreate,
  useAgents: () => ({ data: [], isLoading: false, error: null }),
}));

vi.mock("@/systems/session", () => ({
  useSessionCatalogStreams: () => "connected",
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
  useWorktreeCatalogStream: () => "connected",
  useWorktreeListings: () => ({}),
}));

import { useDesktopShellModel } from "../use-desktop-shell-model";

describe("useDesktopShellModel", () => {
  it("Should mount before the desktop shell context exists", () => {
    const { result } = renderHook(() => useDesktopShellModel());

    expect(result.current.activeWorkspaceId).toBeNull();
    expect(result.current.worktreeListing).toBeUndefined();
  });
});
