// Suite: Loops window route parser
// Invariant: a window route names exactly one Loops surface, with its path segments decoded and
// its search validated at the boundary; the window and its loading fallback read the same table.
import { describe, expect, it } from "vitest";

import { parseLoopWindowLocation } from "../loop-window-location";

describe("parseLoopWindowLocation", () => {
  it.each([
    ["/loops", "catalog"],
    ["/loops/ship-it", "detail"],
    ["/loops/ship-it/run", "run-form"],
    ["/loops/ship-it/editor", "editor"],
    ["/loops/ship-it/configure", "configure"],
    ["/loop-runs", "runs"],
    ["/loop-runs/r1", "run"],
    ["/loop-runs/r1/diff", "run-diff"],
    ["/anything-else", "catalog"],
  ] as const)("Should read %s as the %s surface", (pathname, kind) => {
    expect(parseLoopWindowLocation({ pathname, search: {} }).kind).toBe(kind);
  });

  it("Should decode the Loop name and run id segments, keeping a malformed escape as written", () => {
    expect(parseLoopWindowLocation({ pathname: "/loops/ship%20it", search: {} })).toEqual({
      kind: "detail",
      name: "ship it",
    });
    expect(parseLoopWindowLocation({ pathname: "/loop-runs/%E0%A4%A", search: {} })).toMatchObject({
      kind: "run",
      runId: "%E0%A4%A",
    });
  });

  it("Should validate each surface's search at the window boundary", () => {
    expect(
      parseLoopWindowLocation({ pathname: "/loops", search: { view: "cards", q: "  deploy " } })
    ).toMatchObject({ kind: "catalog", search: { view: "cards", q: "deploy" } });
    expect(
      parseLoopWindowLocation({
        pathname: "/loop-runs",
        search: { nodes: "waiting", view: "bogus" },
      })
    ).toMatchObject({ kind: "runs", search: { nodes: "waiting", view: undefined } });
    expect(
      parseLoopWindowLocation({ pathname: "/loop-runs/r1/diff", search: { against_run: "r0" } })
    ).toMatchObject({ kind: "run-diff", search: { against_run: "r0" } });
  });

  it("Should carry a request focus only when the route names a node and a valid item", () => {
    expect(
      parseLoopWindowLocation({
        pathname: "/loop-runs/r1",
        search: { request_node: " review ", request_item: "2" },
      })
    ).toEqual({ kind: "run", runId: "r1", requestFocus: { nodeId: "review", itemIndex: 2 } });
    expect(
      parseLoopWindowLocation({
        pathname: "/loop-runs/r1",
        search: { request_node: "review", request_item: "-1" },
      })
    ).toEqual({ kind: "run", runId: "r1", requestFocus: undefined });
  });
});
