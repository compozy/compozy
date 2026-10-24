import { describe, expect, it } from "vitest";

import { bridgeFilterChange, LOOP_AGGREGATE_KEY, loopsKeys, loopsPopulation } from "../query-keys";

describe("loopsKeys", () => {
  it("Should scope every read key by workspace so a workspace switch never serves another cache", () => {
    expect(loopsKeys.catalog("ws_a")).toEqual([
      "loops",
      "catalog",
      "ws_a",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
    ]);
    expect(loopsKeys.detail("ws_a", "delivery")).toEqual(["loops", "detail", "ws_a", "delivery"]);
    expect(loopsKeys.config("ws_a", "delivery")).toEqual(["loops", "config", "ws_a", "delivery"]);
    expect(loopsKeys.annotations("ws_a", "delivery")).toEqual([
      "loops",
      "annotations",
      "ws_a",
      "delivery",
    ]);
    expect(loopsKeys.runDetail("ws_a", "run_1")).toEqual(["loops", "run-detail", "ws_a", "run_1"]);

    expect(loopsKeys.catalog("ws_a")).not.toEqual(loopsKeys.catalog("ws_b"));
  });

  it("Should include every stable catalog filter while keeping cursor out of the key", () => {
    expect(
      loopsKeys.catalog("ws_a", {
        category: "Engineering",
        kind: "read_only",
        limit: 25,
        q: "delivery",
        sort: "name",
        status: "running",
      })
    ).toEqual([
      "loops",
      "catalog",
      "ws_a",
      "delivery",
      "read_only",
      "Engineering",
      "running",
      "name",
      "25",
      "",
    ]);
  });

  it("Should nest each key under its root so invalidation can target a scope or everything", () => {
    expect(loopsKeys.detail("ws_a", "delivery").slice(0, 2)).toEqual(loopsKeys.details());
    expect(loopsKeys.runDetail("ws_a", "run_1").slice(0, 2)).toEqual(loopsKeys.runDetails());
    expect(loopsKeys.runs("ws_a").slice(0, 2)).toEqual(loopsKeys.runsRoot());
    expect(loopsKeys.catalog("ws_a")[0]).toBe(loopsKeys.all[0]);
  });

  it("Should scope runs invalidation to one workspace across every filter permutation", () => {
    expect(loopsKeys.runsByWorkspace("ws_a")).toEqual(["loops", "runs", "ws_a"]);
    // The per-workspace key is a prefix of every filtered runs key for that workspace...
    const filtered = loopsKeys.runs("ws_a", { loop: "delivery", status: "running", limit: 5 });
    expect(filtered.slice(0, 3)).toEqual(loopsKeys.runsByWorkspace("ws_a"));
    // ...but never a prefix of another workspace's runs.
    expect(loopsKeys.runs("ws_b").slice(0, 3)).not.toEqual(loopsKeys.runsByWorkspace("ws_a"));
  });

  it("Should fold every runs filter dimension into a stable, order-independent key", () => {
    expect(
      loopsKeys.runs("ws_a", {
        loop: "delivery",
        status: "running",
        origin: "session",
        origin_session: "session_1",
        live: true,
        limit: 20,
        profile: "marketing",
      })
    ).toEqual([
      "loops",
      "runs",
      "ws_a",
      "delivery",
      "running",
      "session",
      "session_1",
      true,
      "20",
      "marketing",
    ]);
    expect(loopsKeys.runs("ws_a")).toEqual(["loops", "runs", "ws_a", "", "", "", "", "", "", ""]);
    expect(loopsKeys.runs("ws_a", { loop: "delivery" })).not.toEqual(
      loopsKeys.runs("ws_a", { loop: "watch" })
    );
    // Runs are profile-owned work, so two profiles reading one workspace are two
    // lists — and the aggregate is neither of them.
    expect(loopsKeys.runs("ws_a", { profile: "marketing" })).not.toEqual(
      loopsKeys.runs("ws_a", { profile: "default" })
    );
    expect(loopsKeys.runs("ws_a", { all_profiles: true })).not.toEqual(
      loopsKeys.runs("ws_a", { profile: "default" })
    );
    expect(loopsKeys.runs("ws_a")).toEqual(loopsKeys.runs("ws_a", { all_profiles: false }));
    expect(loopsKeys.runs("ws_a", { all_profiles: true }).at(-1)).toBe("@all");
  });

  it("Should encode the stream resume seed distinctly per run and after_sequence", () => {
    expect(loopsKeys.stream("ws_a", "run_1", { after_sequence: "14" })).toEqual([
      "loops",
      "stream",
      "ws_a",
      "run_1",
      "14",
    ]);
    expect(loopsKeys.stream("ws_a", "run_1")).toEqual(["loops", "stream", "ws_a", "run_1", ""]);
  });

  it("Should key the request inventory by state and run while keeping cursor out of the key", () => {
    expect(loopsKeys.requests("ws_a", { state: "pending", run_id: "run_1", limit: 50 })).toEqual([
      "loops",
      "requests",
      "ws_a",
      "pending",
      "run_1",
      "50",
      "",
    ]);
    expect(loopsKeys.requests("ws_a", { state: "pending" })).not.toEqual(
      loopsKeys.requests("ws_a", { state: "resolved" })
    );
    expect(loopsKeys.requests("ws_a").slice(0, 3)).toEqual(loopsKeys.requestsByWorkspace("ws_a"));
    expect(loopsKeys.requestsByWorkspace("ws_a")).not.toEqual(
      loopsKeys.requestsByWorkspace("ws_b")
    );
  });

  it("Should address one request by run, node, and fan-out lane", () => {
    expect(loopsKeys.requestDetail("ws_a", "run_1", 3, "confirm-rollout", 2)).toEqual([
      "loops",
      "requests",
      "detail",
      "ws_a",
      "run_1",
      3,
      "confirm-rollout",
      "2",
    ]);

    expect(loopsKeys.requestDetail("ws_a", "run_1", 3, "confirm-rollout")).toEqual([
      "loops",
      "requests",
      "detail",
      "ws_a",
      "run_1",
      3,
      "confirm-rollout",
      "",
    ]);
    expect(loopsKeys.requestDetail("ws_a", "run_1", 3, "confirm-rollout", 1)).not.toEqual(
      loopsKeys.requestDetail("ws_a", "run_1", 3, "confirm-rollout", 2)
    );
  });

  it("Should isolate normal attention projections from the paged request cache", () => {
    expect(loopsKeys.requestAttention("ws_a")).toEqual(["loops", "requests", "ws_a", "attention"]);
  });

  it("Should make every compared diff pair its own key, never one filtered read", () => {
    expect(loopsKeys.runDiff("ws_a", "run_1", { generation: 2, against_generation: 3 })).toEqual([
      "loops",
      "run-diff",
      "ws_a",
      "run_1",
      "2",
      "3",
      "",
    ]);
    expect(loopsKeys.runDiff("ws_a", "run_1", { against_run: "run_2" })).toEqual([
      "loops",
      "run-diff",
      "ws_a",
      "run_1",
      "",
      "",
      "run_2",
    ]);
    expect(loopsKeys.runDiff("ws_a", "run_1", { against_generation: 2 })).not.toEqual(
      loopsKeys.runDiff("ws_a", "run_1", { against_generation: 3 })
    );
    expect(loopsKeys.runDiff("ws_a", "run_1").slice(0, 2)).toEqual(loopsKeys.runDiffRoot());
  });
  // ADR-005: three projections of one source. They share a root so a node verb
  // or a stream reconnect invalidates all three together, and each carries the
  // filters that make it a distinct population rather than a filtered copy.
  it("Should root the three run reads together and key each by its own population", () => {
    expect(loopsKeys.runBriefing("ws_a", "run_1")).toEqual([
      "loops",
      "run-reads",
      "ws_a",
      "run_1",
      "briefing",
    ]);
    expect(loopsKeys.runBriefing("ws_a", "run_1").slice(0, 4)).toEqual(
      loopsKeys.runReads("ws_a", "run_1")
    );
    expect(loopsKeys.runReads("ws_a", "run_1").slice(0, 2)).toEqual(loopsKeys.runReadsRoot());
    expect(loopsKeys.runReads("ws_a", "run_1")).not.toEqual(loopsKeys.runReads("ws_b", "run_1"));

    expect(loopsKeys.runRoster("ws_a", "run_1", { state: "failed", generation: 2 })).toEqual([
      "loops",
      "run-reads",
      "ws_a",
      "run_1",
      "roster",
      "failed",
      "2",
      "",
    ]);
    // A filtered roster is its own population with its own cursor.
    expect(loopsKeys.runRoster("ws_a", "run_1", { state: "failed" })).not.toEqual(
      loopsKeys.runRoster("ws_a", "run_1", { state: "running" })
    );
    expect(loopsKeys.runRoster("ws_a", "run_1", { generation: 1 })).not.toEqual(
      loopsKeys.runRoster("ws_a", "run_1", { generation: 2 })
    );

    // `notable` and `all` are two fenced histories, never one list filtered down.
    expect(loopsKeys.runTimeline("ws_a", "run_1", { view: "all", limit: 50 })).toEqual([
      "loops",
      "run-reads",
      "ws_a",
      "run_1",
      "timeline",
      "all",
      "50",
      "",
    ]);
    expect(loopsKeys.runTimeline("ws_a", "run_1", { view: "all" })).not.toEqual(
      loopsKeys.runTimeline("ws_a", "run_1", { view: "notable" })
    );
  });
});

