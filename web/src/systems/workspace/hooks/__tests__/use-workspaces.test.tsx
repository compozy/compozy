import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { FIXTURE_AGENT_DEFINITION_DIGEST } from "@/systems/agent/mocks";
import { statusKeys } from "@/systems/status";
import { ACTIVE_WORKSPACE_PERSIST_KEY } from "@/systems/workspace";
import { statusFixture } from "@/systems/status/mocks";

import { useActiveWorkspace } from "../use-active-workspace";
import { activeWorkspaceStore, rehydrateActiveWorkspaceStore } from "../use-active-workspace-store";
import {
  pruneWorktreeScopes,
  setActiveWorkspaceId,
  setActiveWorktreeId,
} from "../../stores/active-workspace-store";
import { useUserHomeDir } from "../use-user-home-dir";
import {
  useDeleteWorkspace,
  useCreateWorkspace,
  useResolveWorkspace,
  useWorkspace,
  useWorkspaces,
} from "../use-workspaces";
import { workspaceKeys } from "../../lib/query-keys";
import {
  WORKSPACE_REFETCH_INTERVAL,
  workspaceDetailOptions,
  workspacesListOptions,
} from "../../lib/query-options";

vi.mock("@/systems/workspace/adapters/workspace-api", () => ({
  deleteWorkspace: vi.fn(),
  createWorkspace: vi.fn(),
  fetchWorkspace: vi.fn(),
  fetchWorkspaces: vi.fn(),
  resolveWorkspace: vi.fn(),
}));

import {
  deleteWorkspace,
  createWorkspace,
  fetchWorkspace,
  fetchWorkspaces,
  resolveWorkspace,
} from "@/systems/workspace/adapters/workspace-api";
import type { WorkspacePayload } from "@/systems/workspace/types";

function createWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

function makeWorkspace(overrides: Partial<WorkspacePayload> = {}): WorkspacePayload {
  return {
    id: "ws_alpha",
    root_dir: "/workspace/alpha",
    add_dirs: [],
    name: "alpha",
    created_at: "2026-04-06T10:00:00Z",
    updated_at: "2026-04-06T10:00:00Z",
    ...overrides,
  };
}

