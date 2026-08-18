import { describe, expect, it } from "vitest";

import {
  goalTurnsOptions,
  loopConfigOptions,
  loopDetailOptions,
  loopRequestDetailOptions,
  loopRequestsOptions,
  loopRunDetailOptions,
  loopRunDiffOptions,
  loopRunsOptions,
  loopsCatalogOptions,
} from "../query-options";

describe("loop query-options", () => {
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

  it("Should stop paging the request inventory when the daemon returns no cursor", () => {
    const { getNextPageParam } = loopRequestsOptions("ws_a");
    const page = { aggregates: { pending: 2 }, items: [], next_cursor: "" };
    expect(getNextPageParam(page, [page], undefined, [undefined])).toBeUndefined();
    expect(getNextPageParam({ ...page, next_cursor: "c2" }, [page], undefined, [undefined])).toBe(
      "c2"
    );
  });

  it("Should gate the request detail read on workspace, run, and node", () => {
    expect(loopRequestDetailOptions("ws_a", "run_1", "ask_node", 1).queryKey).toEqual([
      "loops",
      "requests",
      "detail",
      "ws_a",
      "run_1",
      "ask_node",
      "1",
    ]);
    expect(loopRequestDetailOptions("", "run_1", "ask_node").enabled).toBe(false);
    expect(loopRequestDetailOptions("ws_a", "", "ask_node").enabled).toBe(false);
    expect(loopRequestDetailOptions("ws_a", "run_1", "").enabled).toBe(false);

    expect(loopRequestDetailOptions("ws_a", "run_1", "ask_node", 0, false).enabled).toBe(false);
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
