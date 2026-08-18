import { editorEdgeId, type EditorEdge, type EditorNode } from "./codec";
import { readRoutes } from "./loop-node-route-fields";
import { uniqueNodeId } from "./loop-palette";

export interface LoopEditorClipboardContent {
  nodes: EditorNode[];
  edges: EditorEdge[];
}

const PASTE_OFFSET = 32;

export function copyEditorSelection(
  nodes: readonly EditorNode[],
  edges: readonly EditorEdge[],
  nodeIds: readonly string[]
): LoopEditorClipboardContent {
  const selected = new Set(nodeIds);
  const copiedNodes: EditorNode[] = [];
  for (const node of nodes) {
    if (selected.has(node.id)) copiedNodes.push({ ...node });
  }

  const copiedEdges: EditorEdge[] = [];
  for (const edge of edges) {
    if (selected.has(edge.source) && selected.has(edge.target)) copiedEdges.push({ ...edge });
  }
  return { nodes: copiedNodes, edges: copiedEdges };
}

export interface LoopEditorPasteResult {
  nodes: EditorNode[];
  edges: EditorEdge[];

  selectedNodeId: string;
}

export function pasteEditorClipboard(
  nodes: readonly EditorNode[],
  edges: readonly EditorEdge[],
  clipboard: LoopEditorClipboardContent
): LoopEditorPasteResult | null {
  if (clipboard.nodes.length === 0) return null;
  const existing = new Set(nodes.map(node => node.id));
  const idByOriginal = new Map<string, string>();
  for (const node of clipboard.nodes) {
    const id = uniqueNodeId(node.id, existing);
    existing.add(id);
    idByOriginal.set(node.id, id);
  }
  const pastedNodes: EditorNode[] = clipboard.nodes.map(node => {
    const id = idByOriginal.get(node.id);
    if (id === undefined) return node;
    const raw: EditorNode["data"]["raw"] = { ...node.data.raw, id };
    if (node.data.kind === "route") {
      raw.routes = readRoutes(node.data.raw).flatMap(route => {
        const target = idByOriginal.get(route.to);
        return target ? [{ ...route, to: target }] : [];
      });
      const fallback =
        typeof node.data.raw.default === "string"
          ? idByOriginal.get(node.data.raw.default)
          : undefined;
      if (fallback === undefined) delete raw.default;
      else raw.default = fallback;
    }
    return {
      ...node,
      id,
      selected: false,
      position: { x: node.position.x + PASTE_OFFSET, y: node.position.y + PASTE_OFFSET },
      data: { ...node.data, raw },
    };
  });
  const pastedEdges: EditorEdge[] = clipboard.edges.flatMap((edge, index) => {
    const source = idByOriginal.get(edge.source);
    const target = idByOriginal.get(edge.target);
    if (!source || !target) return [];
    return [
      {
        ...edge,
        id: editorEdgeId(source, target, edges.length + index),
        source,
        target,
        data: { raw: { ...edge.data?.raw, from: source, to: target } },
      },
    ];
  });
  const selectedNodeId = pastedNodes[0]?.id;
  if (selectedNodeId === undefined) return null;
  return {
    nodes: [...nodes, ...pastedNodes],
    edges: [...edges, ...pastedEdges],
    selectedNodeId,
  };
}
