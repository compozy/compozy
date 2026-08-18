import dagre from "@dagrejs/dagre";

import type { EditorEdge, EditorNode } from "./codec";
import type { LoopAnnotation } from "../types";

export const EDITOR_NODE_WIDTH = 188;
export const EDITOR_NODE_HEIGHT = 96;

export const EDITOR_ROUTE_ROW_HEIGHT = 22;

export function editorNodeHeight(node: EditorNode): number {
  if (node.data.kind !== "route") return EDITOR_NODE_HEIGHT;
  const routes = Array.isArray(node.data.raw.routes) ? node.data.raw.routes.length : 0;
  return EDITOR_NODE_HEIGHT + (routes + 1) * EDITOR_ROUTE_ROW_HEIGHT;
}

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
    graph.setNode(node.id, { width: EDITOR_NODE_WIDTH, height: editorNodeHeight(node) });
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
        y: laid.y - editorNodeHeight(node) / 2,
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
