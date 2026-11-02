// Suite: home dashboard model
// Invariant: dashboard sections expose current scoped data without hidden-window refresh work.
// Boundary IN: dashboard query composition, liveness, and derived view state.
// Boundary OUT: card rendering and task transport behavior, owned by their domain suites.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { StatusPayload } from "@/systems/status";
import { runtimeIdentityFixture, statusFixture } from "@/systems/status/mocks";

import { makeEmptyHomeOverview, makeHomeOverview } from "../mocks/fixtures";
import type { HomeOverview } from "../types";

const liveFreshness = () => ({ ...makeEmptyHomeOverview().freshness, has_live_work: true });
const singleApproval = () => ({
  total: 1,
  by_kind: { approval: 1, failure: 0, needs_input: 0 },
  items: [
    {
      kind: "approval" as const,
      title: "Deploy docs site",
      task_id: "task-approval",
      workspace_id: "ws-proj",
      occurred_at: "2026-07-23T10:00:00Z",
      actions: ["approve" as const, "reject" as const, "open" as const],
    },
  ],
});
const oneRunCompleted = () => ({ runs_completed: 1, runs_failed: 0, tasks_closed: 0 });
const busyPulse = () => ({
  ...makeEmptyHomeOverview().pulse,
  buckets: [{ weekday: 2, hour: 14, events: 3 }],
});

const getHomeOverview = vi.fn();
const getHomeActivity = vi.fn();
const fetchStatus = vi.fn();
const fetchRuntimeIdentity = vi.fn();
const useHomeLive = vi.fn();
const useHomeWorkingNow = vi.fn();
const profileScope = vi.hoisted(() => ({
  aggregate: false,
  params: { profile: "default" } as { profile: string } | { all_profiles: true },
}));

vi.mock("@/systems/profiles", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/profiles")>();
  return {
    ...actual,
    useProfileReadScope: () => ({
      aggregate: profileScope.aggregate,
      destination: "default",
      params: profileScope.params,
    }),
  };
});

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
    fetchRuntimeIdentity: (...args: Parameters<typeof actual.fetchRuntimeIdentity>) =>
      fetchRuntimeIdentity(...args),
  };
});

const activeWorkspaceState = {
  activeWorkspace: { id: "ws-home", root_dir: "/Users/tester", name: "home" } as unknown,
  activeWorkspaceId: "ws-home",
  runtimeWorkspaceId: "ws-home",
  dataScope: { kind: "all" } as
    | { kind: "all" }
    | { kind: "user" }
    | { kind: "project"; workspaceId: string },
  registeredWorkspaces: [
    { id: "ws-home", root_dir: "/Users/tester", name: "home" },
    { id: "ws-proj", root_dir: "/Users/tester/dev/proj", name: "proj" },
  ],
  isLoading: false,
};

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => activeWorkspaceState,
}));

