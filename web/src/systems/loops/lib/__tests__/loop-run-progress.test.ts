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

  // A finished review round: one action step ran, the gate passed, and the whole
  // fix branch was provably declined. Seven rows, of which the reader needs one.
  function reviewRoundWithoutIssues() {
    return buildStepsProgress({
      progress: progress({ round: 2, steps_done: 1, steps_total: 1 }),
      nodes: [
        node("review", "succeeded", { generation: 2 }),
        node("has_issues", "succeeded", { generation: 2 }),
        node("write_artifacts", "not_taken", { generation: 2 }),
        node("fix_batches", "not_taken", { generation: 2 }),
        node("fix_batch", "not_taken", { generation: 2 }),
        node("collect_fixes", "not_taken", { generation: 2 }),
        node("finalize_round", "not_taken", { generation: 2 }),
      ],
      rollups: [],
      rosterIsComplete: true,
      graph: graph([
        ["review", "action"],
        ["has_issues", "control"],
        ["write_artifacts", "action"],
        ["fix_batches", "control"],
        ["fix_batch", "action"],
        ["collect_fixes", "control"],
        ["finalize_round", "action"],
      ]),
    });
  }

  it("Should fold settled control steps and declined branches behind one summary", () => {
    const model = reviewRoundWithoutIssues();

    // Every row is still there, in graph order — the fold hides, it never drops.
    expect(model.steps.map(step => step.nodeId)).toEqual([
      "review",
      "has_issues",
      "write_artifacts",
      "fix_batches",
      "fix_batch",
      "collect_fixes",
      "finalize_round",
    ]);
    expect(model.steps.filter(step => !step.quiet).map(step => step.nodeId)).toEqual(["review"]);
    // The summary keeps the hidden rows' fates in the chips' own words.
    expect(model.fold).toEqual({
      hiddenCount: 6,
      summary: "1 succeeded · 5 not taken",
    });
  });

  it("Should keep parked, failed and pending control steps in the default read", () => {
    const model = buildStepsProgress({
      progress: progress({ round: 1, steps_done: 2, steps_total: 3 }),
      nodes: [
        node("review", "succeeded"),
        node("has_issues", "succeeded"),
        node("approve", "control_pending"),
        node("route", "failed"),
        node("collect_fixes", "pending"),
        node("write_artifacts", "not_taken"),
      ],
      rollups: [],
      rosterIsComplete: true,
      graph: graph([
        ["review", "action"],
        ["has_issues", "control"],
        ["approve", "control"],
        ["route", "control"],
        ["collect_fixes", "control"],
        ["write_artifacts", "action"],
      ]),
    });

    // A gate waiting on a person, a control step that broke, and a control step
    // still ahead of the run are where the run is going; only the clean gate and
    // the declined branch fold.
    expect(model.steps.filter(step => step.quiet).map(step => step.nodeId)).toEqual([
      "has_issues",
      "write_artifacts",
    ]);
    expect(model.fold).toEqual({
      hiddenCount: 2,
      summary: "1 succeeded · 1 not taken",
    });
  });

  it("Should not fold a single quiet row or a round that would fold away entirely", () => {
    const oneQuiet = buildStepsProgress({
      progress: progress({ round: 1, steps_done: 1, steps_total: 2 }),
      nodes: [
        node("review", "succeeded"),
        node("has_issues", "succeeded"),
        node("fix_batch", "running"),
      ],
      rollups: [],
      rosterIsComplete: true,
      graph: graph([
        ["review", "action"],
        ["has_issues", "control"],
        ["fix_batch", "action"],
      ]),
    });
    // Folding one row behind a line of the same height would save nothing.
    expect(oneQuiet.fold).toBeNull();

    const onlyQuiet = buildStepsProgress({
      progress: progress({ round: 1, steps_done: 0, steps_total: 0 }),
      nodes: [node("has_issues", "succeeded"), node("collect_fixes", "succeeded")],
      rollups: [],
      rosterIsComplete: true,
      graph: graph([
        ["has_issues", "control"],
        ["collect_fixes", "control"],
      ]),
    });
    // A list that is all summary and no rows would hide the very thing it was
    // asked to show; nothing folds when nothing would remain.
    expect(onlyQuiet.fold).toBeNull();
    expect(onlyQuiet.steps.every(step => step.quiet)).toBe(true);
  });

  it("Should give a source node no segment, so the bar matches the served count", () => {
    const model = buildStepsProgress({
      progress: progress({ round: 2, steps_done: 2, steps_total: 2 }),
      nodes: [
        node("slug_input", "succeeded", { generation: 2 }),
        node("select_mode", "succeeded", { generation: 2 }),
        node("load_tasks", "succeeded", { generation: 2 }),
        node("orchestrate", "succeeded", { generation: 2 }),
      ],
      rollups: [],
      rosterIsComplete: true,
      graph: graph([
        ["slug_input", "source"],
        ["select_mode", "control"],
        ["load_tasks", "action"],
        ["orchestrate", "action"],
      ]),
    });

    // The daemon counts two steps; the bar draws two, not four. The source and
    // the route fold together, and the summary names neither class.
    expect(model.segments).toEqual(["clean", "clean"]);
    expect(model.ariaLabel).toBe("Step 2 of 2 · round 2: 2 settled");
    expect(model.fold).toEqual({ hiddenCount: 2, summary: "2 succeeded" });
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
