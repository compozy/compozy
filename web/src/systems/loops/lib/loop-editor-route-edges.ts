import type { EditorEdge, EditorNode } from "./codec";
import { readRoutes } from "./loop-node-route-fields";

export const LOOP_EDITOR_EDGE_TYPE = "loopEdge";

export function forwardNodeIds(
  nodes: readonly EditorNode[],
  edges: readonly EditorEdge[],
  nodeId: string
): string[] {
  const downstream = new Map<string, string[]>();
  for (const edge of edges) {
    downstream.set(edge.source, [...(downstream.get(edge.source) ?? []), edge.target]);
  }
  const reachable = new Set<string>();
  const queue = [...(downstream.get(nodeId) ?? [])];
  for (let head = 0; head < queue.length; head += 1) {
    const id = queue[head];
    if (id === undefined || reachable.has(id) || id === nodeId) continue;
    reachable.add(id);
    for (const next of downstream.get(id) ?? []) queue.push(next);
  }

  const connected = new Set<string>();
  for (const edge of edges) {
    connected.add(edge.source);
    connected.add(edge.target);
  }
  for (const node of nodes) {
    if (node.id !== nodeId && !connected.has(node.id)) reachable.add(node.id);
  }
  const forward: string[] = [];
  for (const node of nodes) {
    if (reachable.has(node.id)) forward.push(node.id);
  }
  return forward;
}

export function routeHandleId(index: number): string {
  return `route:${index}`;
}

export const ROUTE_DEFAULT_HANDLE_ID = "route:default";

export interface RouteEdgeDisplay {
  sourceHandle: string;

  routeLabel: string;
}

export function routeCardRows(node: EditorNode): { handle: string; label: string; to: string }[] {
  if (node.data.kind !== "route") return [];
  const routes = readRoutes(node.data.raw);
  const rows = routes.map((route, index) => ({
    handle: routeHandleId(index),
    label: route.when || "(no condition)",
    to: route.to,
  }));
  const fallback = typeof node.data.raw.default === "string" ? node.data.raw.default : "";
  rows.push({ handle: ROUTE_DEFAULT_HANDLE_ID, label: "default", to: fallback });
  return rows;
}

function routeDisplayFor(node: EditorNode, target: string): RouteEdgeDisplay | null {
  const routes = readRoutes(node.data.raw);
  const index = routes.findIndex(route => route.to === target);
  if (index >= 0) {
    return {
      sourceHandle: routeHandleId(index),
      routeLabel: routes[index]?.when || "(no condition)",
    };
  }
  const fallback = typeof node.data.raw.default === "string" ? node.data.raw.default : "";
  if (fallback !== "" && fallback === target) {
    return { sourceHandle: ROUTE_DEFAULT_HANDLE_ID, routeLabel: "default" };
  }
  return null;
}

export interface DisplayEdgeOptions {
  readOnly: boolean;
  onDelete: (edgeId: string) => void;
}

export function buildDisplayEdges(
  edges: readonly EditorEdge[],
  nodes: readonly EditorNode[],
  { readOnly, onDelete }: DisplayEdgeOptions
): EditorEdge[] {
  const routeNodes = new Map<string, EditorNode>();
  for (const node of nodes) {
    if (node.data.kind === "route") routeNodes.set(node.id, node);
  }
  return edges.map(edge => {
    const source = routeNodes.get(edge.source);
    const display = source ? routeDisplayFor(source, edge.target) : null;
    return {
      ...edge,
      type: LOOP_EDITOR_EDGE_TYPE,
      ...(display ? { sourceHandle: display.sourceHandle } : {}),
      data: {
        ...edge.data,
        raw: edge.data?.raw ?? { from: edge.source, to: edge.target },
        ...(display ? { routeLabel: display.routeLabel } : {}),
        readOnly,
        onDelete,
      },
    } as EditorEdge;
  });
}
