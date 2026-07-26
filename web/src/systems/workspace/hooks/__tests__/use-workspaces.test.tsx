import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { FIXTURE_AGENT_DEFINITION_DIGEST } from "@/systems/agent/mocks";

import { useActiveWorkspace } from "../use-active-workspace";
import { useActiveWorkspaceStore } from "../use-active-workspace-store";
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
    useActiveWorkspaceStore.setState({ selectedWorkspaceId: null });
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
    useActiveWorkspaceStore.setState({ selectedWorkspaceId: null });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("waits for persisted selection hydration before auto-selecting the first workspace", async () => {
    vi.spyOn(useActiveWorkspaceStore.persist, "hasHydrated").mockReturnValue(false);
    vi.mocked(fetchWorkspaces).mockResolvedValue([
      makeWorkspace({ id: "ws_alpha", name: "alpha" }),
      makeWorkspace({ id: "ws_beta", name: "beta", root_dir: "/workspace/beta" }),
    ]);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    const { result } = renderHook(() => useActiveWorkspace(), {
      wrapper: createWrapper(queryClient),
    });

    await waitFor(() => {
      expect(result.current.workspaces).toHaveLength(2);
    });

    expect(result.current.activeWorkspaceId).toBeNull();
    expect(useActiveWorkspaceStore.getState().selectedWorkspaceId).toBeNull();
  });

  it("uses the persisted selected workspace after rehydration", async () => {
    window.localStorage.setItem(
      "agh:active-workspace",
      JSON.stringify({ state: { selectedWorkspaceId: "ws_beta" }, version: 0 })
    );
    await act(async () => {
      await useActiveWorkspaceStore.persist.rehydrate();
    });
    vi.mocked(fetchWorkspaces).mockResolvedValue([
      makeWorkspace({ id: "ws_alpha", name: "alpha" }),
      makeWorkspace({ id: "ws_beta", name: "beta", root_dir: "/workspace/beta" }),
    ]);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    const { result } = renderHook(() => useActiveWorkspace(), {
      wrapper: createWrapper(queryClient),
    });

    await waitFor(() => {
      expect(result.current.activeWorkspaceId).toBe("ws_beta");
    });

    expect(result.current.activeWorkspace?.name).toBe("beta");
  });
});
