// Suite: Network window-location and workspace synchronization
// Invariants: nested Network locations are parsed from WM state; an explicit
// workspace switch wins over a stale detail location; root follows the active workspace.
// Boundary IN: useNetworkRouteShell location/workspace synchronization behavior.
// Boundary OUT: Link history writes and rendered Network surfaces, covered by router and component suites.

import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { mockNavigate, mockSetActiveWorkspaceId, mockUseNetworkPage } = vi.hoisted(() => ({
  mockNavigate: vi.fn(),
  mockSetActiveWorkspaceId: vi.fn<(workspaceId: string | null) => void>(),
  mockUseNetworkPage: vi.fn(),
}));

let mockActiveWorkspaceId: string | null = "ws_alpha";
let mockSelectedWorkspaceId: string | null = "ws_alpha";

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({
    activeWorkspaceId: mockActiveWorkspaceId,
    selectedWorkspaceId: mockSelectedWorkspaceId,
    setActiveWorkspaceId: mockSetActiveWorkspaceId,
  }),
}));

vi.mock("../use-network-page", () => ({
  useNetworkPage: (...args: unknown[]) => {
    mockUseNetworkPage(...args);
    return {
      channels: [
        {
          channel: "copy",
          last_activity_at: "2026-05-14T02:00:00Z",
        },
      ],
      firstVisibleChannel: { channel: "copy" },
      recents: [],
    };
  },
}));

import { useNetworkRouteShell } from "../use-network-route-shell";

describe("useNetworkRouteShell", () => {
  beforeEach(() => {
    mockActiveWorkspaceId = "ws_alpha";
    mockSelectedWorkspaceId = "ws_alpha";
    mockNavigate.mockReset();
    mockSetActiveWorkspaceId.mockReset();
    mockUseNetworkPage.mockReset();
  });

  it("Should parse a nested thread location from WM state and adopt its deep-linked workspace", async () => {
    mockActiveWorkspaceId = "ws_beta";
    mockSelectedWorkspaceId = null;

    const { result } = renderHook(() =>
      useNetworkRouteShell({
        active: true,
        location: {
          pathname: "/network/ws_alpha/copy/threads/thread%2Flaunch",
          search: { view: "full" },
        },
        navigation: { push: mockNavigate },
      })
    );

    expect(mockUseNetworkPage).toHaveBeenCalledWith("ws_alpha", { enabled: true });
    expect(result.current.activeTab).toBe("threads");
    expect(result.current.activeThreadId).toBe("thread/launch");
    expect(result.current.activeDirectId).toBeNull();
    expect(result.current.location.view).toBe("full");

    await waitFor(() => {
      expect(mockSetActiveWorkspaceId).toHaveBeenCalledWith("ws_alpha");
    });
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("Should parse direct and activity locations without router params", () => {
    const { result, rerender } = renderHook(
      ({ pathname }: { pathname: string }) =>
        useNetworkRouteShell({
          active: true,
          location: { pathname, search: {} },
          navigation: { push: mockNavigate },
        }),
      { initialProps: { pathname: "/network/ws_alpha/copy/directs/direct-1" } }
    );

    expect(result.current.activeTab).toBe("directs");
    expect(result.current.activeDirectId).toBe("direct-1");

    rerender({ pathname: "/network/ws_alpha/copy/activity" });
    expect(result.current.activeTab).toBe("activity");
    expect(result.current.activeDirectId).toBeNull();
  });

  it("Should not let a stale detail location overwrite an explicit workspace switch", async () => {
    mockActiveWorkspaceId = "ws_beta";
    mockSelectedWorkspaceId = "ws_beta";

    renderHook(() =>
      useNetworkRouteShell({
        active: true,
        location: { pathname: "/network/ws_alpha/copy/threads", search: {} },
        navigation: { push: mockNavigate },
      })
    );

    expect(mockUseNetworkPage).toHaveBeenCalledWith("ws_alpha", { enabled: false });
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith({ pathname: "/network", search: {} });
    });
    expect(mockSetActiveWorkspaceId).not.toHaveBeenCalledWith("ws_alpha");
  });

  it("Should deepen the root location into the active workspace first channel", async () => {
    renderHook(() =>
      useNetworkRouteShell({
        active: true,
        location: { pathname: "/network", search: {} },
        navigation: { push: mockNavigate },
      })
    );

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith({
        pathname: "/network/ws_alpha/copy/threads",
        search: {},
      });
    });
  });

  it("Should leave a background root location inert when its channels resolve", async () => {
    renderHook(() =>
      useNetworkRouteShell({
        active: false,
        location: { pathname: "/network", search: {} },
        navigation: { push: mockNavigate },
      })
    );

    await waitFor(() => expect(mockUseNetworkPage).toHaveBeenCalled());
    expect(mockNavigate).not.toHaveBeenCalled();
    expect(mockSetActiveWorkspaceId).not.toHaveBeenCalled();
  });
});
