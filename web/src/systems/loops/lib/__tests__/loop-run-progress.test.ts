import { describe, expect, it } from "vitest";

import type { LoopFanoutRollup, LoopRosterNode, LoopStepProgress } from "../../types";
import type { LoopGraph, LoopGraphNode } from "../loop-graph";
import { buildStepsProgress } from "../loop-run-progress";

type TestGraphNodeClass = "action" | "control" | "source";

function graphNode(id: string, nodeClass: TestGraphNodeClass): LoopGraphNode {
  return {
    id,
    nodeClass,
    kind: nodeClass === "control" ? "gate" : "run-agent",
    isGate: nodeClass === "control",
    eventsCount: 0,
    routes: [],
    hasAskExpect: false,
  };
}

function graph(ids: [string, TestGraphNodeClass][]): LoopGraph {
  const nodes = ids.map(([id, nodeClass]) => graphNode(id, nodeClass));
  const edges = ids.slice(1).map(([id], index) => ({ from: ids[index][0], to: id }));
  return { nodes, edges };
}

function node(
  nodeId: string,
  state: string,
  overrides: Partial<LoopRosterNode> = {}
): LoopRosterNode {
  return {
    generation: 1,
    node_id: nodeId,
    item_index: 0,
    state,
    attempt: 1,
    attempts: [],
    ...overrides,
  } as LoopRosterNode;
}

function progress(overrides: Partial<LoopStepProgress> = {}): LoopStepProgress {
  return { round: 1, steps_done: 0, steps_total: 0, ...overrides };
}

