import { describe, expect, it } from "vitest";

import {
  goalTurnsOptions,
  loopConfigOptions,
  loopDetailOptions,
  loopRunDetailOptions,
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
});
