import dagre from "@dagrejs/dagre";

import type { EditorEdge, EditorNode } from "./codec";
import type { LoopAnnotation } from "../types";

/**
 * Auto-layout for definitions that carry no saved node positions (agent- or
 * file-authored bodies, or a freshly forked read-only Loop). A left-to-right dagre pass
 * lays the DAG out deterministically; any node with a saved annotation (the
 * positions sidecar, ADR-015) overrides the computed coordinate so a hand-arranged
 * layout is never clobbered. Positions live only here + in the sidecar, never in the
 * canonical definition.
 */

export const EDITOR_NODE_WIDTH = 132;
export const EDITOR_NODE_HEIGHT = 64;

/** Indexes a saved-annotations list by node id for O(1) position override lookup. */
export function annotationsToPositions(
  annotations: readonly LoopAnnotation[]
): Map<string, { x: number; y: number }> {
  const map = new Map<string, { x: number; y: number }>();
  for (const annotation of annotations) {
    if (Number.isFinite(annotation.x) && Number.isFinite(annotation.y)) {
      map.set(annotation.node_id, { x: annotation.x, y: annotation.y });
    }
  }
  return map;
}

function dagrePositions(
  nodes: readonly EditorNode[],
  edges: readonly EditorEdge[]
): Map<string, { x: number; y: number }> {
  const graph = new dagre.graphlib.Graph();
  graph.setGraph({ rankdir: "LR", ranksep: 72, nodesep: 28, marginx: 28, marginy: 28 });
  graph.setDefaultEdgeLabel(() => ({}));
  for (const node of nodes) {
    graph.setNode(node.id, { width: EDITOR_NODE_WIDTH, height: EDITOR_NODE_HEIGHT });
  }
  for (const edge of edges) {
    if (graph.hasNode(edge.source) && graph.hasNode(edge.target)) {
      graph.setEdge(edge.source, edge.target);
    }
  }
  dagre.layout(graph);
  const positions = new Map<string, { x: number; y: number }>();
  for (const node of nodes) {
    const laid = graph.node(node.id);
    if (laid) {
      // dagre reports node centers; React Flow positions from the top-left corner.
      positions.set(node.id, {
        x: laid.x - EDITOR_NODE_WIDTH / 2,
        y: laid.y - EDITOR_NODE_HEIGHT / 2,
      });
    }
  }
  return positions;
}

/**
 * Positions every editor node: saved annotations win; otherwise the dagre-computed
 * coordinate is used. Returns a new node array (never mutates the input).
 */
export function layoutEditorGraph(
  nodes: readonly EditorNode[],
  edges: readonly EditorEdge[],
  annotations: readonly LoopAnnotation[]
): EditorNode[] {
  const saved = annotationsToPositions(annotations);
  const computed = dagrePositions(nodes, edges);
  return nodes.map((node, index) => {
    const position = saved.get(node.id) ??
      computed.get(node.id) ?? { x: index * (EDITOR_NODE_WIDTH + 72), y: 0 };
    return { ...node, position };
  });
}
