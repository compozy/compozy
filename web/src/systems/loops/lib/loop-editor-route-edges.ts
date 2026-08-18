import { editorEdgeId, type EditorEdge, type EditorNode, type RawLoopNode } from "./codec";
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

function routeTargets(raw: RawLoopNode): string[] {
  const targets = readRoutes(raw).map(route => route.to);
  const fallback = typeof raw.default === "string" ? raw.default : "";
  if (fallback !== "") targets.push(fallback);
  return [...new Set(targets.filter(Boolean))];
}

export function reconcileRouteEdges(
  nodes: readonly EditorNode[],
  edges: readonly EditorEdge[],
  sourceId: string
): EditorEdge[] {
  const source = nodes.find(node => node.id === sourceId);
  if (source?.data.kind !== "route") return [...edges];
  const nodeIds = new Set(nodes.map(node => node.id));
  const retained = edges.filter(edge => edge.source !== sourceId);
  const appended = routeTargets(source.data.raw).flatMap((target, index) => {
    if (!nodeIds.has(target) || target === sourceId) return [];
    return [
      {
        id: editorEdgeId(sourceId, target, retained.length + index),
        source: sourceId,
        target,
        data: { raw: { from: sourceId, to: target } },
      } satisfies EditorEdge,
    ];
  });
  return [...retained, ...appended];
}

export function updateRouteTarget(
  nodes: readonly EditorNode[],
  sourceId: string,
  sourceHandle: string | null | undefined,
  target: string
): EditorNode[] {
  return nodes.map(node => {
    if (node.id !== sourceId || node.data.kind !== "route") return node;
    const raw = { ...node.data.raw };
    if (sourceHandle === ROUTE_DEFAULT_HANDLE_ID) {
      raw.default = target;
    } else if (sourceHandle?.startsWith("route:")) {
      const index = Number(sourceHandle.slice("route:".length));
      const routes = readRoutes(raw);
      const route = routes[index];
      if (!route) return node;
      routes[index] = { ...route, to: target };
      raw.routes = routes;
    } else {
      return node;
    }
    return { ...node, data: { ...node.data, raw } };
  });
}

export function removeRouteTargets(
  nodes: readonly EditorNode[],
  targets: ReadonlySet<string>,
  sourceId?: string
): EditorNode[] {
  return nodes.map(node => {
    if (node.data.kind !== "route" || (sourceId !== undefined && node.id !== sourceId)) return node;
    const routes = readRoutes(node.data.raw).filter(route => !targets.has(route.to));
    const fallback = typeof node.data.raw.default === "string" ? node.data.raw.default : "";
    const raw: RawLoopNode = { ...node.data.raw, routes };
    if (targets.has(fallback)) delete raw.default;
    return { ...node, data: { ...node.data, raw } };
  });
}

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
