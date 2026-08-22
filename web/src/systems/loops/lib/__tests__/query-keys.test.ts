import { describe, expect, it } from "vitest";

import { loopsKeys } from "../query-keys";

describe("loopsKeys", () => {
  it("Should scope every read key by workspace so a workspace switch never serves another cache", () => {
    expect(loopsKeys.catalog("ws_a")).toEqual(["loops", "catalog", "ws_a", "", "", "", "", "", ""]);
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
      "",
    ]);
    expect(loopsKeys.runs("ws_a")).toEqual([
      "loops",
      "runs",
      "ws_a",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
    ]);
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
  });

  it("Should key Goal turn pages by their stable run and axis filters", () => {
    expect(loopsKeys.goalTurns("ws_a", "run_1", { node: "goal", item: 0, limit: 50 })).toEqual([
      "loops",
      "goal-turns",
      "ws_a",
      "run_1",
      "goal",
      "0",
      "50",
    ]);
    expect(loopsKeys.goalTurns("ws_a", "run_1")).not.toEqual(loopsKeys.goalTurns("ws_a", "run_2"));
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
    expect(loopsKeys.runRequestCounts("ws_a")).toEqual(["loops", "requests", "ws_a", "run-counts"]);
    expect(loopsKeys.runRequestCounts("ws_a")).not.toEqual(
      loopsKeys.requests("ws_a", { state: "pending", run_id: "run_1", limit: 1 })
    );
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
});
