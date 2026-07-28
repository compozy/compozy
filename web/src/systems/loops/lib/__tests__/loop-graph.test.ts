import { describe, expect, it } from "vitest";

import { loopDetailByName } from "../../mocks/fixtures";
import type { LoopDefinition } from "../../types";
import {
  fanOutSummary,
  findWatchNode,
  goalNodeIds,
  nodeClassLabel,
  readLoopGraph,
  topoOrder,
} from "../loop-graph";

const definition = loopDetailByName.get("software-delivery")!.definition;

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
