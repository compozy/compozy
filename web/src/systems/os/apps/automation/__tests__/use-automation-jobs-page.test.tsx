import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AutomationApiError } from "@/systems/automation";

const pageState = vi.hoisted(() => ({
  current: {
    activeWorkspace: null,
    activeWorkspaceId: null as string | null,
    automationRuntime: null,
    hasChildMatch: false,
    listFilters: { limit: 50 },
    scopeFilter: "all" as const,
    searchQuery: "",
    setScopeFilter: vi.fn(),
    setSearchQuery: vi.fn(),
    workspaces: [],
  },
}));

const jobsState = vi.hoisted(() => ({
  error: null as Error | null,
  jobs: [] as Array<{ id: string }>,
  triggerJob: vi.fn(),
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => vi.fn(),
}));

vi.mock("@/systems/os/apps/automation/use-automation-page-base", async importOriginal => {
  const actual =
    await importOriginal<typeof import("@/systems/os/apps/automation/use-automation-page-base")>();
  return {
    ...actual,
    useAutomationPageBase: () => pageState.current,
  };
});

vi.mock("@/systems/automation", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/automation")>();
  return {
    ...actual,
    useAutomationJobs: () => ({
      error: jobsState.error,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      isLoading: false,
      jobs: jobsState.jobs,
      total: jobsState.jobs.length,
    }),
    useAutomationJob: () => ({ data: null, error: null, isLoading: false }),
    useAutomationJobRuns: () => ({ data: [], error: null, isLoading: false }),
    useCreateAutomationJob: () => ({ isPending: false, mutateAsync: vi.fn() }),
    useDeleteAutomationJob: () => ({ isPending: false, mutateAsync: vi.fn() }),
    useTriggerAutomationJob: () => ({ isPending: false, mutateAsync: jobsState.triggerJob }),
    useUpdateAutomationJob: () => ({ isPending: false, mutateAsync: vi.fn() }),
  };
});

const { useAutomationJobsPage } =
  await import("@/systems/os/apps/automation/use-automation-jobs-page");

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useAutomationJobsPage", () => {
  beforeEach(() => {
    jobsState.error = null;
    jobsState.jobs = [];
    jobsState.triggerJob.mockReset();
    pageState.current = {
      activeWorkspace: null,
      activeWorkspaceId: null,
      automationRuntime: null,
      hasChildMatch: false,
      listFilters: { limit: 50 },
      scopeFilter: "all",
      searchQuery: "",
      setScopeFilter: vi.fn(),
      setSearchQuery: vi.fn(),
      workspaces: [],
    };
  });

  it("Should wait for an active workspace before consuming a loop-target job seed", async () => {
    const { result, rerender } = renderHook(
      () => useAutomationJobsPage({ loop: "software-delivery" }),
      { wrapper: createWrapper() }
    );

    await act(async () => {});
    expect(result.current.editorDialogProps.editor).toBeNull();

    pageState.current = {
      ...pageState.current,
      activeWorkspaceId: "ws_default",
    };
    rerender();

    await waitFor(() =>
      expect(result.current.editorDialogProps.editor?.draft.loop_target).toMatchObject({
        loop_name: "software-delivery",
        workspace_id: "ws_default",
      })
    );
    expect(result.current.editorDialogProps.editor?.draft.workspace_id).toBe("ws_default");
  });

  it("Should preserve loaded jobs while exposing a later query failure", () => {
    jobsState.jobs = [{ id: "job-1" }];
    jobsState.error = new Error("Jobs refresh failed");

    const { result } = renderHook(() => useAutomationJobsPage(), { wrapper: createWrapper() });

    expect(result.current.jobs).toHaveLength(1);
    expect(result.current.error).toBeNull();
    expect(result.current.errorMessage).toBe("Jobs refresh failed");
  });

  it("Should block cached job runs while the automation runtime is unavailable", () => {
    jobsState.jobs = [{ id: "job-1" }];
    jobsState.error = new AutomationApiError("runtime unavailable", 503);

    const { result } = renderHook(() => useAutomationJobsPage(), { wrapper: createWrapper() });

    expect(result.current.jobs).toHaveLength(1);
    expect(result.current.runDisabled).toBe(true);
    act(() => result.current.onRunJob("job-1"));
    expect(jobsState.triggerJob).not.toHaveBeenCalled();
  });

  it("Should keep every job pending until its own manual run request settles", async () => {
    const resolvers = new Map<string, (value: { id: string }) => void>();
    jobsState.triggerJob.mockImplementation(
      ({ id }: { id: string }) =>
        new Promise(resolve => {
          resolvers.set(id, resolve);
        })
    );
    const { result } = renderHook(() => useAutomationJobsPage(), { wrapper: createWrapper() });

    act(() => {
      result.current.onRunJob("job-a");
      result.current.onRunJob("job-b");
      result.current.onRunJob("job-a");
    });

    await waitFor(() => {
      expect([...result.current.runPendingIds]).toEqual(["job-a", "job-b"]);
    });
    expect(jobsState.triggerJob).toHaveBeenCalledTimes(2);

    await act(async () => {
      resolvers.get("job-a")?.({ id: "run-a" });
    });
    await waitFor(() => {
      expect([...result.current.runPendingIds]).toEqual(["job-b"]);
    });

    await act(async () => {
      resolvers.get("job-b")?.({ id: "run-b" });
    });
    await waitFor(() => expect(result.current.runPendingIds.size).toBe(0));
  });
});
