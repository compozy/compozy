import { QueryClient, QueryClientProvider, useInfiniteQuery } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const getNetworkUsage = vi.hoisted(() => vi.fn());

vi.mock("../../adapters/network-coordination-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../adapters/network-coordination-api")>()),
  getNetworkUsage,
}));

import { networkUsageOptions } from "../../lib/query-options";

function usagePage(wakeId: string, nextCursor?: string) {
  return {
    workspace_id: "ws-1",
    details: [
      {
        wake_id: wakeId,
        task_run_id: "run-1",
        participation_status: {
          owner: { workspace_id: "ws-1", kind: "task_run", id: "run-1" },
          available: true,
          participating: true,
        },
        workspace_id: "ws-1",
        channel: "builders",
        root_id: wakeId,
        depth: 0,
        state: "succeeded",
        usage_state: "actual",
        charged_wall_time: "1ms",
        input_tokens: 1,
        output_tokens: 1,
        reserved_at: "2026-07-16T12:00:00Z",
      },
    ],
    total: {
      wake_count: 2,
      reserved_wake_count: 0,
      actual_wake_count: 2,
      unavailable_wake_count: 0,
      charged_wall_time: "2ms",
      input_tokens: 2,
      output_tokens: 2,
    },
    next_cursor: nextCursor,
  };
}

describe("networkUsageOptions", () => {
  beforeEach(() => {
    getNetworkUsage.mockReset();
    getNetworkUsage.mockImplementation((_workspaceId: string, query: { cursor?: string }) =>
      Promise.resolve(
        query.cursor === "cursor-2" ? usagePage("wake-1") : usagePage("wake-2", "cursor-2")
      )
    );
  });

  it("Should keep filter-scoped pages while refreshing the full cursor chain", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const options = networkUsageOptions(
      "ws-1",
      { owner_kind: "task_run", owner_id: "run-1", channel: "builders", limit: 1 },
      true
    );
    const { result } = renderHook(() => useInfiniteQuery(options), { wrapper });

    await waitFor(() => expect(result.current.data?.pages).toHaveLength(1));
    await act(async () => {
      await result.current.fetchNextPage();
    });
    await waitFor(() =>
      expect(result.current.data?.pages.map(page => page.details[0]?.wake_id)).toEqual([
        "wake-2",
        "wake-1",
      ])
    );

    const refresh = queryClient.invalidateQueries({ queryKey: options.queryKey });
    expect(result.current.data?.pages).toHaveLength(2);
    await act(async () => {
      await refresh;
    });
    await waitFor(() => expect(result.current.data?.pages).toHaveLength(2));
    expect(getNetworkUsage).toHaveBeenCalledWith(
      "ws-1",
      expect.objectContaining({
        owner_kind: "task_run",
        owner_id: "run-1",
        channel: "builders",
        limit: 1,
      }),
      expect.any(AbortSignal)
    );
    expect(getNetworkUsage.mock.calls.some(([, query]) => query.cursor === "cursor-2")).toBe(true);
  });
});
