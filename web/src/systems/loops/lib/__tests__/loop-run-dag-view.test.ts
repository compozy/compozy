import { describe, expect, it } from "vitest";

import type { LoopFanoutRollup, LoopRosterNode } from "../../types";
import type { LoopGraph, LoopGraphNode } from "../loop-graph";
import { buildRunDag } from "../loop-run-dag-view";
import { LOOP_ROSTER_STATES, loopRosterStateChip } from "../loop-run-state-copy";

function graphNode(
  id: string,
  kind: string,
  nodeClass: "action" | "control" | "source"
): LoopGraphNode {
  return {
    id,
    nodeClass,
    kind,
    isGate: kind === "gate",
    eventsCount: 0,
    routes: [],
    hasAskExpect: false,
  };
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

const CHAIN: LoopGraph = {
  nodes: [
    graphNode("implementar", "run-agent", "action"),
    graphNode("aplicar-correcoes", "gate", "control"),
    graphNode("sintetizador", "collect", "control"),
    graphNode("saida", "run-agent", "action"),
  ],
  edges: [
    { from: "implementar", to: "aplicar-correcoes" },
    { from: "aplicar-correcoes", to: "sintetizador" },
    { from: "sintetizador", to: "saida" },
  ],
};

describe("loopRosterStateChip", () => {
  // UT-046: colour is never the sole carrier. Every state a run can project
  // must arrive with a word, or a colour-blind operator reads nothing.
  it("Should pair a literal word with a tone for every roster state", () => {
    for (const state of LOOP_ROSTER_STATES) {
      const chip = loopRosterStateChip(state);
      expect(chip.label.length).toBeGreaterThan(0);
      // The wire spelling never reaches the DOM.
      expect(chip.label).not.toContain("_");
      expect(chip.tone).toBeTruthy();
      // Everything except the live accent carries a glyph too; `running` carries
      // a pulsing dot instead, which the renderer supplies.
      if (state === "running") expect(chip.pulse).toBe(true);
      else expect(chip.icon).not.toBeNull();
    }
  });

  it("Should keep pending and not-taken distinct without either turning to alarm", () => {
    const pending = loopRosterStateChip("pending");
    const notTaken = loopRosterStateChip("not_taken");

    expect(pending.label).toBe("pending");
    expect(notTaken.label).toBe("not taken");
    // Same calm ramp, different form and glyph — the distinction Safety
    // Invariant 14 requires, carried by shape rather than by colour.
    expect(pending.tone).toBe("neutral");
    expect(notTaken.tone).toBe("neutral");
    expect(pending.form).not.toBe(notTaken.form);
    expect(pending.icon).not.toBe(notTaken.icon);
  });

  it("Should degrade an unrecognised state without printing it", () => {
    const chip = loopRosterStateChip("some_future_state");
    expect(chip.label).toBe("unknown");
    expect(chip.label).not.toContain("some_future_state");
  });
});

describe("buildRunDag", () => {
  it("Should draw every authored node and mark unreached ones pending, not not-taken", () => {
    const model = buildRunDag({
      graph: CHAIN,
      nodes: [node("implementar", "succeeded")],
      rollups: [],
      round: 1,
    });

    expect(model.nodes.map(entry => entry.nodeId)).toEqual([
      "implementar",
      "aplicar-correcoes",
      "sintetizador",
      "saida",
    ]);
    // Absence of a roster row means "not reached yet", never "route declined".
    // Only durable route evidence may say not_taken.
    expect(model.nodes.slice(1).every(entry => entry.chip.state === "pending")).toBe(true);
    expect(model.nodes.map(entry => entry.chip.state)).not.toContain("not_taken");
  });

  it("Should keep the kind glyph neutral while the state chip carries the signal", () => {
    const model = buildRunDag({
      graph: CHAIN,
      nodes: [node("implementar", "failed")],
      rollups: [],
      round: 1,
    });

    const failed = model.nodes[0];
    expect(failed.chip.tone).toBe("danger");
    expect(failed.chip.label).toBe("failed");
    // A failed agent is still an agent: structure and status never share a channel.
    expect(failed.kindLabel).toBe("agent");
    expect(failed.kindIcon).not.toBeNull();
  });

  it("Should light the edge into a running node and mark a flowed edge taken", () => {
    const model = buildRunDag({
      graph: CHAIN,
      nodes: [
        node("implementar", "succeeded"),
        node("aplicar-correcoes", "succeeded"),
        node("sintetizador", "running"),
      ],
      rollups: [],
      round: 1,
    });

    const [first, second, third] = model.edges;
    expect(first.state).toBe("taken");
    // Liveness sits on the edge *into* the working node, pulling the eye to the
    // front of the run rather than to something that has already stopped.
    expect(second.state).toBe("live");
    expect(third.state).toBe("idle");
  });

  it("Should draw a declined branch as not-taken rather than idle", () => {
    const routed: LoopGraph = {
      nodes: [
        graphNode("rota", "route", "control"),
        graphNode("rota-manual", "run-agent", "action"),
      ],
      edges: [{ from: "rota", to: "rota-manual" }],
    };
    const model = buildRunDag({
      graph: routed,
      nodes: [node("rota", "succeeded"), node("rota-manual", "not_taken")],
      rollups: [],
      round: 1,
    });

    expect(model.edges[0].state).toBe("not_taken");
    const declined = model.nodes.find(entry => entry.nodeId === "rota-manual");
    expect(declined?.chip.label).toBe("not taken");
    expect(declined?.note).toBe("The run took another route.");
  });

  it("Should centre on the step waiting on a human before anything else", () => {
    const model = buildRunDag({
      graph: CHAIN,
      nodes: [
        node("implementar", "succeeded"),
        node("aplicar-correcoes", "control_pending"),
        node("sintetizador", "running"),
      ],
      rollups: [],
      round: 1,
    });

    // A running node is interesting; a node waiting on a person is urgent.
    expect(model.focusNodeId).toBe("aplicar-correcoes");
    expect(model.focusReason).toContain("waiting on you");
    expect(model.nodes.find(entry => entry.isFocus)?.nodeId).toBe("aplicar-correcoes");
  });

  it("Should say plainly when nothing needs a human", () => {
    const model = buildRunDag({
      graph: CHAIN,
      nodes: [node("implementar", "succeeded")],
      rollups: [],
      round: 1,
    });
    expect(model.focusNodeId).toBeNull();
    expect(model.focusReason).toBe("Nothing needs you now.");
  });

  it("Should keep a wide fan-out one entity with a rollup, never ten nodes", () => {
    const fanGraph: LoopGraph = {
      nodes: [
        graphNode("revisores", "fan-out", "control"),
        ...Array.from({ length: 10 }, (_unused, index) =>
          graphNode(`revisor-${index}`, "run-agent", "action")
        ),
        graphNode("sintetizador", "collect", "control"),
      ],
      edges: [
        ...Array.from({ length: 10 }, (_unused, index) => ({
          from: "revisores",
          to: `revisor-${index}`,
        })),
        ...Array.from({ length: 10 }, (_unused, index) => ({
          from: `revisor-${index}`,
          to: "sintetizador",
        })),
      ],
    };
    const rollup: LoopFanoutRollup = {
      generation: 1,
      node_id: "revisores",
      done: 7,
      total: 10,
      failed: 1,
    };
    const workers = Array.from({ length: 10 }, (_unused, index) =>
      node(`revisor-${index}`, index < 7 ? "succeeded" : index === 7 ? "failed" : "running")
    );

    const model = buildRunDag({
      graph: fanGraph,
      nodes: workers,
      rollups: [rollup],
      round: 1,
    });

    // Ten workers, one entity in the lane.
    expect(model.nodes.map(entry => entry.nodeId)).toEqual(["revisores", "sintetizador"]);
    const fan = model.nodes[0];
    // Too wide to name, so the renderer draws lanes and a sentence — but the
    // band still knows exactly which rows belong to it, which is what keeps
    // them out of the lane in the first place.
    expect(fan.fanOut?.wide).toBe(true);
    expect(fan.fanOut?.branches).toHaveLength(10);
    expect(fan.fanOut?.countLabel).toBe("partial 7 of 10");
    expect(fan.fanOut?.summary).toBe("7 done · 1 failed · 2 still running");
    // Width and fate stay drawn: one lane per worker, not a bare fraction.
    expect(fan.fanOut?.segments).toHaveLength(10);
    // The failed worker still colours the fan — a fan never hides a failure.
    expect(fan.chip.tone).toBe("danger");
  });

  it("Should fall back to the roster when the definition cannot be read", () => {
    const model = buildRunDag({ graph: null, nodes: [], rollups: [], round: 1 });
    expect(model.empty).toBe(true);
    expect(model.nodes).toEqual([]);
  });
});

// A lane drawn in topological order is not a chain. Connecting whatever happens
// to sit next to each other draws relationships nobody authored — the failure
// this suite exists to catch.
describe("buildRunDag topology", () => {
  const DIAMOND: LoopGraph = {
    nodes: [
      graphNode("implementar", "run-agent", "action"),
      graphNode("revisor-a", "run-agent", "action"),
      graphNode("revisor-b", "run-agent", "action"),
      graphNode("sintetizador", "collect", "control"),
    ],
    edges: [
      { from: "implementar", to: "revisor-a" },
      { from: "implementar", to: "revisor-b" },
      { from: "revisor-a", to: "sintetizador" },
      { from: "revisor-b", to: "sintetizador" },
    ],
  };

  it("Should draw only authored edges and never join two parallel siblings", () => {
    const model = buildRunDag({
      graph: DIAMOND,
      nodes: [
        node("implementar", "succeeded"),
        node("revisor-a", "succeeded"),
        node("revisor-b", "running"),
      ],
      rollups: [],
      round: 1,
    });

    const drawn = model.edges.map(edge => `${edge.from}->${edge.to}`).sort();
    expect(drawn).toEqual([
      "implementar->revisor-a",
      "implementar->revisor-b",
      "revisor-a->sintetizador",
      "revisor-b->sintetizador",
    ]);
    // The two reviewers run in parallel. Neither feeds the other.
    expect(drawn).not.toContain("revisor-a->revisor-b");
    expect(drawn).not.toContain("revisor-b->revisor-a");
  });

  it("Should place parallel siblings in one column and layer the rest", () => {
    const model = buildRunDag({
      graph: DIAMOND,
      nodes: [node("implementar", "succeeded")],
      rollups: [],
      round: 1,
    });

    expect(model.columns.map(column => column.nodes.map(entry => entry.nodeId))).toEqual([
      ["implementar"],
      ["revisor-a", "revisor-b"],
      ["sintetizador"],
    ]);
    // Two edges cross the first gutter, so the fan reads as a fan.
    expect(model.columns[0].gutter).toHaveLength(2);
  });

  it("Should compress an authored path through a collapsed fan-out, not invent one", () => {
    const fanGraph: LoopGraph = {
      nodes: [
        graphNode("implementar", "run-agent", "action"),
        graphNode("revisores", "fan-out", "control"),
        graphNode("revisar", "run-agent", "action"),
        graphNode("sintetizador", "collect", "control"),
      ],
      edges: [
        { from: "implementar", to: "revisores" },
        { from: "revisores", to: "revisar" },
        { from: "revisar", to: "sintetizador" },
      ],
    };
    const model = buildRunDag({
      graph: fanGraph,
      nodes: [node("implementar", "succeeded"), node("revisar", "succeeded")],
      rollups: [{ generation: 1, node_id: "revisores", done: 1, total: 1, failed: 0 }],
      round: 1,
    });

    const drawn = model.edges.map(edge => `${edge.from}->${edge.to}`);
    // `revisar` folded into the band, so the path it carried survives as
    // revisores->sintetizador. The band's edge to its own worker is now
    // internal and disappears rather than becoming a self-loop.
    expect(drawn).toContain("implementar->revisores");
    expect(drawn).toContain("revisores->sintetizador");
    expect(drawn).not.toContain("revisores->revisar");
    expect(drawn).not.toContain("revisores->revisores");
  });
});
