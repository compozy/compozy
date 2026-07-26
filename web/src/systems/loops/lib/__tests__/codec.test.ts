import { describe, expect, it } from "vitest";

import { loopDetailByName } from "../../mocks/fixtures";
import type { LoopDefinition } from "../../types";
import { definitionToGraph, editorEdgeId, graphToDefinition } from "../codec";
import { setNodeField } from "../loop-editor-draft";

/**
 * A definition carrying file-imported / agent-authored fields the editor never models
 * explicitly (nested params, an unknown top-level `provenance`, extra node keys). The
 * bijective codec (web-unit-1) MUST pass every one of these through verbatim.
 */
function richDefinition(): LoopDefinition {
  return {
    apiVersion: "agh.loop/v1",
    kind: "Loop",
    concurrency: "forbid",
    meta: { name: "rich", version: 4, catalog: { category: "delivery" } },
    contract: {
      goal: "ship",
      definition_of_done: "green",
      iteration_cap: 50,
      budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: "halt" },
      no_progress: { window: 3, hash_fields: ["artifact"] },
    },
    inputs: { slug: { type: "string", required: true } },
    start: [{ kind: "manual" }, { kind: "cli" }],
    graph: {
      nodes: [
        // Opaque graph JSON with a file-only `provenance` key the editor never models.
        {
          id: "load",
          class: "source",
          kind: "file-import",
          pattern: "x/*.md",
          parse: "json",
          provenance: "file",
        },
        {
          id: "execute",
          class: "action",
          kind: "run-agent",
          params: {
            agent: "impl",
            prompt: "do {{ .item.title }}",
            output_schema: { type: "object" },
          },
          session: { isolated: true },
          timeout: "45m",
          retry: { max: 2 },
        },
        {
          id: "review",
          class: "control",
          kind: "gate",
          verdict_policy: "revise_until_clean",
          max_revisions: 3,
        },
      ],
      edges: [
        { from: "load", to: "execute" },
        { from: "execute", to: "review" },
      ],
    } as unknown as LoopDefinition["graph"],
  };
}

describe("loop codec", () => {
  it("Should round-trip definitionToGraph → graphToDefinition losslessly, preserving unknown fields", () => {
    const def = richDefinition();
    const { nodes, edges } = definitionToGraph(def);
    const rebuilt = graphToDefinition(def, nodes, edges);
    expect(rebuilt).toEqual(def);
  });

  it("Should round-trip the real software-delivery definition", () => {
    const def = loopDetailByName.get("software-delivery")!.definition;
    const { nodes, edges } = definitionToGraph(def);
    expect(graphToDefinition(def, nodes, edges)).toEqual(def);
  });

  it("Should carry every raw node/edge field into React Flow node.data.raw for verbatim publish", () => {
    const def = richDefinition();
    const { nodes, edges } = definitionToGraph(def);
    const execute = nodes.find(node => node.id === "execute")!;
    expect(execute.type).toBe("loopNode");
    expect(execute.data.nodeClass).toBe("action");
    expect(execute.data.kind).toBe("run-agent");
    expect((execute.data.raw.params as Record<string, unknown>).prompt).toBe(
      "do {{ .item.title }}"
    );
    expect(edges[0].id).toBe(editorEdgeId("load", "execute", 0));
    expect(edges[0].source).toBe("load");
  });

  it("Should preserve sibling nodes' unknown fields when one node is edited then published", () => {
    const def = richDefinition();
    const { nodes, edges } = definitionToGraph(def);
    const edited = setNodeField(nodes, "review", ["verdict_policy"], "fixed_passes");
    const published = graphToDefinition(def, edited, edges);
    const rawNodes = (published.graph as unknown as { nodes: Record<string, unknown>[] }).nodes;
    // The edited node changed...
    expect(rawNodes.find(node => node.id === "review")?.verdict_policy).toBe("fixed_passes");
    // ...but the file-imported provenance and nested run-agent params are untouched.
    expect(rawNodes.find(node => node.id === "load")?.provenance).toBe("file");
    const executeParams = rawNodes.find(node => node.id === "execute")?.params as
      | Record<string, unknown>
      | undefined;
    expect(executeParams?.agent).toBe("impl");
  });

  it("Should round-trip a watch-events node's events (kind + CEL filter) verbatim", () => {
    const def: LoopDefinition = {
      apiVersion: "agh.loop/v1",
      kind: "Loop",
      meta: { name: "watch", version: 1, catalog: {} },
      contract: {
        goal: "react",
        definition_of_done: "handled",
        iteration_cap: 0,
        budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: "halt" },
        no_progress: { window: 2 },
      },
      graph: {
        nodes: [
          {
            id: "on_events",
            class: "source",
            kind: "watch-events",
            events: [
              { kind: "task.status_changed", filter: "event.payload.to_status == 'completed'" },
              { kind: "loop.terminal" },
            ],
          },
          {
            id: "handle",
            class: "action",
            kind: "run-agent",
            params: { agent: "a", prompt: "go" },
          },
        ],
        edges: [{ from: "on_events", to: "handle" }],
      } as unknown as LoopDefinition["graph"],
    };
    const { nodes, edges } = definitionToGraph(def);
    const watch = nodes.find(node => node.id === "on_events")!;
    expect(watch.data.kind).toBe("watch-events");
    expect((watch.data.raw.events as unknown[])[0]).toEqual({
      kind: "task.status_changed",
      filter: "event.payload.to_status == 'completed'",
    });
    expect(graphToDefinition(def, nodes, edges)).toEqual(def);
  });

  it("Should synthesize a raw edge for a connection drawn with no original JSON", () => {
    const def = richDefinition();
    const { nodes } = definitionToGraph(def);
    const drawn = {
      id: "new",
      source: "review",
      target: "execute",
      data: undefined as never,
    };
    const published = graphToDefinition(def, nodes, [drawn as never]);
    const rawEdges = (published.graph as unknown as { edges: Record<string, unknown>[] }).edges;
    expect(rawEdges).toEqual([{ from: "review", to: "execute" }]);
  });
});