import { homePrefsStore } from "../hooks/use-home-prefs-store";
import { useHomeDashboard } from "../hooks/use-home-dashboard";
import { buildHomeSystemModel } from "../lib/home-system";

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
    fetchRuntimeIdentity.mockReset();
    fetchRuntimeIdentity.mockResolvedValue(runtimeIdentityFixture);
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
    activeWorkspaceState.dataScope = { kind: "all" };
    activeWorkspaceState.isLoading = false;
    profileScope.aggregate = false;
    profileScope.params = { profile: "default" };
  });

  it("Should read User-scope Home through @user, never the whole-system aggregate", async () => {
    activeWorkspaceState.activeWorkspace = undefined;
    activeWorkspaceState.activeWorkspaceId = "";
    activeWorkspaceState.runtimeWorkspaceId = "";
    activeWorkspaceState.dataScope = { kind: "user" };
    getHomeOverview.mockResolvedValue(makeHomeOverview());

    const { result } = renderHook(() => useHomeDashboard(), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.overviewStatus).toBe("ready"));

    expect(result.current.scope.workspaceParam).toBe("@user");
    expect(result.current.activeWorkspaceName).toBe("User");
    for (const [filter] of getHomeOverview.mock.calls) {
      expect(filter).toMatchObject({ workspace: "@user" });
    }
    for (const [filter] of getHomeActivity.mock.calls) {
      expect(filter).toMatchObject({ workspace_id: "@user" });
    }
  });

  // W-22: Home under All projects reads the explicit overview and task aggregates, never
  // the overview's omission (which is User's tasks only), and its header names All projects.
  it("Should read every project under All projects through the explicit aggregates", async () => {
    getHomeOverview.mockResolvedValue(makeHomeOverview());

    const { result } = renderHook(() => useHomeDashboard(), { wrapper: wrapper() });
    expect(result.current.overviewStatus).toBe("loading");

    await waitFor(() => {
      expect(result.current.overviewStatus).toBe("ready");
    });
    expect(result.current.scope.workspaceParam).toBe("");
    expect(result.current.scope.taskScope).toEqual({ all_workspaces: true });
    expect(result.current.activeWorkspaceName).toBe("All projects");
    expect(result.current.agents.workspaceId).toBe("ws-home");
    expect(result.current.overview?.attention.total).toBe(2);
    expect(getHomeOverview).toHaveBeenCalledWith(
      expect.objectContaining({ allWorkspaces: true, usageWindow: 30 }),
      expect.anything()
    );
    for (const [filter] of getHomeOverview.mock.calls) {
      expect(filter).not.toHaveProperty("workspace");
    }
    // Acknowledgements name the same all-projects population the overview showed.
    expect(result.current.notificationScope).toMatchObject({ all_workspaces: true });
    // Working-now cards name their projects from the registered catalog.
    const [, , names] = useHomeWorkingNow.mock.calls.at(-1) ?? [];
    expect(names).toEqual(
      new Map([
        ["ws-home", "home"],
        ["ws-proj", "proj"],
      ])
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

      // The stream carries the same lens as the reads beside it, so the feed and
      // the list can never disagree about which owners are in view.
      expect(useHomeLive).toHaveBeenLastCalledWith({
        workspaceId: "",
        scope: { profile: "default" },
        enabled: liveEnabled,
      });
      expect(useHomeWorkingNow).toHaveBeenLastCalledWith(
        expect.objectContaining({ workspaceParam: "" }),
        liveEnabled,
        expect.any(Map)
      );
    }
  );

  it("Should carry the aggregate profile lens through every dashboard read", async () => {
    profileScope.aggregate = true;
    profileScope.params = { all_profiles: true };
    getHomeOverview.mockResolvedValue(makeHomeOverview());

    const { result } = renderHook(() => useHomeDashboard(), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.overviewStatus).toBe("ready"));

    expect(getHomeOverview).toHaveBeenCalledWith(
      expect.objectContaining({ allProfiles: true }),
      expect.anything()
    );
    expect(getHomeActivity).toHaveBeenCalledWith(
      expect.objectContaining({ all_profiles: true }),
      expect.anything()
    );
    expect(useHomeLive).toHaveBeenLastCalledWith({
      workspaceId: "",
      scope: { all_profiles: true },
      enabled: true,
    });
    expect(result.current.profileAggregate).toBe(true);
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
    activeWorkspaceState.dataScope = { kind: "project", workspaceId: "ws-proj" };
    getHomeOverview.mockResolvedValue(makeHomeOverview());

    const { result } = renderHook(() => useHomeDashboard(), { wrapper: wrapper() });
    await waitFor(() => {
      expect(result.current.overviewStatus).toBe("ready");
    });

    for (const [filter] of getHomeOverview.mock.calls) {
      expect(filter).toMatchObject({ workspace: "ws-proj" });
    }
    for (const [filter] of getHomeActivity.mock.calls) {
      // The lens rides the same filter as the workspace, so activity is never
      // read unscoped on either axis.
      expect(filter).toMatchObject({ workspace_id: "ws-proj", profile: "default" });
    }
    expect(getHomeOverview).not.toHaveBeenCalledWith(
      expect.objectContaining({ workspace: undefined }),
      expect.anything()
    );
    expect(getHomeActivity).not.toHaveBeenCalledWith(
      expect.objectContaining({ workspace_id: undefined }),
      expect.anything()
    );
    expect(result.current.scope.workspaceParam).toBe("ws-proj");
    expect(getHomeOverview).toHaveBeenCalledWith(
      expect.objectContaining({ workspace: "ws-proj" }),
      expect.anything()
    );
  });

  it("Should report no work only when every overview counter is zero and nothing is live", async () => {
    getHomeOverview.mockResolvedValue(makeEmptyHomeOverview());

    const { result } = renderHook(() => useHomeDashboard(), { wrapper: wrapper() });
    expect(result.current.hasNoWork).toBe(false);

    await waitFor(() => expect(result.current.overviewStatus).toBe("ready"));
    expect(result.current.hasNoWork).toBe(true);
  });

  it.each([
    ["live work is running", makeEmptyHomeOverview({ freshness: liveFreshness() })],
    ["something needs the person", makeEmptyHomeOverview({ attention: singleApproval() })],
    ["a run closed today", makeEmptyHomeOverview({ today: oneRunCompleted() })],
    ["the pulse recorded events outside today", makeEmptyHomeOverview({ pulse: busyPulse() })],
  ])("Should keep the dashboard when %s", async (_case, overview) => {
    getHomeOverview.mockResolvedValue(overview);

    const { result } = renderHook(() => useHomeDashboard(), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.overviewStatus).toBe("ready"));
    expect(result.current.hasNoWork).toBe(false);
  });

  it("Should surface an error status with the adapter message", async () => {
    getHomeOverview.mockRejectedValue(new Error("daemon unavailable"));

    const { result } = renderHook(() => useHomeDashboard(), { wrapper: wrapper() });
    await waitFor(() => {
      expect(result.current.overviewStatus).toBe("error");
    });
    expect(result.current.overviewErrorMessage).toBe("daemon unavailable");
  });

  // Switching the usage window re-keys the overview read; the page keeps what it shows rather
  // than blanking into its loading shape for a change that only moves one chart.
  it("Should keep the last overview on screen while another usage window loads", async () => {
    const thirtyDays = makeHomeOverview();
    getHomeOverview.mockResolvedValueOnce(thirtyDays);

    const { result } = renderHook(() => useHomeDashboard(), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.overviewStatus).toBe("ready"));

    let finishSevenDays: (overview: HomeOverview) => void = () => {};
    getHomeOverview.mockReturnValueOnce(
      new Promise<HomeOverview>(resolve => {
        finishSevenDays = resolve;
      })
    );
    act(() => homePrefsStore.trigger.usageWindowSelected({ usageWindow: 7 }));

    await waitFor(() => expect(getHomeOverview).toHaveBeenCalledTimes(2));
    expect(result.current.overviewStatus).toBe("ready");
    expect(result.current.overview).toBe(thirtyDays);

    const sevenDays = makeHomeOverview({ usage: { ...thirtyDays.usage, window_days: 7 } });
    await act(async () => finishSevenDays(sevenDays));
    await waitFor(() => expect(result.current.overview).toBe(sevenDays));
  });

  it("Should not carry an overview into another project while its read loads", async () => {
    getHomeOverview.mockResolvedValueOnce(makeHomeOverview());

    const { result, rerender } = renderHook(() => useHomeDashboard(), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.overviewStatus).toBe("ready"));

    getHomeOverview.mockReturnValueOnce(new Promise<HomeOverview>(() => {}));
    activeWorkspaceState.activeWorkspace = {
      id: "ws-proj",
      root_dir: "/Users/tester/dev/proj",
      name: "proj",
    };
    activeWorkspaceState.runtimeWorkspaceId = "ws-proj";
    activeWorkspaceState.dataScope = { kind: "project", workspaceId: "ws-proj" };
    rerender();

    await waitFor(() => expect(getHomeOverview).toHaveBeenCalledTimes(2));
    expect(result.current.overviewStatus).toBe("loading");
    expect(result.current.overview).toBeUndefined();
  });

  it("Should hold the system row loading until the daemon status answers", async () => {
    getHomeOverview.mockResolvedValue(makeHomeOverview());

    const { result } = renderHook(() => useHomeDashboard(), { wrapper: wrapper() });
    expect(result.current.systemLoading).toBe(true);

    await waitFor(() => expect(result.current.systemLoading).toBe(false));
    expect(result.current.system.tiles.length).toBeGreaterThan(0);
  });
});

