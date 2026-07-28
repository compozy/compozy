import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

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

const triggersState = vi.hoisted(() => ({
  error: null as Error | null,
  triggers: [] as Array<{ id: string }>,
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
    useAutomationTriggers: () => ({
      error: triggersState.error,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      isLoading: false,
      total: triggersState.triggers.length,
      triggers: triggersState.triggers,
    }),
    useCreateAutomationTrigger: () => ({ isPending: false, mutateAsync: vi.fn() }),
    useDeleteAutomationTrigger: () => ({ isPending: false, mutateAsync: vi.fn() }),
    useUpdateAutomationTrigger: () => ({ isPending: false, mutateAsync: vi.fn() }),
  };
});

const { useAutomationTriggersPage } =
  await import("@/systems/os/apps/automation/use-automation-triggers-page");

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useAutomationTriggersPage", () => {
  beforeEach(() => {
    triggersState.error = null;
    triggersState.triggers = [];
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

  it("Should wait for an active workspace before consuming a loop-target trigger seed", async () => {
    const { result, rerender } = renderHook(
      () => useAutomationTriggersPage({ loop: "software-delivery" }),
      { wrapper: createWrapper() }
    );

    // Cold deep-link load: the workspace is not resolved yet, so the seed must
    // not open a create draft with an unbound workspace target.
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

  it("Should preserve loaded triggers while exposing a later query failure", () => {
    triggersState.triggers = [{ id: "trigger-1" }];
    triggersState.error = new Error("Triggers refresh failed");

    const { result } = renderHook(() => useAutomationTriggersPage(), {
      wrapper: createWrapper(),
    });

    expect(result.current.triggers).toHaveLength(1);
    expect(result.current.error).toBeNull();
    expect(result.current.errorMessage).toBe("Triggers refresh failed");
  });
});
