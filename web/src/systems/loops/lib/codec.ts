import type { Edge as FlowEdge, Node as FlowNode } from "@xyflow/react";

import type { LoopDefinition, LoopDefinitionGraph } from "../types";
import { toLoopNodeClass, type LoopNodeClass } from "./loop-graph";

/**
 * The bijective `agh.loop/v1` ↔ `@xyflow/react` codec (ADR-015).
 *
 * `definitionToGraph` opens the one canonical definition into a React Flow graph;
 * `graphToDefinition` publishes it back. The codec is lossless: every node/edge
 * carries its ENTIRE original JSON in `data.raw`, and `graphToDefinition` is a
 * structural merge over the original definition — it only replaces `graph.nodes`
 * and `graph.edges`, passing meta / concurrency / inputs / contract / start and any
 * unknown or file-imported node fields through verbatim. Positions are NEVER part
 * of the definition; they live in the annotations sidecar and are applied by the
 * layout module. The GUI never owns invariants — the shared Go linter does.
 */

/** The opaque per-node JSON exactly as it appears in the canonical graph. */
export type RawLoopNode = Record<string, unknown> & { id: string; class: string; kind: string };
/** The opaque per-edge JSON (`{ from, to, ... }`, may carry `sourceHandle`/`condition`). */
export type RawLoopEdge = Record<string, unknown> & { from: string; to: string };

/** Data carried on each React Flow node: the full raw node plus its projected labels. */
export interface EditorNodeData extends Record<string, unknown> {
  raw: RawLoopNode;
  nodeClass: LoopNodeClass | null;
  kind: string;
  /** Attached by the linter when a per-node validation error targets this node. */
  hasError: boolean;
}

/** Data carried on each React Flow edge: the full raw edge for verbatim round-trip. */
export interface EditorEdgeData extends Record<string, unknown> {
  raw: RawLoopEdge;
}

export type EditorNode = FlowNode<EditorNodeData, "loopNode">;
export type EditorEdge = FlowEdge<EditorEdgeData>;

export interface EditorGraph {
  nodes: EditorNode[];
  edges: EditorEdge[];
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null ? (value as Record<string, unknown>) : null;
}

function asString(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function isRawLoopNode(value: unknown): value is RawLoopNode {
  const record = asRecord(value);
  return Boolean(
    record &&
    asString(record.id) !== "" &&
    asString(record.class) !== "" &&
    asString(record.kind) !== ""
  );
}

function isRawLoopEdge(value: unknown): value is RawLoopEdge {
  const record = asRecord(value);
  return Boolean(record && asString(record.from) !== "" && asString(record.to) !== "");
}

function rawGraph(definition: Pick<LoopDefinition, "graph">): {
  nodes: unknown[];
  edges: unknown[];
} {
  const graph = asRecord(definition.graph);
  return {
    nodes: Array.isArray(graph?.nodes) ? graph.nodes : [],
    edges: Array.isArray(graph?.edges) ? graph.edges : [],
  };
}

/** A stable, unique React Flow edge id; the id is UI-only and never round-trips. */
export function editorEdgeId(from: string, to: string, index: number): string {
  return `e_${index}_${from}__${to}`;
}

/**
 * Projects the canonical definition graph into React Flow nodes + edges. Nodes are
 * positioned at the origin; the layout module applies dagre / saved annotations
 * separately so the codec stays a pure structural transform.
 */
export function definitionToGraph(definition: Pick<LoopDefinition, "graph">): EditorGraph {
  const { nodes, edges } = rawGraph(definition);
  const editorNodes: EditorNode[] = [];
  for (const candidate of nodes) {
    if (!isRawLoopNode(candidate)) continue;
    // A node without a non-empty string id (or an edge missing from/to) cannot be placed
    // on the canvas or referenced, and is unreachable for a valid agh.loop/v1 document —
    // ADR-020 enforces snake_case string ids, and the linter rejects anything else. Such
    // an element is intentionally dropped here; the definition it came from is malformed
    // and would fail publish, so the round-trip contract holds for every valid input.
    const raw = candidate;
    editorNodes.push({
      id: raw.id,
      type: "loopNode",
      position: { x: 0, y: 0 },
      data: {
        raw,
        nodeClass: toLoopNodeClass(raw.class),
        kind: raw.kind,
        hasError: false,
      },
    });
  }
  const editorEdges: EditorEdge[] = [];
  edges.forEach((candidate, index) => {
    if (!isRawLoopEdge(candidate)) return;
    const raw = candidate;
    const handle = asString(raw.sourceHandle);
    editorEdges.push({
      id: editorEdgeId(raw.from, raw.to, index),
      source: raw.from,
      target: raw.to,
      ...(handle === "" ? {} : { sourceHandle: handle }),
      data: { raw },
    });
  });
  return { nodes: editorNodes, edges: editorEdges };
}

function definitionNode(raw: RawLoopNode): LoopDefinitionGraph["nodes"][number] {
  const nodeClass = toLoopNodeClass(raw.class);
  if (!nodeClass) {
    throw new Error(`Loop node ${raw.id} has unsupported class ${raw.class}`);
  }
  return { ...raw, class: nodeClass, id: raw.id, kind: raw.kind };
}

/** Rebuilds a raw edge for a connection the user drew that has no original JSON. */
function synthesizeEdge(edge: EditorEdge): RawLoopEdge {
  const handle = typeof edge.sourceHandle === "string" ? edge.sourceHandle : "";
  return { from: edge.source, to: edge.target, ...(handle === "" ? {} : { sourceHandle: handle }) };
}

/**
 * Publishes the React Flow graph back into a canonical definition via structural
 * merge: the original definition is the base, and only `graph.nodes`/`graph.edges`
 * are replaced (in current canvas order) from each element's `data.raw`. Every other
 * field — including unknown/file-imported node keys — passes through untouched, so
 * `graphToDefinition(def, definitionToGraph(def))` round-trips losslessly.
 *
 * Ordering contract: the output preserves the order of the controlled `nodes`/`edges`
 * arrays. React Flow selection/measurement changes must not reorder those arrays (they
 * only touch per-node flags / z-index) — the lossless round-trip depends on it.
 */
export function graphToDefinition(
  base: LoopDefinition,
  nodes: EditorNode[],
  edges: EditorEdge[]
): LoopDefinition {
  const rawNodes = nodes.map(node => definitionNode(node.data.raw));
  const rawEdges = edges.map(edge => edge.data?.raw ?? synthesizeEdge(edge));
  const graph: LoopDefinitionGraph = {
    ...base.graph,
    nodes: rawNodes,
    edges: rawEdges,
  };
  return {
    ...base,
    graph,
  };
}
