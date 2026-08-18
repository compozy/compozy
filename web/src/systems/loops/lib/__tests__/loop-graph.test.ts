import { describe, expect, it } from "vitest";

import { loopDetailByName } from "../../mocks/fixtures";
import { releaseTrainDetail } from "../../mocks/fixture-release-train";
import type { LoopDefinition } from "../../types";
import {
  fanOutSummary,
  findWatchNode,
  goalNodeIds,
  iterationNamesSummary,
  nodeClassLabel,
  readLoopGraph,
  routeSummary,
  strategySummary,
  topoOrder,
} from "../loop-graph";

const definition = loopDetailByName.get("quality-gate-demo")!.definition;

function graphOf(nodes: Record<string, unknown>[]) {
  return readLoopGraph({ graph: { nodes, edges: [] } } as unknown as Pick<LoopDefinition, "graph">);
}

function nodeOf(raw: Record<string, unknown>) {
  return graphOf([raw]).nodes[0];
}

describe("loop-graph", () => {
  it("Should project the daemon graph (opaque in OpenAPI) into typed nodes and edges", () => {
    const graph = readLoopGraph(definition);
    expect(graph.nodes).toHaveLength(8);
    expect(graph.edges).toHaveLength(7);
    const fanOut = graph.nodes.find(node => node.id === "implement");
    expect(fanOut).toMatchObject({
      nodeClass: "control",
      kind: "fan-out",
      batchSize: 1,
      maxParallel: 1,
      maxFanOut: 64,
    });
    const gate = graph.nodes.find(node => node.id === "review");
    expect(gate?.isGate).toBe(true);
  });

  it("Should drop unreadable nodes and edges rather than surface empty rows", () => {
    const malformed = {
      graph: {
        nodes: [{ id: "ok", class: "action", kind: "run-agent" }, { class: "action" }, 42, null],
        edges: [{ from: "ok", to: "next" }, { from: "" }, "bad"],
      },
    } as unknown as Pick<LoopDefinition, "graph">;
    const graph = readLoopGraph(malformed);
    expect(graph.nodes).toHaveLength(1);
    expect(graph.nodes[0].id).toBe("ok");
    expect(graph.edges).toHaveLength(1);
  });

  it("Should accept a missing graph without throwing", () => {
    expect(readLoopGraph({ graph: undefined } as unknown as Pick<LoopDefinition, "graph">)).toEqual(
      {
        nodes: [],
        edges: [],
      }
    );
  });

  it("Should label node class neutrally, tinting only gate/fan-out sublabels", () => {
    const graph = readLoopGraph(definition);
    const fanOut = graph.nodes.find(node => node.id === "implement")!;
    const gate = graph.nodes.find(node => node.id === "review")!;
    const source = graph.nodes.find(node => node.id === "slug")!;
    expect(nodeClassLabel(fanOut)).toBe("control · fan-out");
    expect(nodeClassLabel(gate)).toBe("control · gate");
    expect(nodeClassLabel(source)).toBe("source");
  });

  it("Should summarize fan-out knobs, marking sequential and unbounded execution", () => {
    const graph = readLoopGraph(definition);
    const fanOut = graph.nodes.find(node => node.id === "implement")!;
    expect(fanOutSummary(fanOut)).toBe("batch 1 · seq · ≤64");
    const source = graph.nodes.find(node => node.id === "slug")!;
    expect(fanOutSummary(source)).toBeNull();
  });

  it("Should resolve the park node from a watch spec, declared events, then a watch-source kind", () => {
    const byWatchSpec = readLoopGraph({
      graph: {
        nodes: [
          { id: "a", class: "action", kind: "run-agent" },
          { id: "w", class: "source", kind: "watch-events", watch: { poll: "30s" } },
        ],
        edges: [],
      },
    } as unknown as Pick<LoopDefinition, "graph">);
    expect(findWatchNode(byWatchSpec)?.id).toBe("w");

    const byEvents = readLoopGraph({
      graph: {
        nodes: [
          { id: "a", class: "action", kind: "run-agent" },
          {
            id: "w",
            class: "source",
            kind: "watch-events",
            events: [{ kind: "review.completed" }],
          },
        ],
        edges: [],
      },
    } as unknown as Pick<LoopDefinition, "graph">);
    expect(findWatchNode(byEvents)?.id).toBe("w");

    const bySourceKind = readLoopGraph({
      graph: {
        nodes: [
          { id: "a", class: "action", kind: "run-agent" },
          { id: "s", class: "source", kind: "watch-source" },
        ],
        edges: [],
      },
    } as unknown as Pick<LoopDefinition, "graph">);
    expect(findWatchNode(bySourceKind)?.id).toBe("s");

    const noWatch = readLoopGraph({
      graph: { nodes: [{ id: "a", class: "action", kind: "run-agent" }], edges: [] },
    } as unknown as Pick<LoopDefinition, "graph">);
    expect(findWatchNode(noWatch)).toBeNull();
  });

  it("Should collect only goal-kind node ids", () => {
    const graph = readLoopGraph({
      graph: {
        nodes: [
          { id: "g1", class: "action", kind: "goal" },
          { id: "act", class: "action", kind: "run-agent" },
          { id: "g2", class: "action", kind: "goal" },
        ],
        edges: [],
      },
    } as unknown as Pick<LoopDefinition, "graph">);
    const ids = goalNodeIds(graph);
    expect([...ids].sort()).toEqual(["g1", "g2"]);
    expect(ids.has("act")).toBe(false);
  });

  it("Should topologically order nodes and append cyclic leftovers in declaration order", () => {
    const graph = readLoopGraph({
      graph: {
        nodes: [
          { id: "a", class: "action", kind: "run-agent" },
          { id: "b", class: "action", kind: "run-agent" },
          { id: "c", class: "action", kind: "run-agent" },
          { id: "x", class: "action", kind: "run-agent" },
          { id: "y", class: "action", kind: "run-agent" },
        ],
        edges: [
          { from: "a", to: "b" },
          { from: "b", to: "c" },
          { from: "x", to: "y" },
          { from: "y", to: "x" },
        ],
      },
    } as unknown as Pick<LoopDefinition, "graph">);
    expect(topoOrder(graph)).toEqual(["a", "b", "c", "x", "y"]);
  });
});

