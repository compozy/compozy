import { describe, expect, it } from "vitest";

import { definitionToGraph } from "../codec";
import {
  activeReferenceQuery,
  buildReferenceNamespace,
  filterReferences,
} from "../loop-references";
import type { LoopDefinition } from "../../types";

const definition: Pick<LoopDefinition, "inputs" | "start"> = {
  inputs: { slug: { type: "string", required: true }, implementer: { type: "agent" } },
  start: [{ kind: "manual" }, { kind: "schedule" }],
};

const { nodes } = definitionToGraph({
  graph: {
    nodes: [
      { id: "load_tasks", class: "source", kind: "file-import" },
      { id: "execute", class: "action", kind: "run-agent" },
    ],
    edges: [],
  } as unknown as LoopDefinition["graph"],
});

describe("loop references", () => {
  it("Should enumerate inputs, upstream node outputs, trigger, and generation paths", () => {
    const suggestions = buildReferenceNamespace(definition, { nodes, currentNodeId: "execute" });
    const paths = suggestions.map(suggestion => suggestion.path);
    expect(paths).toContain("inputs.slug");
    expect(paths).toContain("inputs.implementer");
    expect(paths).toContain("nodes.load_tasks.output");
    expect(paths).toContain("nodes.load_tasks.status");
    // The current node never references its own output; trigger appears via schedule start.
    expect(paths).not.toContain("nodes.execute.output");
    expect(paths).toContain("trigger");
    expect(paths).toContain("generation");
  });

  it("Should offer the trigger namespace for webhook/trigger/schedule/network starts only", () => {
    const withWebhook = buildReferenceNamespace(
      { inputs: {}, start: [{ kind: "webhook" }] },
      { nodes }
    ).map(s => s.path);
    expect(withWebhook).toContain("trigger");
    const manualOnly = buildReferenceNamespace(
      { inputs: {}, start: [{ kind: "manual" }, { kind: "cli" }] },
      { nodes }
    ).map(s => s.path);
    expect(manualOnly).not.toContain("trigger");
  });

  it("Should restrict node-output suggestions to upstream ancestors when edges are supplied", () => {
    const graph = definitionToGraph({
      graph: {
        nodes: [
          { id: "a", class: "source", kind: "input" },
          { id: "b", class: "action", kind: "run-agent" },
          { id: "c", class: "control", kind: "gate" },
        ],
        edges: [
          { from: "a", to: "b" },
          { from: "b", to: "c" },
        ],
      } as unknown as LoopDefinition["graph"],
    });
    const paths = buildReferenceNamespace(definition, {
      nodes: graph.nodes,
      edges: graph.edges,
      currentNodeId: "b",
    }).map(s => s.path);
    // b can reference its ancestor a, but not its downstream c (forward ref = unknown_reference).
    expect(paths).toContain("nodes.a.output");
    expect(paths).not.toContain("nodes.c.output");
  });

  it("Should offer item/index only inside a fan-out branch body", () => {
    const outside = buildReferenceNamespace(definition, { nodes }).map(s => s.path);
    expect(outside).not.toContain("item");
    const inside = buildReferenceNamespace(definition, { nodes, inFanoutBranch: true }).map(
      s => s.path
    );
    expect(inside).toContain("item");
    expect(inside).toContain("index");
  });

  it("Should filter the namespace by a partial query", () => {
    const suggestions = buildReferenceNamespace(definition, { nodes });
    expect(filterReferences(suggestions, "inp").every(s => s.path.startsWith("inputs"))).toBe(true);
    expect(filterReferences(suggestions, "")).toHaveLength(suggestions.length);
  });

  it("Should extract the active {{ .partial reference fragment under the caret", () => {
    expect(activeReferenceQuery("do {{ .inp", 10)).toBe("inp");
    expect(activeReferenceQuery("do {{ .inputs.slug }} then", 26)).toBeNull();
    expect(activeReferenceQuery("plain text", 5)).toBeNull();
    expect(activeReferenceQuery("{{ .nodes.load", 14)).toBe("nodes.load");
  });

  it("Should extract a bare namespace identifier in CEL mode (no braces, ADR-020)", () => {
    // A CEL condition has no `{{ }}`; the picker triggers on a namespace-rooted identifier.
    expect(activeReferenceQuery("nodes.load", 10, "cel")).toBe("nodes.load");
    expect(activeReferenceQuery("nod", 3, "cel")).toBe("nod");
    expect(activeReferenceQuery("inputs.slug == 'x'", 11, "cel")).toBe("inputs.slug");
    // A CEL function/operator identifier is not a namespace root → no picker.
    expect(activeReferenceQuery("size(", 5, "cel")).toBeNull();
    expect(activeReferenceQuery("matches", 7, "cel")).toBeNull();
  });
});
