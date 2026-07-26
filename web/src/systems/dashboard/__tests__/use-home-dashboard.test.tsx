import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { statusFixture } from "@/systems/status/mocks/fixtures";

import { makeHomeOverview } from "../mocks/fixtures";

const getHomeOverview = vi.fn();
const getHomeActivity = vi.fn();
const fetchStatus = vi.fn();

vi.mock("../adapters/overview-api", async importOriginal => {
  const actual = await importOriginal<typeof import("../adapters/overview-api")>();
  return {
    ...actual,
    getHomeOverview: (...args: Parameters<typeof actual.getHomeOverview>) =>
      getHomeOverview(...args),
    getHomeActivity: (...args: Parameters<typeof actual.getHomeActivity>) =>
      getHomeActivity(...args),
  };
});

// The home dir now flows from the awaited daemon status query (not the Zustand
// store), so drive it through the real status adapter.
vi.mock("@/systems/status/adapters/daemon-api", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/status/adapters/daemon-api")>();
  return {
    ...actual,
    fetchStatus: (...args: Parameters<typeof actual.fetchStatus>) => fetchStatus(...args),
  };
});

const activeWorkspaceState = {
  activeWorkspace: { id: "ws-home", root_dir: "/Users/tester", name: "home" } as unknown,
  activeWorkspaceId: "ws-home",
  isLoading: false,
};

vi.mock("@/systems/workspace", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/workspace")>();
  return {
    ...actual,
    useActiveWorkspace: () => activeWorkspaceState,
  };
});

vi.mock("@/systems/status", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/status")>();
  return {
    ...actual,
    useDaemonHealth: () => ({
      connectionStatus: "connected" as const,
      health: undefined,
      isInitialLoading: false,
    }),
  };
});

import { useHomeDashboard } from "../hooks/use-home-dashboard";

function wrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useHomeDashboard", () => {
  beforeEach(() => {
    getHomeOverview.mockReset();
    getHomeActivity.mockReset();
    getHomeActivity.mockResolvedValue([]);
    fetchStatus.mockReset();
    // Full status payload (useHomeSystem reads health/automation/memory) with the
    // home dir aligned to the active workspace root so scope resolves to global.
    fetchStatus.mockResolvedValue({
      ...statusFixture,
      daemon: { ...statusFixture.daemon, user_home_dir: "/Users/tester" },
    });
    activeWorkspaceState.activeWorkspace = {
      id: "ws-home",
      root_dir: "/Users/tester",
      name: "home",
    };
    activeWorkspaceState.activeWorkspaceId = "ws-home";
    activeWorkspaceState.isLoading = false;
  });

  it("Should resolve the home workspace to the global scope and expose the overview", async () => {
    getHomeOverview.mockResolvedValue(makeHomeOverview());

    const { result } = renderHook(() => useHomeDashboard(), { wrapper: wrapper() });
    expect(result.current.overviewStatus).toBe("loading");

    await waitFor(() => {
      expect(result.current.overviewStatus).toBe("ready");
    });
    expect(result.current.scope.workspaceParam).toBe("");
    expect(result.current.overview?.attention.total).toBe(2);
    expect(getHomeOverview).toHaveBeenCalledWith(
      expect.objectContaining({ workspace: undefined, usageWindow: 30 }),
      expect.anything()
    );
  });

  it("Should scope the overview to a project workspace", async () => {
    activeWorkspaceState.activeWorkspace = {
      id: "ws-proj",
      root_dir: "/Users/tester/dev/proj",
      name: "proj",
    };
    activeWorkspaceState.activeWorkspaceId = "ws-proj";
    getHomeOverview.mockResolvedValue(makeHomeOverview());

    const { result } = renderHook(() => useHomeDashboard(), { wrapper: wrapper() });
    await waitFor(() => {
      expect(result.current.overviewStatus).toBe("ready");
    });
    expect(result.current.scope.workspaceParam).toBe("ws-proj");
    expect(getHomeOverview).toHaveBeenCalledWith(
      expect.objectContaining({ workspace: "ws-proj" }),
      expect.anything()
    );
  });

  it("Should never issue a global overview or activity read for a project workspace on first mount", async () => {
    // Workspace-isolation guard: the queries stay disabled until the awaited home
    // dir settles the scope to the project, so the daemon never sees an
    // undefined-workspace (whole-system) overview/activity read during mount.
    activeWorkspaceState.activeWorkspace = {
      id: "ws-proj",
      root_dir: "/Users/tester/dev/proj",
      name: "proj",
    };
    activeWorkspaceState.activeWorkspaceId = "ws-proj";
    getHomeOverview.mockResolvedValue(makeHomeOverview());

    const { result } = renderHook(() => useHomeDashboard(), { wrapper: wrapper() });
    await waitFor(() => {
      expect(result.current.overviewStatus).toBe("ready");
    });

    for (const [filter] of getHomeOverview.mock.calls) {
      expect(filter).toMatchObject({ workspace: "ws-proj" });
    }
    for (const [filter] of getHomeActivity.mock.calls) {
      expect(filter).toMatchObject({ workspace_id: "ws-proj" });
    }
    expect(getHomeOverview).not.toHaveBeenCalledWith(
      expect.objectContaining({ workspace: undefined }),
      expect.anything()
    );
    expect(getHomeActivity).not.toHaveBeenCalledWith(
      expect.objectContaining({ workspace_id: undefined }),
      expect.anything()
    );
  });

  it("Should surface an error status with the adapter message", async () => {
    getHomeOverview.mockRejectedValue(new Error("daemon unavailable"));

    const { result } = renderHook(() => useHomeDashboard(), { wrapper: wrapper() });
    await waitFor(() => {
      expect(result.current.overviewStatus).toBe("error");
    });
    expect(result.current.overviewErrorMessage).toBe("daemon unavailable");
  });
});
