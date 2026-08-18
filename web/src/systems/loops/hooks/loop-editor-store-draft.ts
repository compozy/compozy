import { editorEdgeId, type EditorEdge, type EditorNode, type RawLoopNode } from "../lib/codec";
import type { PaletteItem } from "../lib/loop-palette";
import { uniqueNodeId } from "../lib/loop-palette";

import {
  createLoopEditorState,
  type LoopEditorEvents,
  type LoopEditorState,
} from "./loop-editor-store-contracts";

function nodeKind(raw: RawLoopNode): string {
  return typeof raw.kind === "string" ? raw.kind : "";
}

function nextDropPosition(nodes: readonly EditorNode[]): { x: number; y: number } {
  if (nodes.length === 0) return { x: 40, y: 40 };
  const rightmost = nodes.reduce((max, node) => (node.position.x > max.position.x ? node : max));
  return { x: rightmost.position.x + 200, y: rightmost.position.y };
}

function buildAddedNode(
  current: LoopEditorState,
  item: PaletteItem,
  position?: { x: number; y: number }
): EditorNode {
  const existing = new Set(current.nodes.map(node => node.id));
  const id = uniqueNodeId(item.idBase, existing);
  const raw = item.buildRaw(id);
  return {
    id,
    type: "loopNode",
    position: position ?? nextDropPosition(current.nodes),
    data: { raw, nodeClass: item.nodeClass, kind: nodeKind(raw), hasError: false },
  };
}

function withAddedNode(current: LoopEditorState, added: readonly EditorNode[]) {
  return {
    isDirty: true,
    nodes: [...current.nodes, ...added],
    positionsDirty: true,
    positionsGeneration: current.positionsGeneration + 1,
    structuralRevision: current.structuralRevision + 1,
    validationGeneration: current.validationGeneration + 1,
  };
}

