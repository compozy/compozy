// Suite: exit-plan read follow-up
// Invariant: a settled exit-plan read rereads the worktree status key once, because the daemon
// persisted a fresh status for that read without announcing it on the stream; the plan read never
// rereads itself, so the strip follows the plan without a read loop. Owning layer: workspace exit
// hooks. Canonical suite: this hook test.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { workspaceKeys } from "../../lib/query-keys";
import type { WorktreeExitPlanPayload } from "../../types";
import { useRunWorktreeExitAction, useWorktreeExitPlan } from "../use-worktree-exit";

const mocks = vi.hoisted(() => ({
  fetchWorktreeExitPlan: vi.fn(),
  runWorktreeExitAction: vi.fn(),
}));

vi.mock("../../adapters/worktree-exit-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../adapters/worktree-exit-api")>()),
  fetchWorktreeExitPlan: mocks.fetchWorktreeExitPlan,
  runWorktreeExitAction: mocks.runWorktreeExitAction,
}));

const WORKSPACE_ID = "ws-a";
const WORKTREE_ID = "wt-a";
const PLAN = { worktree_id: WORKTREE_ID, actions: [] } as unknown as WorktreeExitPlanPayload;
/** The suite runs under turbo alongside lint and typecheck, so settles wait longer than the default. */
const SETTLE = { timeout: 5_000 };

function setup(cachedPlan?: WorktreeExitPlanPayload) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: cachedPlan ? 60_000 : 0 } },
  });
  if (cachedPlan) {
    queryClient.setQueryData(workspaceKeys.worktreeExit(WORKSPACE_ID, WORKTREE_ID), cachedPlan);
  }
  const invalidate = vi.spyOn(queryClient, "invalidateQueries");
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  const view = renderHook(() => useWorktreeExitPlan(WORKSPACE_ID, WORKTREE_ID), { wrapper });
  return { invalidate, view };
}

describe("useWorktreeExitPlan", () => {
  it("Should reread the status key once after each settled plan read and never the plan itself", async () => {
    mocks.fetchWorktreeExitPlan.mockResolvedValue(PLAN);
    const { invalidate, view } = setup();

    await waitFor(() => expect(view.result.current.data).toBe(PLAN), SETTLE);
    await act(async () => {});
    await waitFor(() => expect(invalidate).toHaveBeenCalledTimes(1), SETTLE);
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: workspaceKeys.worktreeStatus(WORKSPACE_ID, WORKTREE_ID),
      exact: true,
    });
    expect(mocks.fetchWorktreeExitPlan).toHaveBeenCalledTimes(1);

    await act(async () => {
      await view.result.current.refetch();
    });
    await waitFor(() => expect(invalidate).toHaveBeenCalledTimes(2), SETTLE);
    expect(mocks.fetchWorktreeExitPlan).toHaveBeenCalledTimes(2);
  });

  it("Should reread the status key after a plan read that fails", async () => {
    mocks.fetchWorktreeExitPlan.mockRejectedValue(new Error("plan read failed"));
    const { invalidate, view } = setup();

    await waitFor(() => expect(view.result.current.isError).toBe(true), SETTLE);
    await waitFor(() => expect(invalidate).toHaveBeenCalledTimes(1), SETTLE);
  });

  it("Should not reread the status key for a plan already cached at mount", async () => {
    mocks.fetchWorktreeExitPlan.mockClear();
    const { invalidate, view } = setup(PLAN);

    expect(view.result.current.data).toBe(PLAN);
    await act(async () => {});
    expect(invalidate).not.toHaveBeenCalled();
  });
});

describe("useRunWorktreeExitAction", () => {
  it("Should reread the exit scope when the daemon refuses the action", async () => {
    mocks.runWorktreeExitAction.mockRejectedValue(new Error("action refused"));
    const queryClient = new QueryClient();
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const view = renderHook(() => useRunWorktreeExitAction(WORKSPACE_ID, WORKTREE_ID), {
      wrapper,
    });

    await act(async () => {
      await view.result.current.mutateAsync({} as never).catch(() => undefined);
    });
    await waitFor(
      () =>
        expect(invalidate).toHaveBeenCalledWith({
          queryKey: workspaceKeys.worktreeExit(WORKSPACE_ID, WORKTREE_ID),
          exact: true,
        }),
      SETTLE
    );
  });
});
