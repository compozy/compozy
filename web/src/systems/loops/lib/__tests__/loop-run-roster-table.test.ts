import { describe, expect, it } from "vitest";

import type { LoopFanoutRollup, LoopRosterNode } from "../../types";
import type { LoopGraph } from "../loop-graph";
import { buildRosterTable } from "../loop-run-roster-table";

const NOW = Date.parse("2026-08-19T18:50:00Z");

const GRAPH: LoopGraph = {
  nodes: [
    {
      id: "implementar",
      nodeClass: "action",
      kind: "run-agent",
      isGate: false,
      eventsCount: 0,
      routes: [],
      hasAskExpect: false,
    },
    {
      id: "revisores",
      nodeClass: "control",
      kind: "fan-out",
      isGate: false,
      eventsCount: 0,
      routes: [],
      hasAskExpect: false,
    },
    {
      id: "revisar",
      nodeClass: "action",
      kind: "run-agent",
      isGate: false,
      eventsCount: 0,
      routes: [],
      hasAskExpect: false,
    },
  ],
  edges: [
    { from: "implementar", to: "revisores" },
    { from: "revisores", to: "revisar" },
  ],
};

function node(overrides: Partial<LoopRosterNode> = {}): LoopRosterNode {
  return {
    generation: 1,
    node_id: "implementar",
    item_index: 0,
    state: "succeeded",
    attempt: 1,
    attempts: [
      {
        attempt: 1,
        state: "succeeded",
        disposition: "settled",
        started_at: "2026-08-19T18:40:00Z",
        ended_at: "2026-08-19T18:41:00Z",
      },
    ],
    started_at: "2026-08-19T18:40:00Z",
    ended_at: "2026-08-19T18:41:00Z",
    usage: { tokens: 14_800 },
    ...overrides,
  } as LoopRosterNode;
}

function build(
  nodes: LoopRosterNode[],
  rollups: LoopFanoutRollup[] = [],
  round: number | null = 1,
  isComplete = true
) {
  return buildRosterTable({ nodes, rollups, graph: GRAPH, round, nowMs: NOW, isComplete });
}