export const loopEditorDraftTransitions = {
  connectionCreated: (current: LoopEditorState, event: LoopEditorEvents["connectionCreated"]) => ({
    ...current,
    edges: event.edges,
    isDirty: true,
    structuralRevision: current.structuralRevision + 1,
    validationGeneration: current.validationGeneration + 1,
  }),
  contractFieldChanged: (
    current: LoopEditorState,
    event: LoopEditorEvents["contractFieldChanged"]
  ) => ({
    ...current,
    baseDefinition: event.definition,
    isDirty: true,
    publishError: null,
    publishFailureKind: null,
    publishRejectedIssues: [],
    publishRejectedDockStale: false,
    structuralRevision: current.structuralRevision + 1,
    validationGeneration: current.validationGeneration + 1,
  }),
  draftInitialized: (current: LoopEditorState, event: LoopEditorEvents["draftInitialized"]) => {
    if (event.sourceKey !== undefined && event.sourceKey === current.initializedSourceKey) return;
    return {
      ...createLoopEditorState(current.scopeGeneration + 1),
      baseDefinition: event.definition,
      edges: event.edges,
      initializedSourceKey: event.sourceKey ?? current.initializedSourceKey,
      nodes: event.nodes,
      selectedNodeId: event.nodes[0]?.id ?? null,
      structuralRevision: current.structuralRevision + 1,
    };
  },
  graphEdgesApplied: (current: LoopEditorState, event: LoopEditorEvents["graphEdgesApplied"]) => ({
    ...current,
    edges: event.edges,
    isDirty: event.structureChanged ? true : current.isDirty,
    structuralRevision: event.structureChanged
      ? current.structuralRevision + 1
      : current.structuralRevision,
    validationGeneration: event.structureChanged
      ? current.validationGeneration + 1
      : current.validationGeneration,
  }),
  graphNodesApplied: (current: LoopEditorState, event: LoopEditorEvents["graphNodesApplied"]) => ({
    ...current,
    nodes: event.nodes,
    isDirty: event.structureChanged ? true : current.isDirty,
    positionsDirty: event.positionsChanged ? true : current.positionsDirty,
    positionsGeneration: event.positionsChanged
      ? current.positionsGeneration + 1
      : current.positionsGeneration,
    structuralRevision: event.structureChanged
      ? current.structuralRevision + 1
      : current.structuralRevision,
    validationGeneration: event.structureChanged
      ? current.validationGeneration + 1
      : current.validationGeneration,
  }),
  layoutApplied: (current: LoopEditorState, event: LoopEditorEvents["layoutApplied"]) => ({
    ...current,
    nodes: event.nodes,
    positionsDirty: true,
    positionsGeneration: current.positionsGeneration + 1,
  }),
  nodeAdded: (current: LoopEditorState, event: LoopEditorEvents["nodeAdded"]) => {
    const node = buildAddedNode(current, event.item, event.position);
    return {
      ...current,
      ...withAddedNode(current, [node]),
      selectedNodeId: node.id,
      selectionSeq: current.selectionSeq + 1,
      sidebarTab: "node" as const,
    };
  },
  nodeAddedWithEdge: (current: LoopEditorState, event: LoopEditorEvents["nodeAddedWithEdge"]) => {
    const node = buildAddedNode(current, event.item, event.position);
    const edge: EditorEdge = {
      id: editorEdgeId(event.source, node.id, current.edges.length),
      source: event.source,
      target: node.id,
      data: { raw: { from: event.source, to: node.id } },
    };
    return {
      ...current,
      ...withAddedNode(current, [node]),

      edges: [...current.edges, edge],
      selectedNodeId: node.id,
      selectionSeq: current.selectionSeq + 1,
      sidebarTab: "node" as const,
    };
  },
  nodesDeleted: (current: LoopEditorState, event: LoopEditorEvents["nodesDeleted"]) => {
    const removed = new Set(event.nodeIds);
    if (removed.size === 0) return;
    const nodes = current.nodes.filter(node => !removed.has(node.id));
    if (nodes.length === current.nodes.length) return;

    const edges = current.edges.filter(
      edge => !removed.has(edge.source) && !removed.has(edge.target)
    );
    const selectedNodeId =
      current.selectedNodeId !== null && removed.has(current.selectedNodeId)
        ? null
        : current.selectedNodeId;
    return {
      ...current,
      edges,
      nodes,
      selectedNodeId,
      selectionSeq: current.selectionSeq + 1,
      isDirty: true,
      publishError: null,
      publishFailureKind: null,
      publishRejectedIssues: [],
      publishRejectedDockStale: false,
      structuralRevision: current.structuralRevision + 1,
      validationGeneration: current.validationGeneration + 1,
    };
  },
  nodesPasted: (current: LoopEditorState, event: LoopEditorEvents["nodesPasted"]) => ({
    ...current,
    edges: event.edges,
    nodes: event.nodes,
    selectedNodeId: event.selectedNodeId,
    selectionSeq: current.selectionSeq + 1,
    sidebarTab: "node" as const,
    isDirty: true,
    positionsDirty: true,
    positionsGeneration: current.positionsGeneration + 1,
    publishError: null,
    publishFailureKind: null,
    publishRejectedIssues: [],
    publishRejectedDockStale: false,
    structuralRevision: current.structuralRevision + 1,
    validationGeneration: current.validationGeneration + 1,
  }),
  nodeFieldChanged: (current: LoopEditorState, event: LoopEditorEvents["nodeFieldChanged"]) => ({
    ...current,
    nodes: event.nodes,
    isDirty: true,
    publishError: null,
    publishFailureKind: null,
    publishRejectedIssues: [],
    publishRejectedDockStale: false,
    structuralRevision: current.structuralRevision + 1,
    validationGeneration: current.validationGeneration + 1,
  }),
  nodeRenamed: (current: LoopEditorState, event: LoopEditorEvents["nodeRenamed"]) => ({
    ...current,
    edges: event.edges,
    nodes: event.nodes,
    selectedNodeId: event.selectedNodeId,
    isDirty: true,
    publishError: null,
    publishFailureKind: null,
    publishRejectedIssues: [],
    publishRejectedDockStale: false,
    structuralRevision: current.structuralRevision + 1,
    validationGeneration: current.validationGeneration + 1,
  }),
  nodeSelectionChanged: (
    current: LoopEditorState,
    event: LoopEditorEvents["nodeSelectionChanged"]
  ) => ({
    ...current,
    selectedNodeId: event.id,
    selectionSeq: current.selectionSeq + 1,
    sidebarTab: event.id === null ? current.sidebarTab : "node",
    view: event.reveal ? "graph" : current.view,
  }),
  sidebarTabSelected: (
    current: LoopEditorState,
    event: LoopEditorEvents["sidebarTabSelected"]
  ) => ({
    ...current,
    sidebarTab: event.tab,
  }),
  viewSelected: (current: LoopEditorState, event: LoopEditorEvents["viewSelected"]) => ({
    ...current,
    view: event.view,
  }),
};
