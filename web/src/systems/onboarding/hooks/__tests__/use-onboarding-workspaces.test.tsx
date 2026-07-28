// Suite: onboarding workspace draft reconciliation
// Invariant: persisted workspace identities are valid only when present in the current daemon catalog.
// Boundary IN: onboarding workspace hook and persisted draft store.
// Boundary OUT: workspace HTTP adapters and browser journey replay.
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { WorkspacePayload } from "@/systems/workspace";

import {
  ONBOARDING_DRAFT_STORAGE_KEY,
  useOnboardingDraftStore,
} from "../../stores/use-onboarding-draft-store";
import { useOnboardingWorkspaces } from "../use-onboarding-workspaces";

const mocks = vi.hoisted(() => ({
  deleteWorkspace: vi.fn(),
  registeredWorkspaces: [] as WorkspacePayload[] | undefined,
  resolveWorkspace: vi.fn(),
  workspacesIsLoading: false,
  workspacesError: null as Error | null,
  refetchWorkspaces: vi.fn(),
}));

vi.mock("@/systems/workspace", () => ({
  useDeleteWorkspace: () => ({
    isPending: false,
    mutateAsync: mocks.deleteWorkspace,
  }),
  useResolveWorkspace: () => ({
    isPending: false,
    mutateAsync: mocks.resolveWorkspace,
  }),
  useWorkspaces: () => ({
    data: mocks.registeredWorkspaces,
    error: mocks.workspacesError,
    isFetching: mocks.workspacesIsLoading,
    isLoading: mocks.workspacesIsLoading,
    refetch: mocks.refetchWorkspaces,
  }),
}));

vi.mock("../use-directory-browser", () => ({
  useDirectoryBrowser: () => ({
    data: {
      path: "/Users/operator",
      parent: null,
      home: "/Users/operator",
      entries: [],
    },
    error: null,
    isFetching: false,
    isLoading: false,
  }),
}));

const now = "2026-05-27T00:00:00Z";

function workspace(overrides: Partial<WorkspacePayload> = {}): WorkspacePayload {
  return {
    id: "ws_home",
    root_dir: "/Users/operator",
    add_dirs: [],
    name: "operator",
    created_at: now,
    updated_at: now,
    ...overrides,
  };
}

