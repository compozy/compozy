import {
  addEdge,
  applyEdgeChanges,
  applyNodeChanges,
  type Connection,
  type EdgeChange,
  type NodeChange,
} from "@xyflow/react";

import { editorEdgeId, type EditorEdge, type EditorNode } from "../lib/codec";
import {
  reconcileRouteEdges,
  removeRouteTargets,
  updateRouteTarget,
} from "../lib/loop-editor-route-edges";
import type { useLoopEditorState } from "./use-loop-editor-state";

type LoopEditorState = ReturnType<typeof useLoopEditorState>;

interface UseLoopEditorGraphActionsOptions {
  actions: Pick<
    LoopEditorState,
    "applyGraphEdges" | "applyGraphNodes" | "changeNodeField" | "connectNodes" | "deleteNodes"
  >;
  definitionEditable: boolean;
  edges: EditorEdge[];
  nodes: EditorNode[];
}

export function useLoopEditorGraphActions({
  actions,
  definitionEditable,
  edges,
  nodes,
}: UseLoopEditorGraphActionsOptions) {
  const onNodesChange = (changes: NodeChange<EditorNode>[]) => {
    const allowed = definitionEditable
      ? changes
      : changes.filter(change => change.type !== "remove");
    if (allowed.length === 0) return;
    const removals: string[] = [];
    const rest: NodeChange<EditorNode>[] = [];
    for (const change of allowed) {
      if (change.type === "remove") removals.push(change.id);
      else rest.push(change);
    }
    if (rest.length > 0) {
      const positionsChanged = rest.some(change => change.type === "position");
      actions.applyGraphNodes(applyNodeChanges(rest, nodes), positionsChanged, false);
    }
    if (removals.length > 0) actions.deleteNodes(removals);
  };

  const onEdgesChange = (changes: EdgeChange<EditorEdge>[]) => {
    const allowed = definitionEditable
      ? changes
      : changes.filter(change => change.type !== "remove");
    if (allowed.length === 0) return;
    const removedEdges = allowed.flatMap(change => {
      if (change.type !== "remove") return [];
      const edge = edges.find(candidate => candidate.id === change.id);
      return edge ? [edge] : [];
    });
    if (removedEdges.length === 0) {
      actions.applyGraphEdges(applyEdgeChanges(allowed, edges), false);
      return;
    }
    let nextNodes = nodes;
    let nextEdges = applyEdgeChanges(allowed, edges);
    for (const edge of removedEdges) {
      nextNodes = removeRouteTargets(nextNodes, new Set([edge.target]), edge.source);
      nextEdges = reconcileRouteEdges(nextNodes, nextEdges, edge.source);
    }
    actions.changeNodeField(nextNodes, nextEdges);
  };

  return {
    onConnect: (connection: Connection) => {
      if (!definitionEditable) return;
      const { source, sourceHandle, target } = connection;
      if (!source || !target) return;
      const routeNodes = updateRouteTarget(nodes, source, sourceHandle, target);
      if (routeNodes !== nodes && routeNodes.some((node, index) => node !== nodes[index])) {
        actions.changeNodeField(routeNodes, reconcileRouteEdges(routeNodes, edges, source));
        return;
      }
      const edge: EditorEdge = {
        id: editorEdgeId(source, target, edges.length),
        source,
        target,
        data: { raw: { from: source, to: target } },
      };
      actions.connectNodes(addEdge(edge, edges));
    },
    onEdgesChange,
    onNodesChange,
    onNodesDelete: (deleted: EditorNode[]) => {
      if (!definitionEditable || deleted.length === 0) return;
      actions.deleteNodes(deleted.map(node => node.id));
    },
  };
}