// The counts are the daemon's. These cases pin what the page does *around* them:
// which steps exist, which of them count, and what the label says when the
// numbers alone would mislead.
describe("buildStepsProgress", () => {
  // UT-035
  it("Should render the served step count and let control segments contribute no step", () => {
    const model = buildStepsProgress({
      progress: progress({ round: 1, steps_done: 3, steps_total: 6 }),
      nodes: [
        node("implementar", "succeeded"),
        node("revisor-a", "succeeded"),
        node("revisor-b", "succeeded"),
        node("aplicar-correcoes", "control_pending"),
        node("sintetizador", "pending"),
        node("saida", "pending"),
      ],
      rollups: [],
      rosterIsComplete: true,
      graph: graph([
        ["implementar", "action"],
        ["revisor-a", "action"],
        ["revisor-b", "action"],
        ["aplicar-correcoes", "control"],
        ["sintetizador", "action"],
        ["saida", "action"],
      ]),
    });

    // The label is the served verdict verbatim — the page never recounts it.
    expect(model.label).toBe("Step 3 of 6");
    expect(model.stepsDone).toBe(3);
    expect(model.stepsTotal).toBe(6);

    // The gate still renders as a step row (it has state worth reading) but it
    // contributes no segment, and the action steps either side of it stay
    // adjacent — the numbering closes over the control node with no gap.
    expect(model.steps.map(step => step.nodeId)).toContain("aplicar-correcoes");
    expect(model.segments).toEqual(["clean", "clean", "clean", "pending", "pending"]);
    expect(model.rightMeta).toBe("3 to go");
  });

  // UT-036
  it("Should omit the round counter on a single-pass round and show it afterwards", () => {
    const nodes = [node("implementar", "succeeded"), node("saida", "running")];
    const definition = graph([
      ["implementar", "action"],
      ["saida", "action"],
    ]);

    const firstRound = buildStepsProgress({
      progress: progress({ round: 1, steps_done: 1, steps_total: 2 }),
      nodes,
      rollups: [],
      rosterIsComplete: true,
      graph: definition,
    });
    expect(firstRound.showRound).toBe(false);
    expect(firstRound.label).toBe("Step 1 of 2");
    expect(firstRound.label).not.toContain("round");

    const secondRound = buildStepsProgress({
      progress: progress({ round: 2, steps_done: 1, steps_total: 2 }),
      nodes: nodes.map(entry => ({ ...entry, generation: 2 })),
      rollups: [],
      rosterIsComplete: true,
      graph: definition,
    });
    expect(secondRound.showRound).toBe(true);
    expect(secondRound.label).toBe("Step 1 of 2 · round 2");
  });

  // UT-037
  it("Should leave a not-taken branch out of the segments and keep it neutral", () => {
    const model = buildStepsProgress({
      progress: progress({ round: 1, steps_done: 1, steps_total: 2 }),
      nodes: [
        node("implementar", "succeeded"),
        node("rota-manual", "not_taken"),
        node("saida", "pending"),
      ],
      rollups: [],
      rosterIsComplete: true,
      graph: graph([
        ["implementar", "action"],
        ["rota-manual", "action"],
        ["saida", "action"],
      ]),
    });

    // The road not taken leaves the denominator: two segments, not three.
    expect(model.segments).toEqual(["clean", "pending"]);

    const notTaken = model.steps.find(step => step.nodeId === "rota-manual");
    // It still appears — absence is evidence — but it reads as calm, never as
    // something the run owes or something that went wrong.
    expect(notTaken?.chip.label).toBe("not taken");
    expect(notTaken?.chip.tone).toBe("neutral");
    expect(notTaken?.chip.form).toBe("absent");
    expect(notTaken?.chip.tone).not.toBe("danger");
  });

  // UT-038
  it("Should state the dominant park reason when every action step is parked", () => {
    const model = buildStepsProgress({
      progress: progress({ round: 2, steps_done: 2, steps_total: 4 }),
      nodes: [
        node("revisor-a", "waiting", { generation: 2 }),
        node("revisor-b", "waiting", { generation: 2 }),
        node("revisor-c", "retrying", { generation: 2 }),
        node("aplicar-correcoes", "control_pending", { generation: 2 }),
      ],
      rollups: [],
      rosterIsComplete: true,
      graph: graph([
        ["revisor-a", "action"],
        ["revisor-b", "action"],
        ["revisor-c", "action"],
        ["aplicar-correcoes", "control"],
      ]),
    });

    expect(model.parkedReason).toBe("waiting on something");
    expect(model.leftMeta).toBe("Nothing is moving — waiting on something");
    // No percentage: a frozen bar reads as progress that has stalled rather than
    // as work that is suspended, which is the misreading US-006.EC-3 is about.
    expect(model.leftMeta).not.toMatch(/%/);
    // Compared whole: `every` is vacuously true on an empty array, so it would
    // still pass if segment construction disappeared.
    expect(model.segments).toEqual(["parked", "parked", "parked"]);
  });

  it("Should draw a fan-out as one step whose branches are its lanes", () => {
    const rollup: LoopFanoutRollup = {
      generation: 1,
      node_id: "revisores",
      done: 2,
      total: 3,
      failed: 0,
    };
    const model = buildStepsProgress({
      progress: progress({ round: 1, steps_done: 3, steps_total: 4 }),
      nodes: [
        node("implementar", "succeeded"),
        node("revisor-seguranca", "succeeded"),
        node("revisor-estilo", "succeeded", { attempt: 2 }),
        node("revisor-perf", "waiting"),
      ],
      rollups: [rollup],
      rosterIsComplete: true,
      graph: {
        nodes: [
          graphNode("implementar", "action"),
          graphNode("revisores", "control"),
          graphNode("revisor-seguranca", "action"),
          graphNode("revisor-estilo", "action"),
          graphNode("revisor-perf", "action"),
        ],
        edges: [
          { from: "implementar", to: "revisores" },
          { from: "revisores", to: "revisor-seguranca" },
          { from: "revisores", to: "revisor-estilo" },
          { from: "revisores", to: "revisor-perf" },
        ],
      },
    });

    // Three workers, one step row. They never become sibling steps.
    const fanOut = model.steps.find(step => step.nodeId === "revisores");
    expect(fanOut?.fanOut?.countLabel).toBe("2/3");
    expect(fanOut?.fanOut?.branches.map(branch => branch.label)).toEqual([
      "revisor-estilo",
      "revisor-perf",
      "revisor-seguranca",
    ]);
    // An attempt is metadata on the branch, not a fourth worker.
    expect(fanOut?.fanOut?.branches.find(b => b.label === "revisor-estilo")?.attemptLabel).toBe(
      "attempt 2"
    );
    expect(model.steps.map(step => step.nodeId)).not.toContain("revisor-perf");
    // The bar still counts each worker: one lane per branch, plus implementar.
    expect(model.segments).toHaveLength(4);
  });

  it("Should say so plainly when a round has no steps rather than count to zero", () => {
    const model = buildStepsProgress({
      progress: progress({ round: 1, steps_done: 0, steps_total: 0 }),
      nodes: [],
      rollups: [],
      rosterIsComplete: true,
      graph: null,
    });
    expect(model.label).toBe("No steps in this round yet");
    expect(model.segments).toEqual([]);
    expect(model.rightMeta).toBe("");
  });
});
