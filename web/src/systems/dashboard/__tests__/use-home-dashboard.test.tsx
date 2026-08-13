// Suite: home dashboard model
// Invariant: dashboard sections expose current scoped data without hidden-window refresh work.
// Boundary IN: dashboard query composition, liveness, and derived view state.
// Boundary OUT: card rendering and task transport behavior, owned by their domain suites.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { statusFixture } from "@/systems/status/mocks";

import { makeHomeOverview } from "../mocks/fixtures";

const getHomeOverview = vi.fn();
const getHomeActivity = vi.fn();
const fetchStatus = vi.fn();
const useHomeLive = vi.fn();
const useHomeWorkingNow = vi.fn();

vi.mock("../hooks/use-home-live", () => ({
  useHomeLive: (options: unknown) => useHomeLive(options),
}));

vi.mock("../hooks/use-home-working-now", () => ({
  useHomeWorkingNow: (...args: unknown[]) => useHomeWorkingNow(...args),
}));

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

// The home dir now flows from the awaited daemon status query, not a client store,
// so drive it through the real status adapter.
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
  runtimeWorkspaceId: "ws-home",
  scope: "global" as "global" | "workspace",
  isLoading: false,
};

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => activeWorkspaceState,
}));

vi.mock("@/systems/status/hooks/use-daemon-health", () => ({
  useDaemonHealth: () => ({
    connectionStatus: "connected" as const,
    health: undefined,
    isInitialLoading: false,
  }),
}));

import { homePrefsStore } from "../hooks/use-home-prefs-store";
import { useHomeDashboard } from "../hooks/use-home-dashboard";

function wrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useHomeDashboard", () => {
  beforeEach(() => {
    homePrefsStore.trigger.usageWindowSelected({ usageWindow: 30 });
    homePrefsStore.trigger.systemPanelClosed();
    getHomeOverview.mockReset();
    getHomeActivity.mockReset();
    useHomeLive.mockReset();
    useHomeWorkingNow.mockReset();
    getHomeActivity.mockResolvedValue([]);
    fetchStatus.mockReset();
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
    activeWorkspaceState.runtimeWorkspaceId = "ws-home";
    activeWorkspaceState.scope = "global";
    activeWorkspaceState.isLoading = false;
  });

  it("Should resolve Global menubar scope to the empty workspace param", async () => {
    getHomeOverview.mockResolvedValue(makeHomeOverview());

    const { result } = renderHook(() => useHomeDashboard(), { wrapper: wrapper() });
    expect(result.current.overviewStatus).toBe("loading");

    await waitFor(() => {
      expect(result.current.overviewStatus).toBe("ready");
    });
    expect(result.current.scope.workspaceParam).toBe("");
    expect(result.current.agents.workspaceId).toBe("ws-home");
    expect(result.current.overview?.attention.total).toBe(2);
    expect(getHomeOverview).toHaveBeenCalledWith(
      expect.objectContaining({ workspace: undefined, usageWindow: 30 }),
      expect.anything()
    );
  });

  it.each([false, true])(
    "Should pass liveEnabled=%s to the Home subscription after scope settles",
    async liveEnabled => {
      getHomeOverview.mockResolvedValue(makeHomeOverview());

      const { result } = renderHook(() => useHomeDashboard({ liveEnabled }), {
        wrapper: wrapper(),
      });
      await waitFor(() => expect(result.current.overviewStatus).toBe("ready"));

      expect(useHomeLive).toHaveBeenLastCalledWith({ workspaceId: "", enabled: liveEnabled });
      expect(useHomeWorkingNow).toHaveBeenLastCalledWith(
        expect.objectContaining({ workspaceParam: "" }),
        liveEnabled
      );
    }
  );

  it("Should scope the overview to a project workspace", async () => {
    activeWorkspaceState.activeWorkspace = {
      id: "ws-proj",
      root_dir: "/Users/tester/dev/proj",
      name: "proj",
    };
    activeWorkspaceState.activeWorkspaceId = "ws-proj";
    activeWorkspaceState.runtimeWorkspaceId = "ws-proj";
    activeWorkspaceState.scope = "workspace";
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
    // Workspace-isolation guard: the queries stay disabled until menubar
    // workspace scope has a project id, so the daemon never sees an
    // undefined-workspace (whole-system) overview/activity read during mount.
    activeWorkspaceState.activeWorkspace = {
      id: "ws-proj",
      root_dir: "/Users/tester/dev/proj",
      name: "proj",
    };
    activeWorkspaceState.activeWorkspaceId = "ws-proj";
    activeWorkspaceState.runtimeWorkspaceId = "ws-proj";
    activeWorkspaceState.scope = "workspace";
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
