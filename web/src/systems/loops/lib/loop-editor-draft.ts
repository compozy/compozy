import type { EditorEdge, EditorNode, RawLoopEdge } from "./codec";
import type { FieldPath } from "./loop-node-schema";

/** Reads the value at a raw-JSON path (dotted/indexed), or `undefined` when absent. */
export function getAtPath(source: unknown, path: FieldPath): unknown {
  let current: unknown = source;
  for (const segment of path) {
    if (Array.isArray(current) && typeof segment === "number") {
      current = current[segment];
      continue;
    }
    if (!isRecord(current) || typeof segment !== "string") return undefined;
    current = current[segment];
  }
  return current;
}

type MutableContainer = Record<string, unknown> | unknown[];

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function cloneContainer(value: unknown, key: string | number): MutableContainer {
  if (Array.isArray(value)) return [...value];
  if (isRecord(value)) return { ...value };
  // Create the missing container: an array when the next key is a numeric index.
  return typeof key === "number" ? [] : {};
}

function readContainer(container: MutableContainer, key: string | number): unknown {
  if (Array.isArray(container)) return typeof key === "number" ? container[key] : undefined;
  return typeof key === "string" ? container[key] : undefined;
}

function readContainerValue(value: unknown, key: string | number): unknown {
  if (Array.isArray(value)) return typeof key === "number" ? value[key] : undefined;
  if (isRecord(value)) return typeof key === "string" ? value[key] : undefined;
  return undefined;
}

function writeContainer(container: MutableContainer, key: string | number, value: unknown): void {
  if (Array.isArray(container)) {
    if (typeof key === "number") container[key] = value;
    return;
  }
  if (typeof key === "string") container[key] = value;
}

function deleteContainerValue(container: MutableContainer, key: string | number): void {
  if (Array.isArray(container)) {
    if (typeof key === "number") container.splice(key, 1);
    return;
  }
  if (typeof key === "string") delete container[key];
}

/**
 * Returns a structural copy of `source` with `value` set at `path`. A `value` of
 * `undefined` deletes the leaf key rather than writing `undefined` (so clearing an
 * optional override removes it from the definition instead of pinning a null field).
 */
export function setAtPath<T extends object>(source: T, path: FieldPath, value: unknown): T {
  if (path.length === 0) return source;
  const [head, ...rest] = path;
  const existing = readContainerValue(source, head);
  // Clearing a value under a missing parent is a no-op — don't synthesize an empty
  // container (e.g. clearing `retry.max` on a node with no `retry` must not add `retry: {}`).
  if (value === undefined && rest.length > 0 && (existing === undefined || existing === null)) {
    return source;
  }
  const clone = cloneContainer(source, head);
  if (rest.length === 0) {
    if (value === undefined) {
      deleteContainerValue(clone, head);
    } else {
      writeContainer(clone, head, value);
    }
    return clone as T;
  }
  writeContainer(
    clone,
    head,
    setAtPath(cloneContainer(readContainer(clone, head), rest[0]), rest, value)
  );
  return clone as T;
}

function isEmptyContainer(value: unknown): boolean {
  if (Array.isArray(value)) return value.length === 0;
  if (isRecord(value)) return Object.keys(value).length === 0;
  return false;
}

/** Removes a path and prunes ancestors emptied by that removal; siblings keep parents alive. */
export function unsetAtPath<T extends object>(source: T, path: FieldPath): T {
  if (path.length === 0) return source;
  const [head, ...rest] = path;
  const existing = readContainerValue(source, head);
  if (existing === undefined || existing === null) return source;
  if (rest.length === 0) {
    const clone = cloneContainer(source, head);
    deleteContainerValue(clone, head);
    return clone as T;
  }
  if (typeof existing !== "object") return source;
  const child = unsetAtPath(existing as object, rest);
  if (child === existing) return source;
  const clone = cloneContainer(source, head);
  if (isEmptyContainer(child)) {
    deleteContainerValue(clone, head);
  } else {
    writeContainer(clone, head, child);
  }
  return clone as T;
}

/** One path write inside a draft transition; `undefined` clears (and prunes) the path. */
export interface NodeFieldEdit {
  path: FieldPath;
  value: unknown;
}

/** Applies several path writes to one node in a single draft transition. */
export function setNodeFields(
  nodes: readonly EditorNode[],
  nodeId: string,
  edits: readonly NodeFieldEdit[]
): EditorNode[] {
  if (edits.length === 0) return [...nodes];
  return nodes.map(node => {
    if (node.id !== nodeId) return node;
    const raw = edits.reduce(
      (current, edit) =>
        edit.value === undefined
          ? unsetAtPath(current, edit.path)
          : setAtPath(current, edit.path, edit.value),
      node.data.raw
    );
    if (raw === node.data.raw) return node;
    return { ...node, data: { ...node.data, raw } };
  });
}

/** Applies one field edit to a node's raw JSON, returning a new node array. */
export function setNodeField(
  nodes: readonly EditorNode[],
  nodeId: string,
  path: FieldPath,
  value: unknown
): EditorNode[] {
  return setNodeFields(nodes, nodeId, [{ path, value }]);
}

/** Whether a field path targets the node id (rename must cascade, not a plain set). */
export function isNodeIdPath(path: FieldPath): boolean {
  return path.length === 1 && path[0] === "id";
}

function renameEdgeRaw(raw: RawLoopEdge, oldId: string, newId: string): RawLoopEdge {
  const next: RawLoopEdge = { ...raw };
  if (next.from === oldId) next.from = newId;
  if (next.to === oldId) next.to = newId;
  return next;
}

function renameRouteRaw(raw: EditorNode["data"]["raw"], oldId: string, newId: string) {
  if (raw.kind !== "route") return raw;
  const routes = Array.isArray(raw.routes)
    ? raw.routes.map(entry => {
        if (typeof entry !== "object" || entry === null) return entry;
        const route = entry as Record<string, unknown>;
        return route.to === oldId ? { ...route, to: newId } : entry;
      })
    : raw.routes;
  return {
    ...raw,
    routes,
    default: raw.default === oldId ? newId : raw.default,
  };
}

/**
 * Renames a node id everywhere it is referenced: the raw node, the React Flow node id,
 * and every edge's `source`/`target` + raw `from`/`to`. Reference strings inside other
 * nodes' templates (`nodes.<id>...`) are intentionally NOT rewritten — the linter
 * surfaces the now-`unknown_reference` so the author fixes it deliberately.
 */
export function renameNodeId(
  nodes: readonly EditorNode[],
  edges: readonly EditorEdge[],
  oldId: string,
  newId: string
): { nodes: EditorNode[]; edges: EditorEdge[] } {
  const nextNodes = nodes.map(node => {
    const routeRaw = renameRouteRaw(node.data.raw, oldId, newId);
    if (node.id !== oldId) {
      return routeRaw === node.data.raw ? node : { ...node, data: { ...node.data, raw: routeRaw } };
    }
    const raw = { ...routeRaw, id: newId };
    return { ...node, id: newId, data: { ...node.data, raw } };
  });
  const nextEdges = edges.map(edge => {
    const source = edge.source === oldId ? newId : edge.source;
    const target = edge.target === oldId ? newId : edge.target;
    if (source === edge.source && target === edge.target) return edge;
    return {
      ...edge,
      source,
      target,
      data: edge.data
        ? { ...edge.data, raw: renameEdgeRaw(edge.data.raw, oldId, newId) }
        : edge.data,
    };
  });
  return { nodes: nextNodes, edges: nextEdges };
}