describe("loop-graph completion grammar", () => {
  const releaseTrain = readLoopGraph(releaseTrainDetail.definition);

  it("Should read a strategy from both the shorthand string and the full object", () => {
    expect(
      nodeOf({ id: "f", class: "control", kind: "fan-out", strategy: "fail_fast" }).strategy
    ).toEqual({ kind: "fail_fast" });

    expect(
      nodeOf({
        id: "f",
        class: "control",
        kind: "fan-out",
        strategy: { kind: "best_effort", threshold: "66%", missing: "acceptable" },
      }).strategy
    ).toEqual({ kind: "best_effort", threshold: "66%", missing: "acceptable" });

    expect(
      nodeOf({
        id: "f",
        class: "control",
        kind: "fan-out",
        strategy: { kind: "best_effort", threshold: { count: 3 } },
      }).strategy
    ).toEqual({ kind: "best_effort", threshold: "3", missing: undefined });

    expect(
      nodeOf({ id: "f", class: "control", kind: "fan-out", strategy: "  " }).strategy
    ).toBeUndefined();
    expect(
      nodeOf({ id: "f", class: "control", kind: "fan-out", strategy: { threshold: "50%" } })
        .strategy
    ).toBeUndefined();
  });

  it("Should read the iteration bindings that un-shadow a nested fan-out", () => {
    const node = releaseTrain.nodes.find(entry => entry.id === "rollout")!;
    expect(node.bindAs).toBe("service");
    expect(node.indexAs).toBe("service_index");
    expect(iterationNamesSummary(node)).toBe("as service · index service_index");
    expect(
      iterationNamesSummary(nodeOf({ id: "f", class: "control", kind: "fan-out" }))
    ).toBeNull();
  });

  it("Should read the ordered route table, its default, and the eval-error policy", () => {
    const triage = releaseTrain.nodes.find(entry => entry.id === "triage")!;
    expect(triage.routes).toEqual([
      { when: 'inputs.severity == "p0"', to: "hotfix" },
      { when: 'inputs.severity == "p1"', to: "standard" },
    ]);
    expect(triage.defaultRoute).toBe("backlog");
    expect(triage.onEvalError).toBe("fail");

    expect(
      nodeOf({
        id: "r",
        class: "control",
        kind: "route",
        routes: [{ when: "a", to: "" }, { when: "b", to: "x" }, "nonsense"],
      }).routes
    ).toEqual([{ when: "b", to: "x" }]);
  });

  it("Should read an ask node's prompt and whether it declares an answer shape", () => {
    const ask = releaseTrain.nodes.find(entry => entry.id === "confirm-rollout")!;
    expect(ask.askPrompt).toBe("Which regions ship first?");
    expect(ask.hasAskExpect).toBe(true);

    const bare = nodeOf({ id: "a", class: "control", kind: "ask", params: { prompt: "  " } });
    expect(bare.askPrompt).toBeUndefined();
    expect(bare.hasAskExpect).toBe(false);
  });

  it("Should read the review block on an action node", () => {
    const action = releaseTrain.nodes.find(entry => entry.id === "apply-migration")!;
    expect(action.review).toEqual({
      decisions: ["approve", "edit", "reject", "respond"],
      prompt: "apply-migration proposes a migrate",
      onRejectRoute: "backlog",
      expiresAfter: "24h",
    });

    expect(nodeOf({ id: "x", class: "action", kind: "transform" }).review).toBeUndefined();
  });

  it("Should keep the strategy summary null when the author declared none", () => {
    expect(strategySummary(nodeOf({ id: "f", class: "control", kind: "fan-out" }))).toBeNull();
    expect(strategySummary(releaseTrain.nodes.find(entry => entry.id === "rollout")!)).toBe(
      "best_effort 66% missing acceptable"
    );
    expect(
      strategySummary(nodeOf({ id: "f", class: "control", kind: "fan-out", strategy: "race" }))
    ).toBe("race");
  });

  it("Should say a route has no default rather than inventing one", () => {
    expect(routeSummary(releaseTrain.nodes.find(entry => entry.id === "triage")!)).toBe(
      "2 routes · default backlog"
    );
    expect(
      routeSummary(
        nodeOf({ id: "r", class: "control", kind: "route", routes: [{ when: "a", to: "x" }] })
      )
    ).toBe("1 route · no default");

    expect(routeSummary(nodeOf({ id: "r", class: "control", kind: "route" }))).toBe(
      "0 routes · no default"
    );
    expect(routeSummary(nodeOf({ id: "x", class: "action", kind: "transform" }))).toBeNull();
  });

  it("Should append the strategy and the iteration names to the fan-out summary", () => {
    expect(fanOutSummary(releaseTrain.nodes.find(entry => entry.id === "rollout")!)).toBe(
      "batch 1 · ×2 · ≤500 · best_effort 66% missing acceptable · as service · index service_index"
    );

    expect(
      fanOutSummary(nodeOf({ id: "f", class: "control", kind: "fan-out", strategy: "race" }))
    ).toBe("race");
  });

  it("Should specialize the class label for route and ask control nodes", () => {
    expect(nodeClassLabel(releaseTrain.nodes.find(entry => entry.id === "triage")!)).toBe(
      "control · route"
    );
    expect(nodeClassLabel(releaseTrain.nodes.find(entry => entry.id === "confirm-rollout")!)).toBe(
      "control · ask"
    );
    expect(nodeClassLabel(releaseTrain.nodes.find(entry => entry.id === "collect-rollout")!)).toBe(
      "control"
    );
  });
});
