import { QueryClient, type QueryFunctionContext } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";

const listLoopRequestsMock = vi.hoisted(() => vi.fn());

vi.mock("../../adapters/loop-requests-api", async importOriginal => {
  const actual = await importOriginal<typeof import("../../adapters/loop-requests-api")>();
  return { ...actual, listLoopRequests: listLoopRequestsMock };
});

import {
  goalTurnsOptions,
  loopConfigOptions,
  loopDetailOptions,
  loopRequestDetailOptions,
  loopRequestAttentionOptions,
  loopRequestsOptions,
  loopRunRequestCountsOptions,
  loopRunDetailOptions,
  loopRunDiffOptions,
  loopRunsOptions,
  loopsCatalogOptions,
} from "../query-options";

function queryContext<TQueryKey extends readonly unknown[]>(queryKey: TQueryKey) {
  return {
    client: new QueryClient(),
    meta: undefined,
    queryKey,
    signal: new AbortController().signal,
  } as QueryFunctionContext<TQueryKey>;
}

describe("loop query-options", () => {
  beforeEach(() => listLoopRequestsMock.mockReset());

  it("Should key each option by its workspace-scoped query key", () => {
    expect(loopsCatalogOptions("ws_a").queryKey).toEqual([
      "loops",
      "catalog",
      "ws_a",
      "",
      "",
      "",
      "",
      "",
      "",
    ]);
    expect(loopDetailOptions("ws_a", "delivery").queryKey).toEqual([
      "loops",
      "detail",
      "ws_a",
      "delivery",
    ]);
    expect(loopRunDetailOptions("ws_a", "run_1").queryKey).toEqual([
      "loops",
      "run-detail",
      "ws_a",
      "run_1",
    ]);
    expect(goalTurnsOptions("ws_a", "run_1", { node: "build", limit: 25 }).queryKey).toEqual([
      "loops",
      "goal-turns",
      "ws_a",
      "run_1",
      "build",
      "",
      "25",
    ]);
  });

  it("Should gate detail/config/run reads on both the workspace and the resource id", () => {
    expect(loopDetailOptions("", "delivery").enabled).toBe(false);
    expect(loopDetailOptions("ws_a", "").enabled).toBe(false);
    expect(loopDetailOptions("ws_a", "delivery").enabled).toBe(true);

    expect(loopConfigOptions("ws_a", "").enabled).toBe(false);
    expect(loopRunDetailOptions("ws_a", "").enabled).toBe(false);
    expect(loopRunDetailOptions("", "run_1").enabled).toBe(false);
  });

  it("Should leave catalog enablement to the hook and gate runs on the workspace", () => {
    expect(loopsCatalogOptions("ws_a").enabled).toBeUndefined();
    expect(loopsCatalogOptions("ws_a").initialPageParam).toBeUndefined();
    expect(loopsCatalogOptions("ws_a").refetchInterval).toBeUndefined();
    expect(loopRunsOptions("ws_a").enabled).toBe(true);
    expect(loopRunsOptions("").enabled).toBe(false);
  });

  it("Should poll a live run detail but stop once the run reaches a terminal state", () => {
    const { refetchInterval } = loopRunDetailOptions("ws_a", "run_1");
    const asFn = refetchInterval as (query: {
      state: { data?: { run: { status: string } } };
    }) => number | false;
    expect(asFn({ state: { data: { run: { status: "running" } } } })).toBe(15_000);
    expect(asFn({ state: { data: { run: { status: "exhausted" } } } })).toBe(false);
    expect(asFn({ state: { data: undefined } })).toBe(15_000);
  });

  it("Should default the request inventory to pending and page by cursor", () => {
    const options = loopRequestsOptions("ws_a");
    expect(options.queryKey).toEqual(["loops", "requests", "ws_a", "pending", "", "50"]);
    expect(options.initialPageParam).toBeUndefined();
    expect(options.enabled).toBe(true);
    expect(loopRequestsOptions("").enabled).toBe(false);
    expect(loopRequestsOptions("ws_a", { state: "resolved" }).queryKey).toContain("resolved");
  });

  it("Should keep exact bell and run counts in non-paged caches", () => {
    expect(loopRequestAttentionOptions("ws_a").queryKey).toEqual([
      "loops",
      "requests",
      "ws_a",
      "attention",
    ]);
    expect(loopRequestAttentionOptions("", true).enabled).toBe(false);
    expect(loopRequestAttentionOptions("ws_a", true, false).refetchInterval).toBe(false);
    expect(loopRunRequestCountsOptions("ws_a").queryKey).toEqual([
      "loops",
      "requests",
      "ws_a",
      "run-counts",
    ]);
    expect(loopRunRequestCountsOptions("").enabled).toBe(false);
  });

  it("Should count every pending request by run across all workspace pages", async () => {
    listLoopRequestsMock
      .mockResolvedValueOnce({
        aggregates: { pending: 3 },
        items: [{ loop_run_id: "run-a" }, { loop_run_id: "run-b" }],
        next_cursor: "next",
      })
      .mockResolvedValueOnce({
        aggregates: { pending: 3 },
        items: [{ loop_run_id: "run-a" }],
        next_cursor: "",
      });
    const options = loopRunRequestCountsOptions("ws_a");
    if (typeof options.queryFn !== "function") throw new Error("Expected queryFn");

    await expect(options.queryFn(queryContext(options.queryKey))).resolves.toEqual({
      "run-a": 2,
      "run-b": 1,
    });
    expect(listLoopRequestsMock).toHaveBeenNthCalledWith(
      1,
      "ws_a",
      { state: "pending", limit: 200, cursor: undefined },
      expect.any(AbortSignal)
    );
    expect(listLoopRequestsMock).toHaveBeenNthCalledWith(
      2,
      "ws_a",
      { state: "pending", limit: 200, cursor: "next" },
      expect.any(AbortSignal)
    );
  });

  it("Should stop paging the request inventory when the daemon returns no cursor", () => {
    const { getNextPageParam } = loopRequestsOptions("ws_a");
    const page = { aggregates: { pending: 2 }, items: [], next_cursor: "" };
    expect(getNextPageParam(page, [page], undefined, [undefined])).toBeUndefined();
    expect(getNextPageParam({ ...page, next_cursor: "c2" }, [page], undefined, [undefined])).toBe(
      "c2"
    );
  });

  it("Should gate the request detail read on workspace, run, and node", () => {
    expect(loopRequestDetailOptions("ws_a", "run_1", 3, "ask_node", 1).queryKey).toEqual([
      "loops",
      "requests",
      "detail",
      "ws_a",
      "run_1",
      3,
      "ask_node",
      "1",
    ]);
    expect(loopRequestDetailOptions("", "run_1", 3, "ask_node").enabled).toBe(false);
    expect(loopRequestDetailOptions("ws_a", "", 3, "ask_node").enabled).toBe(false);
    expect(loopRequestDetailOptions("ws_a", "run_1", 3, "").enabled).toBe(false);

    expect(loopRequestDetailOptions("ws_a", "run_1", 3, "ask_node", 0, false).enabled).toBe(false);
  });

  it("Should keep refreshing a diff while either compared side is still executing", () => {
    const { refetchInterval } = loopRunDiffOptions("ws_a", "run_1", { against_run: "run_2" });
    const asFn = refetchInterval as (query: {
      state: { data?: { against: { status: string }; base: { status: string } } };
    }) => number | false;
    expect(
      asFn({ state: { data: { base: { status: "done" }, against: { status: "running" } } } })
    ).toBe(15_000);
    expect(
      asFn({ state: { data: { base: { status: "done" }, against: { status: "canceled" } } } })
    ).toBe(false);
    expect(asFn({ state: { data: undefined } })).toBe(15_000);
  });
});
