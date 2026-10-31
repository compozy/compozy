// Suite: worktree materialization hook
// Invariant (#1078): a failed creation stays listed with its cause and keeps its name until
// dismissed, so the hook reports that cause and Retry releases the failed row before repeating the
// same request. Owning layer: workspace materialization hook. Canonical suite: this hook test.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const createWorktreeMock = vi.fn();
const dismissWorktreeMock = vi.fn();
const listing = vi.hoisted(() => ({ current: undefined as unknown }));

vi.mock("../use-worktrees", () => ({
  useCreateWorktree: () => ({ mutateAsync: createWorktreeMock }),
  useCancelWorktreeCreate: () => ({ mutateAsync: vi.fn() }),
  useDismissWorktree: () => ({ mutateAsync: dismissWorktreeMock }),
  useWorktrees: () => ({ data: listing.current, isFetched: true, isFetching: false }),
}));

import { buildWorktreeFixture, worktreeListingFixture } from "../../mocks/worktree-fixtures";
import { workspaceKeys } from "../../lib/query-keys";
import { useWorktreeMaterialization } from "../use-worktree-materialization";

const WORKSPACE = "ws_launch_hq";

const pendingRow = buildWorktreeFixture({
  id: "wt_failed_checkout",
  name: "big-one",
  state: "pending",
  pending_phase: "checkout",
});

const failedRow = buildWorktreeFixture({
  ...pendingRow,
  state: "failed",
  create_error: "checkout phase failed: worktree: checkout exceeded worktrees.checkout_timeout",
});

function setListing(queryClient: QueryClient, worktrees: (typeof pendingRow)[]) {
  listing.current = { ...worktreeListingFixture, worktrees };
  queryClient.setQueryData(workspaceKeys.worktrees(WORKSPACE), listing.current);
}

function renderMaterialization() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return { queryClient, ...renderHook(() => useWorktreeMaterialization(WORKSPACE), { wrapper }) };
}

describe("useWorktreeMaterialization", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listing.current = undefined;
  });

  it("Should report the recorded cause and dismiss the failed row before Retry", async () => {
    createWorktreeMock.mockResolvedValueOnce(pendingRow);
    const { result, rerender, queryClient } = renderMaterialization();

    // The accepted row is already in the catalog when the 202 resolves.
    act(() => {
      setListing(queryClient, [pendingRow]);
      rerender();
    });
    act(() => result.current.start("big-one"));
    await waitFor(() => expect(result.current.status).toBe("pending"));

    act(() => {
      setListing(queryClient, [failedRow]);
      rerender();
    });
    expect(result.current.status).toBe("failed");
    expect(result.current.error).toBe(failedRow.create_error);

    const retried = buildWorktreeFixture({
      ...pendingRow,
      id: "wt_retry",
      pending_phase: "branch",
    });
    dismissWorktreeMock.mockResolvedValueOnce({});
    createWorktreeMock.mockResolvedValueOnce(retried);
    act(() => result.current.retry());

    await waitFor(() => expect(createWorktreeMock).toHaveBeenCalledTimes(2));
    expect(dismissWorktreeMock).toHaveBeenCalledWith({ worktreeID: failedRow.id });
    expect(dismissWorktreeMock.mock.invocationCallOrder[0]).toBeLessThan(
      createWorktreeMock.mock.invocationCallOrder[1]
    );
    expect(createWorktreeMock).toHaveBeenLastCalledWith({ name: "big-one" });
  });
});