describe("useOnboardingWorkspaces", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.localStorage.clear();
    mocks.registeredWorkspaces = [];
    mocks.workspacesIsLoading = false;
    mocks.workspacesError = null;
    useOnboardingDraftStore.getState().reset();
  });

  it("hydrates an empty onboarding draft from daemon-registered workspaces", async () => {
    mocks.registeredWorkspaces = [
      workspace(),
      workspace({
        id: "ws_project",
        root_dir: "/Users/operator/project",
        name: "project",
      }),
    ];

    const { result } = renderHook(() => useOnboardingWorkspaces());

    await waitFor(() => {
      expect(result.current.workspaces).toEqual([
        { path: "/Users/operator", name: "operator", workspaceId: "ws_home" },
        { path: "/Users/operator/project", name: "project", workspaceId: "ws_project" },
      ]);
    });
    expect(useOnboardingDraftStore.getState().workspaces).toEqual([
      { path: "/Users/operator", name: "operator", workspaceId: "ws_home" },
      { path: "/Users/operator/project", name: "project", workspaceId: "ws_project" },
    ]);
  });

  it("does not overwrite an existing onboarding draft with daemon workspaces", async () => {
    act(() => {
      useOnboardingDraftStore
        .getState()
        .addWorkspace({ path: "/Users/operator/manual", name: "manual" });
    });
    mocks.registeredWorkspaces = [workspace()];

    const { result } = renderHook(() => useOnboardingWorkspaces());

    await waitFor(() => {
      expect(result.current.workspaces).toEqual([
        { path: "/Users/operator/manual", name: "manual" },
      ]);
    });
    expect(useOnboardingDraftStore.getState().workspaces).toEqual([
      { path: "/Users/operator/manual", name: "manual" },
    ]);
  });

  it("blocks removal while the current daemon workspace catalog is unresolved", async () => {
    const currentWorkspace = {
      path: "/Users/operator/current-daemon",
      name: "current-daemon",
      workspaceId: "ws_current_daemon",
    };
    window.localStorage.setItem(
      ONBOARDING_DRAFT_STORAGE_KEY,
      JSON.stringify({ state: { workspaces: [currentWorkspace] }, version: 0 })
    );
    await act(async () => {
      await useOnboardingDraftStore.persist.rehydrate();
    });
    mocks.registeredWorkspaces = undefined;
    mocks.workspacesIsLoading = true;

    const { result } = renderHook(() => useOnboardingWorkspaces());

    expect.soft(result.current.isRemoving).toBe(false);
    expect.soft(result.current.isCatalogLoading).toBe(true);
    await act(async () => {
      await result.current.removeWorkspace(currentWorkspace.path);
    });

    expect(mocks.deleteWorkspace).not.toHaveBeenCalled();
    expect(useOnboardingDraftStore.getState().workspaces).toEqual([currentWorkspace]);
  });

  it("exposes catalog failure separately and allows an explicit retry", async () => {
    mocks.registeredWorkspaces = undefined;
    mocks.workspacesError = new Error("workspace catalog unavailable");
    mocks.refetchWorkspaces.mockResolvedValue(undefined);

    const { result } = renderHook(() => useOnboardingWorkspaces());

    expect(result.current.catalogError).toBe("workspace catalog unavailable");
    expect(result.current.isCatalogLoading).toBe(false);
    expect(result.current.isRemoving).toBe(false);
    await act(async () => {
      await result.current.reloadCatalog();
    });
    expect(mocks.refetchWorkspaces).toHaveBeenCalledOnce();
  });

  it("removes a persisted workspace from another daemon without deleting through the current daemon", async () => {
    const staleWorkspace = {
      path: "/Users/operator/previous-daemon",
      name: "previous-daemon",
      workspaceId: "ws_previous_daemon",
    };
    window.localStorage.setItem(
      ONBOARDING_DRAFT_STORAGE_KEY,
      JSON.stringify({ state: { workspaces: [staleWorkspace] }, version: 0 })
    );
    await act(async () => {
      await useOnboardingDraftStore.persist.rehydrate();
    });
    mocks.deleteWorkspace.mockRejectedValue(
      new Error(
        'workspace: lookup workspace "ws_previous_daemon" by name fallback: workspace not found'
      )
    );

    const { result } = renderHook(() => useOnboardingWorkspaces());

    await act(async () => {
      await result.current.removeWorkspace(staleWorkspace.path);
    });

    expect(mocks.deleteWorkspace).not.toHaveBeenCalled();
    expect(result.current.resolveError).toBeNull();
    expect(useOnboardingDraftStore.getState().workspaces).toEqual([]);
  });

  it("deletes the registered workspace before removing a resolved folder from the draft", async () => {
    const registeredWorkspace = workspace({
      id: "ws_project",
      root_dir: "/Users/operator/project",
      name: "project",
    });
    mocks.resolveWorkspace.mockResolvedValue(registeredWorkspace);
    mocks.deleteWorkspace.mockImplementation(async () => {
      mocks.registeredWorkspaces = [];
    });

    const { result, rerender } = renderHook(() => useOnboardingWorkspaces());

    await act(async () => {
      await result.current.addWorkspace("/Users/operator/project");
    });
    await waitFor(() => {
      expect(result.current.workspaces).toEqual([
        { path: "/Users/operator/project", name: "project", workspaceId: "ws_project" },
      ]);
    });
    mocks.registeredWorkspaces = [registeredWorkspace];
    rerender();

    await act(async () => {
      await result.current.removeWorkspace("/Users/operator/project");
    });

    expect(mocks.deleteWorkspace).toHaveBeenCalledWith("ws_project");
    expect(useOnboardingDraftStore.getState().workspaces).toEqual([]);
  });

  it("keeps the folder in the draft when removing its registered workspace fails", async () => {
    act(() => {
      useOnboardingDraftStore.getState().addWorkspace({
        path: "/Users/operator/project",
        name: "project",
        workspaceId: "ws_project",
      });
    });
    mocks.registeredWorkspaces = [
      workspace({
        id: "ws_project",
        root_dir: "/Users/operator/project",
        name: "project",
      }),
    ];
    mocks.deleteWorkspace.mockRejectedValue(new Error("workspace has active sessions"));

    const { result } = renderHook(() => useOnboardingWorkspaces());

    await act(async () => {
      await result.current.removeWorkspace("/Users/operator/project");
    });

    expect(mocks.deleteWorkspace).toHaveBeenCalledWith("ws_project");
    expect(result.current.resolveError).toBe("workspace has active sessions");
    expect(useOnboardingDraftStore.getState().workspaces).toEqual([
      { path: "/Users/operator/project", name: "project", workspaceId: "ws_project" },
    ]);
  });
});