describe("buildHomeSystemModel", () => {
  type StatusProvider = NonNullable<StatusPayload["providers"]>[number];
  const provider = (name: string, state: string) => ({ name, state }) as StatusProvider;

  it("Should stay not-normal and list no tiles before the daemon status loads", () => {
    const model = buildHomeSystemModel(undefined, {
      hookRunsToday: undefined,
      hookFailuresToday: undefined,
      retentionDays: undefined,
    });

    expect(model).toEqual({ allNormal: false, summary: "", tiles: [] });
  });

  it("Should flag an unready provider or failed hook and summarize uptime with provider readiness", () => {
    const status: StatusPayload = {
      ...statusFixture,
      providers: [provider("claude", "ready"), provider("codex", "needs_auth")],
    };
    const quiet = buildHomeSystemModel(
      { ...status, providers: [provider("claude", "ready")] },
      { hookRunsToday: 4, hookFailuresToday: 0, retentionDays: 0 }
    );
    const degraded = buildHomeSystemModel(status, {
      hookRunsToday: 4,
      hookFailuresToday: 2,
      retentionDays: 30,
    });

    expect(quiet.allNormal).toBe(true);
    expect(quiet.tiles.find(tile => tile.key === "retention")?.value).toBe("Forever");
    expect(degraded.allNormal).toBe(false);
    expect(degraded.summary).toBe("Running for 2h 0m · 1 of 2 providers ready");
    expect(degraded.tiles.map(tile => tile.key)).toEqual([
      "daemon",
      "providers",
      "scheduler",
      "hooks",
      "retention",
    ]);
    expect(degraded.tiles.find(tile => tile.key === "hooks")).toMatchObject({
      detail: "2 failed",
      tone: "warning",
    });
  });
});