describe("workspace hooks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setActiveWorkspaceId(null);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("loads the workspace registry", async () => {
    vi.mocked(fetchWorkspaces).mockResolvedValue([
      {
        id: "ws_alpha",
        root_dir: "/workspace/alpha",
        add_dirs: [],
        name: "alpha",
        created_at: "2026-04-06T10:00:00Z",
        updated_at: "2026-04-06T10:00:00Z",
      },
    ]);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    const { result } = renderHook(() => useWorkspaces(), {
      wrapper: createWrapper(queryClient),
    });

    await waitFor(() => {
      expect(result.current.data).toHaveLength(1);
    });

    expect(fetchWorkspaces).toHaveBeenCalledOnce();
  });

  it("keeps workspace registry and detail queries fresh for external workspace removal", () => {
    expect(workspacesListOptions().refetchInterval).toBe(WORKSPACE_REFETCH_INTERVAL);
    expect(workspaceDetailOptions("ws_removed").refetchInterval).toBe(WORKSPACE_REFETCH_INTERVAL);
  });

  it("keeps cached workspaces visible without fetching while disabled", () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    queryClient.setQueryData(statusKeys.current(), statusFixture);
    queryClient.setQueryData(workspaceKeys.list(), [makeWorkspace()]);

    const { result } = renderHook(() => useActiveWorkspace({ enabled: false }), {
      wrapper: createWrapper(queryClient),
    });

    // No selection means All projects focused on User — never a `workspaces[0]` fallback.
    expect(result.current.workspaces).toHaveLength(1);
    expect(result.current.breadthMode).toBe("all");
    expect(result.current.focus).toEqual({ kind: "user" });
    expect(result.current.activeWorkspaceId).toBeNull();
    expect(fetchWorkspaces).not.toHaveBeenCalled();
  });

  it("loads one resolved workspace detail", async () => {
    vi.mocked(fetchWorkspace).mockResolvedValue({
      agents: [
        {
          name: "coder",
          prompt: "code",
          provider: "openai",
          origin: "workspace",
          workspace_id: "ws_alpha",
          definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
        },
      ],
      sessions: [],
      skills: [],
      workspace: {
        id: "ws_alpha",
        root_dir: "/workspace/alpha",
        add_dirs: [],
        name: "alpha",
        created_at: "2026-04-06T10:00:00Z",
        updated_at: "2026-04-06T10:00:00Z",
      },
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    const { result } = renderHook(() => useWorkspace("ws_alpha"), {
      wrapper: createWrapper(queryClient),
    });

    await waitFor(() => {
      expect(result.current.data?.workspace.id).toBe("ws_alpha");
    });

    expect(fetchWorkspace).toHaveBeenCalledWith("ws_alpha", expect.any(AbortSignal));
  });

  it("allows callers to disable the workspace detail query", () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    renderHook(() => useWorkspace("ws_alpha", { enabled: false }), {
      wrapper: createWrapper(queryClient),
    });

    expect(fetchWorkspace).not.toHaveBeenCalled();
  });

  it("invalidates the workspace list after resolving a workspace", async () => {
    const resolvedWorkspace = {
      id: "ws_alpha",
      root_dir: "/workspace/alpha",
      add_dirs: [],
      name: "alpha",
      created_at: "2026-04-06T10:00:00Z",
      updated_at: "2026-04-06T10:00:00Z",
    };
    vi.mocked(resolveWorkspace).mockResolvedValue(resolvedWorkspace);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useResolveWorkspace(), {
      wrapper: createWrapper(queryClient),
    });

    result.current.mutate({ path: "/workspace/alpha" });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(resolveWorkspace).toHaveBeenCalledWith({ path: "/workspace/alpha" });
    expect(queryClient.getQueryData(workspaceKeys.list())).toEqual([resolvedWorkspace]);
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: workspaceKeys.lists() });
  });

  it("reconciles created workspaces by identity without duplicating the registry", async () => {
    const existingWorkspace = makeWorkspace({ name: "previous" });
    const createdWorkspace = makeWorkspace({ name: "updated" });
    vi.mocked(createWorkspace).mockResolvedValue(createdWorkspace);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    queryClient.setQueryData(workspaceKeys.list(), [existingWorkspace]);

    const { result } = renderHook(() => useCreateWorkspace(), {
      wrapper: createWrapper(queryClient),
    });

    result.current.mutate({ root_dir: createdWorkspace.root_dir });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(queryClient.getQueryData(workspaceKeys.list())).toEqual([createdWorkspace]);
  });

  it("keeps a resolved archived project out of the active registry", async () => {
    const archived = makeWorkspace({ archived_at: "2026-10-28T12:00:00Z" });
    vi.mocked(resolveWorkspace).mockResolvedValue(archived);
    const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
    queryClient.setQueryData(workspaceKeys.list(), [archived]);
    const { result } = renderHook(() => useResolveWorkspace(), {
      wrapper: createWrapper(queryClient),
    });
    result.current.mutate({ path: archived.root_dir });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(queryClient.getQueryData(workspaceKeys.list())).toEqual([]);
  });

  it("removes deleted workspaces from cached registry data", async () => {
    vi.mocked(deleteWorkspace).mockResolvedValue();

    const removedWorkspace = makeWorkspace({ id: "ws_removed", name: "removed" });
    const keptWorkspace = makeWorkspace({
      id: "ws_kept",
      name: "kept",
      root_dir: "/workspace/kept",
    });
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    queryClient.setQueryData(workspaceKeys.list(), [removedWorkspace, keptWorkspace]);
    queryClient.setQueryData(workspaceKeys.detail("ws_removed"), { workspace: removedWorkspace });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useDeleteWorkspace(), {
      wrapper: createWrapper(queryClient),
    });

    result.current.mutate("ws_removed");

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(deleteWorkspace).toHaveBeenCalledWith("ws_removed");
    expect(queryClient.getQueryData(workspaceKeys.list())).toEqual([keptWorkspace]);
    expect(queryClient.getQueryData(workspaceKeys.detail("ws_removed"))).toBeUndefined();
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: workspaceKeys.lists() });
  });
});