describe("buildRosterTable", () => {
  it("Should read a step that started and has not ended as running, not as not started", () => {
    // The single most common state on a live run. Reading it as "not started"
    // inverts the one fact the reader opened the roster for — and it is exactly
    // what "my run looks stalled" turned out to mean.
    const [row] = build([node({ state: "running", ended_at: null })]).rows;

    expect(row.progressState).toBe("in-progress");
    expect(row.durationMs).toBe(NOW - Date.parse("2026-08-19T18:40:00Z"));
  });

  it("Should keep not-started for a step that genuinely never began", () => {
    const [row] = build([node({ state: "pending", started_at: null, ended_at: null })]).rows;

    expect(row.progressState).toBe("not-started");
    expect(row.durationMs).toBeNull();
  });

  // The crash-interrupted contract row (VC-31) is about exactly this: the daemon
  // restarted mid-step, nothing about the timing survived, and the roster used to
  // answer "not started" about a step that had plainly run.
  it("Should say unknown, not not-started, when a step that ran kept no timing", () => {
    for (const state of ["succeeded", "failed", "running", "canceled"] as const) {
      const [row] = build([node({ state, started_at: null, ended_at: null })]).rows;

      expect(row.progressState).toBe("unknown");
      expect(row.durationMs).toBeNull();
    }
  });

  it("Should still read a declined branch as never started", () => {
    // `not_taken` is durable route evidence, not a missing measurement.
    const [row] = build([node({ state: "not_taken", started_at: null, ended_at: null })]).rows;

    expect(row.progressState).toBe("not-started");
  });

  it("Should measure a settled step by its own span, not by the clock", () => {
    const [row] = build([node()]).rows;

    expect(row.progressState).toBe("settled");
    expect(row.durationMs).toBe(60_000);
  });

  it("Should carry the round on every row so two passes of one step differ", () => {
    // Under "All rounds" the step id repeats, and without the round there is
    // nothing on the row telling the two apart.
    const rows = build(
      [node({ generation: 1 }), node({ generation: 2, state: "running", ended_at: null })],
      [],
      null
    ).rows;

    expect(rows.map(row => row.generation)).toEqual([1, 2]);
    expect(new Set(rows.map(row => row.key)).size).toBe(2);
  });

  it("Should price a step's tokens as an estimate rather than a fact", () => {
    const [row] = build([node({ usage: { tokens: 268_000 } })]).rows;

    expect(row.usageTokens).toBe(268_000);
    // The leading `~` is the qualifier travelling with the value.
    expect(row.usageCostLabel).toBe("~$1.34");
  });

  it("Should offer no cost for a step that reported no tokens", () => {
    const [row] = build([node({ usage: null })]).rows;

    expect(row.usageTokens).toBeNull();
    expect(row.usageCostLabel).toBeNull();
  });

  it("Should keep a round's worker out of another round's fan-out", () => {
    // Claim identity has to carry the round. Without it round 2's fan-out claims
    // the identically-named worker in round 1, and that row disappears from
    // "All rounds" — grouped under a container that never spread it.
    const rollup: LoopFanoutRollup = {
      generation: 2,
      node_id: "revisores",
      done: 1,
      failed: 0,
      total: 1,
    };
    const roundOne = node({ generation: 1, node_id: "revisar", item_index: 0 });
    const roundTwo = node({ generation: 2, node_id: "revisar", item_index: 0 });

    const rows = build([roundOne, roundTwo], [rollup], null).rows;

    // Both rounds are present, and only round 2's worker is nested.
    expect(
      rows
        .filter(row => row.nodeId === "revisar")
        .map(row => row.generation)
        .sort()
    ).toEqual([1, 2]);
    expect(rows.find(row => row.generation === 1 && row.nodeId === "revisar")?.isBranch).toBe(
      false
    );
    expect(rows.find(row => row.generation === 2 && row.nodeId === "revisar")?.isBranch).toBe(true);
  });

  it("Should key every row by round, step and item together", () => {
    const rows = build([node({ generation: 1 }), node({ generation: 2 })], [], null).rows;

    expect(rows.map(row => row.key)).toEqual(["1:implementar:0", "2:implementar:0"]);
  });

  it("Should span a fan-out across its workers and stay running until they all settle", () => {
    const rollup: LoopFanoutRollup = {
      generation: 1,
      node_id: "revisores",
      done: 1,
      failed: 0,
      total: 2,
    };
    const branches = [
      node({
        node_id: "revisar",
        item_index: 0,
        started_at: "2026-08-19T18:42:00Z",
        ended_at: "2026-08-19T18:44:00Z",
        usage: { tokens: 10_000 },
      }),
      node({
        node_id: "revisar",
        item_index: 1,
        state: "running",
        started_at: "2026-08-19T18:43:00Z",
        ended_at: null,
        usage: { tokens: 5_000 },
      }),
    ];

    const container = build(branches, [rollup]).rows.find(row => row.fanOutLabel !== null);

    expect(container?.fanOutLabel).toBe("2 workers");
    expect(container?.startedAt).toBe("2026-08-19T18:42:00Z");
    // One worker still running means the fan-out is still running, whatever its
    // finished sibling says.
    expect(container?.progressState).toBe("in-progress");
    expect(container?.usageTokens).toBe(15_000);
  });

  it("Should say a run reached nothing rather than render an empty table", () => {
    expect(build([]).reachedNothing).toBe(true);
  });

  it("Should not claim a run reached nothing while the roster read is unfinished", () => {
    // A rowless partial read and a rowless run look identical here; only the
    // read's own completeness tells them apart, and "No steps ran" is a claim
    // about the run that an incomplete read cannot support.
    expect(build([], [], 1, false).reachedNothing).toBe(false);
    expect(build([], [], 1, false).rows).toHaveLength(0);
  });
});