// A list may bridge a filter change with its previous rows only inside one population: the same
// scope, page shape and profile lens. These cases pin which key segments that means, so a key that
// gains a segment cannot silently start serving one scope's rows under another's.
describe("loopsPopulation", () => {
  it("Should hold a catalog's population through search and filters, not through scope, page or profile", () => {
    const shape = { limit: 50, profile: "default", sort: "name" } as const;
    const base = loopsPopulation.catalog(loopsKeys.catalog("ws_a", shape));
    const narrowed = loopsKeys.catalog("ws_a", {
      ...shape,
      category: "Engineering",
      kind: "workspace",
      q: "ship",
      status: "running",
    });
    expect(loopsPopulation.catalog(narrowed)).toEqual(base);

    for (const other of [
      loopsKeys.catalog("ws_b", shape),
      loopsKeys.catalog("ws_a", { ...shape, limit: 100 }),
      loopsKeys.catalog("ws_a", { ...shape, sort: undefined }),
      loopsKeys.catalog("ws_a", { ...shape, profile: "engineering" }),
      loopsKeys.catalog("ws_a", { limit: 50, sort: "name", all_profiles: true }),
    ]) {
      expect(loopsPopulation.catalog(other)).not.toEqual(base);
    }
  });

  it("Should hold the All projects batch's population through its Project filter", () => {
    const filters = { limit: 50, sort: "name" } as const;
    const key = (projects: string) => [...loopsKeys.catalog(LOOP_AGGREGATE_KEY, filters), projects];
    expect(loopsPopulation.catalog(key("ws_a"))).toEqual(loopsPopulation.catalog(key("ws_a,ws_b")));
    expect(loopsPopulation.catalog(key("ws_a"))).not.toEqual(
      loopsPopulation.catalog(loopsKeys.catalog("ws_a", filters))
    );
  });

  it("Should hold a roster's population through origin filters, not through loop, status or scope", () => {
    const base = loopsPopulation.runs(loopsKeys.runs("ws_a", { limit: 50, profile: "default" }));
    expect(
      loopsPopulation.runs(
        loopsKeys.runs("ws_a", {
          limit: 50,
          origin: "session",
          origin_session: "ses_1",
          profile: "default",
        })
      )
    ).toEqual(base);

    for (const other of [
      loopsKeys.runs("ws_b", { limit: 50, profile: "default" }),
      loopsKeys.runs("ws_a", { limit: 50, loop: "delivery", profile: "default" }),
      loopsKeys.runs("ws_a", { limit: 50, profile: "default", status: "running" }),
      loopsKeys.runs("ws_a", { limit: 50, live: true, profile: "default" }),
      loopsKeys.runs("ws_a", { limit: 5, profile: "default" }),
      loopsKeys.runs("ws_a", { limit: 50, profile: "engineering" }),
    ]) {
      expect(loopsPopulation.runs(other)).not.toEqual(base);
    }
  });

  it("Should hold an inventory state's population through its loop and run filters only", () => {
    const base = loopsPopulation.nodeInventory(
      loopsKeys.nodeInventory("ws_a", { limit: 50, profile: "default", state: "waiting" })
    );
    expect(
      loopsPopulation.nodeInventory(
        loopsKeys.nodeInventory("ws_a", {
          limit: 50,
          loop: "delivery",
          profile: "default",
          run_id: "run_1",
          state: "waiting",
        })
      )
    ).toEqual(base);

    for (const other of [
      loopsKeys.nodeInventory("ws_a", { limit: 50, profile: "default", state: "attention" }),
      loopsKeys.nodeInventory("ws_b", { limit: 50, profile: "default", state: "waiting" }),
      loopsKeys.nodeInventory("ws_a", { limit: 50, profile: "other", state: "waiting" }),
    ]) {
      expect(loopsPopulation.nodeInventory(other)).not.toEqual(base);
    }
  });
});

describe("bridgeFilterChange", () => {
  it("Should hand the previous rows to the next read inside one population and nothing otherwise", () => {
    const next = loopsKeys.catalog("ws_a", { limit: 50, q: "b" });
    const bridge = bridgeFilterChange(loopsPopulation.catalog, next);
    const rows = { pages: [{ loops: [] }] };

    expect(bridge(rows, { queryKey: loopsKeys.catalog("ws_a", { limit: 50, q: "a" }) })).toBe(rows);
    expect(
      bridge(rows, { queryKey: loopsKeys.catalog("ws_b", { limit: 50, q: "a" }) })
    ).toBeUndefined();
    expect(bridge(rows, undefined)).toBeUndefined();
  });
});