describe("useActiveWorkspace", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setActiveWorkspaceId(null);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("Should keep the desktop on the destination across catalog changes and rehydration", async () => {
    const alpha = makeWorkspace({ id: "ws_alpha" });
    const beta = makeWorkspace({ id: "ws_beta", root_dir: "/workspace/beta" });
    vi.mocked(fetchWorkspaces).mockResolvedValue([alpha]);
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    queryClient.setQueryData(statusKeys.current(), statusFixture);
    const wrapper = createWrapper(queryClient);
    const first = renderHook(() => useActiveWorkspace(), { wrapper });
    await waitFor(() => expect(first.result.current.pending).toBe(false));
    // All projects with User focused: catalog order never picks a project desktop.
    expect(first.result.current.desktopWorkspaceId).toBe("global");

    act(() => queryClient.setQueryData(workspaceKeys.list(), [beta, alpha]));
    await waitFor(() => expect(first.result.current.projectWorkspaces[0]?.id).toBe(beta.id));
    expect(first.result.current.desktopWorkspaceId).toBe("global");
    expect(first.result.current.dataScope).toEqual({ kind: "all" });
    expect(first.result.current.selectedWorkspaceId).toBeNull();
    expect(first.result.current.runtimeWorkspaceId).toBeNull();

    act(() => setActiveWorkspaceId(alpha.id));
    await waitFor(() => expect(first.result.current.desktopWorkspaceId).toBe(alpha.id));
    const persisted = window.localStorage.getItem(ACTIVE_WORKSPACE_PERSIST_KEY);
    expect(persisted).not.toBeNull();
    first.unmount();
    setActiveWorkspaceId(null);
    window.localStorage.setItem(ACTIVE_WORKSPACE_PERSIST_KEY, persisted!);
    await rehydrateActiveWorkspaceStore();

    const restored = renderHook(() => useActiveWorkspace(), { wrapper });
    expect(restored.result.current.desktopWorkspaceId).toBe(alpha.id);
    expect(restored.result.current.dataScope).toEqual({ kind: "project", workspaceId: alpha.id });
  });

  it.each([
    ["added", ["window:editorial", "window:home"]],
    ["removed", []],
  ])("Should retain another tab's saved destination when windows are %s", async (_, liveIds) => {
    const alpha = makeWorkspace();
    setActiveWorktreeId("window:editorial", alpha.id, "wt_review");
    // Another document explicitly chose User; this open document keeps its project view.
    window.localStorage.setItem(
      ACTIVE_WORKSPACE_PERSIST_KEY,
      JSON.stringify({
        context: {
          ...activeWorkspaceStore.getSnapshot().context,
          breadthMode: "focused",
          focus: { kind: "user" },
        },
        version: 2,
      })
    );
    const storedDestination = () => {
      const stored = JSON.parse(window.localStorage.getItem(ACTIVE_WORKSPACE_PERSIST_KEY)!);
      return { breadthMode: stored.context.breadthMode, focus: stored.context.focus };
    };

    pruneWorktreeScopes(`${alpha.id}\u0000default`, liveIds);
    expect(storedDestination()).toEqual({ breadthMode: "focused", focus: { kind: "user" } });

    vi.mocked(fetchWorkspaces).mockResolvedValue([alpha]);
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    queryClient.setQueryData(statusKeys.current(), statusFixture);
    queryClient.setQueryData(workspaceKeys.list(), [alpha]);
    const { result } = renderHook(() => useActiveWorkspace(), {
      wrapper: createWrapper(queryClient),
    });
    await waitFor(() => expect(result.current.desktopWorkspaceId).toBe(alpha.id));
    expect(result.current.dataScope).toEqual({ kind: "project", workspaceId: alpha.id });
    expect(storedDestination()).toEqual({ breadthMode: "focused", focus: { kind: "user" } });

    await act(async () => rehydrateActiveWorkspaceStore());
    expect(result.current.dataScope).toEqual({ kind: "user" });
    expect(result.current.desktopWorkspaceId).toBe("user");
  });

  it("uses the persisted selected workspace after rehydration", async () => {
    setActiveWorkspaceId("ws_alpha");
    expect(window.localStorage.getItem(ACTIVE_WORKSPACE_PERSIST_KEY)).toContain("ws_alpha");
    window.localStorage.setItem(
      ACTIVE_WORKSPACE_PERSIST_KEY,
      JSON.stringify({
        context: {
          scope: "workspace",
          selectedWorkspaceId: "ws_beta",
          worktreeByScope: { shell: "wt_saved" },
        },
        version: 0,
      })
    );
    await act(async () => {
      await rehydrateActiveWorkspaceStore();
    });
    expect(activeWorkspaceStore.getSnapshot().context).toEqual({
      breadthMode: "focused",
      focus: { kind: "project", workspaceId: "ws_beta" },
      selectedWorkspaceId: "ws_beta",
      worktreeByScope: { shell: "wt_saved" },
      worktreePartitionByScope: {},
      legacySessionsScopeApplied: null,
    });
    vi.mocked(fetchWorkspaces).mockResolvedValue([
      makeWorkspace({ id: "ws_alpha", name: "alpha" }),
      makeWorkspace({ id: "ws_beta", name: "beta", root_dir: "/workspace/beta" }),
    ]);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    queryClient.setQueryData(statusKeys.current(), statusFixture);

    const { result } = renderHook(() => useActiveWorkspace(), {
      wrapper: createWrapper(queryClient),
    });

    await waitFor(() => {
      expect(result.current.activeWorkspaceId).toBe("ws_beta");
    });

    expect(result.current.activeWorkspace?.name).toBe("beta");
  });

  it("Should focus User when the remembered project is missing instead of falling back", async () => {
    setActiveWorkspaceId("ws_missing");
    vi.mocked(fetchWorkspaces).mockResolvedValue([
      makeWorkspace({ id: "ws_alpha", name: "alpha" }),
      makeWorkspace({ id: "ws_beta", name: "beta", root_dir: "/workspace/beta" }),
    ]);
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    queryClient.setQueryData(statusKeys.current(), statusFixture);

    const { result } = renderHook(() => useActiveWorkspace(), {
      wrapper: createWrapper(queryClient),
    });

    await waitFor(() => expect(result.current.dataScope).toEqual({ kind: "user" }));
    expect(result.current.desktopWorkspaceId).toBe("user");
    expect(result.current.activeWorkspaceId).toBeNull();
    expect(result.current.selectedWorkspaceId).toBe("ws_missing");
    expect(activeWorkspaceStore.getSnapshot().context.selectedWorkspaceId).toBe("ws_missing");
  });
});

describe("useUserHomeDir", () => {
  it("reads the daemon home directory directly from the status query", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: Number.POSITIVE_INFINITY } },
    });
    queryClient.setQueryData(statusKeys.current(), {
      ...statusFixture,
      daemon: { ...statusFixture.daemon, user_home_dir: "/workspace/home-one" },
    });

    const { result } = renderHook(() => useUserHomeDir(), {
      wrapper: createWrapper(queryClient),
    });

    expect(result.current).toBe("/workspace/home-one");

    act(() => {
      queryClient.setQueryData(statusKeys.current(), {
        ...statusFixture,
        daemon: { ...statusFixture.daemon, user_home_dir: "/workspace/home-two" },
      });
    });

    await waitFor(() => {
      expect(result.current).toBe("/workspace/home-two");
    });
  });
});
