import { describe, expect, it } from "vitest";

import {
  loopConfigOptions,
  loopDetailOptions,
  loopRequestDetailOptions,
  loopRequestAttentionOptions,
  loopRequestsOptions,
  loopRunDetailOptions,
  loopRunDiffOptions,
  loopRunBriefingOptions,
  loopRunRosterOptions,
  loopRunTimelineOptions,
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

  it("Should key the bell cache exactly and disable it without a workspace", () => {
    expect(loopRequestAttentionOptions("ws_a").queryKey).toEqual([
      "loops",
      "requests",
      "ws_a",
      "attention",
    ]);
    expect(loopRequestAttentionOptions("", true).enabled).toBe(false);
    expect(loopRequestAttentionOptions("ws_a", true, false).refetchInterval).toBe(false);
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

  it("Should leave the fenced story to the stream instead of re-anchoring it", () => {
    // Every read of the newest window is unpinned, so anything that triggers one
    // slides the loaded window up and drops the oldest pages. A timer or a window
    // regaining focus is not a lifecycle event and has no business doing that.
    const options = loopRunTimelineOptions("ws_a", "run_1");
    expect(options.refetchOnWindowFocus).toBe(false);
    expect(options.refetchInterval).toBeUndefined();
    expect(options.queryKey).toEqual([
      "loops",
      "run-reads",
      "ws_a",
      "run_1",
      "timeline",
      "notable",
      "50",
      "",
    ]);
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
  it("Should stop polling the briefing once the run is terminal", () => {
    const options = loopRunBriefingOptions("ws_a", "run_1");
    expect(options.queryKey).toEqual(["loops", "run-reads", "ws_a", "run_1", "briefing"]);
    const asFn = options.refetchInterval as (query: {
      state: { data?: { status: string } };
    }) => number | false;
    // A terminal run's briefing is immutable; polling it forever is pure noise.
    expect(asFn({ state: { data: { status: "done" } } })).toBe(false);
    expect(asFn({ state: { data: { status: "running" } } })).toBeGreaterThan(0);
    expect(asFn({ state: { data: undefined } })).toBeGreaterThan(0);

    expect(loopRunBriefingOptions("", "run_1").enabled).toBe(false);
    expect(loopRunBriefingOptions("ws_a", "").enabled).toBe(false);
    expect(loopRunBriefingOptions("ws_a", "run_1", false).enabled).toBe(false);
  });

  it("Should page the roster on its served cursor and stop polling a terminal run", () => {
    const options = loopRunRosterOptions("ws_a", "run_1", { state: "failed" });
    // The page size is normalized into the key, so two page sizes are two caches.
    expect(options.queryKey).toEqual([
      "loops",
      "run-reads",
      "ws_a",
      "run_1",
      "roster",
      "failed",
      "",
      "200",
    ]);
    expect(options.initialPageParam).toBeUndefined();
    // Continuation is the daemon's opaque cursor, never a client-computed offset.
    expect(options.getNextPageParam({ next_cursor: "cur_2" } as never, [], undefined, [])).toBe(
      "cur_2"
    );
    expect(
      options.getNextPageParam({ next_cursor: "" } as never, [], undefined, [])
    ).toBeUndefined();

    const asFn = options.refetchInterval as (query: {
      state: { data?: { pages: { run_status: string }[] } };
    }) => number | false;
    expect(asFn({ state: { data: { pages: [{ run_status: "done" }] } } })).toBe(false);
    expect(asFn({ state: { data: { pages: [{ run_status: "running" }] } } })).toBeGreaterThan(0);

    expect(loopRunRosterOptions("", "run_1").enabled).toBe(false);
    expect(loopRunRosterOptions("ws_a", "").enabled).toBe(false);
    expect(loopRunRosterOptions("ws_a", "run_1", {}, false).enabled).toBe(false);
  });

  it("Should default the story to the notable view and page it backward on a cursor", () => {
    const options = loopRunTimelineOptions("ws_a", "run_1");
    expect(options.initialPageParam).toBeUndefined();
    expect(options.getNextPageParam({ next_cursor: "cur_9" } as never, [], undefined, [])).toBe(
      "cur_9"
    );
    expect(
      options.getNextPageParam({ next_cursor: undefined } as never, [], undefined, [])
    ).toBeUndefined();
    // `all` is a separate fenced history, so it must not share the cache.
    expect(loopRunTimelineOptions("ws_a", "run_1", { view: "all" }).queryKey).not.toEqual(
      options.queryKey
    );

    expect(loopRunTimelineOptions("", "run_1").enabled).toBe(false);
    expect(loopRunTimelineOptions("ws_a", "").enabled).toBe(false);
    expect(loopRunTimelineOptions("ws_a", "run_1", {}, false).enabled).toBe(false);
  });
});
