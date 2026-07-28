import type { EditorNode, RawLoopNode } from "../lib/codec";
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
    structuralRevision: current.structuralRevision + 1,
    validationGeneration: current.validationGeneration + 1,
  }),
  draftInitialized: (current: LoopEditorState, event: LoopEditorEvents["draftInitialized"]) => ({
    ...createLoopEditorState(current.scopeGeneration + 1),
    baseDefinition: event.definition,
    edges: event.edges,
    nodes: event.nodes,
    selectedNodeId: event.nodes[0]?.id ?? null,
    structuralRevision: current.structuralRevision + 1,
  }),
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
    const existing = new Set(current.nodes.map(node => node.id));
    const id = uniqueNodeId(event.item.idBase, existing);
    const raw = event.item.buildRaw(id);
    const node: EditorNode = {
      id,
      type: "loopNode",
      position: nextDropPosition(current.nodes),
      data: { raw, nodeClass: event.item.nodeClass, kind: nodeKind(raw), hasError: false },
    };
    return {
      ...current,
      isDirty: true,
      nodes: [...current.nodes, node],
      selectedNodeId: id,
      selectionSeq: current.selectionSeq + 1,
      sidebarTab: "node" as const,
      structuralRevision: current.structuralRevision + 1,
      validationGeneration: current.validationGeneration + 1,
    };
  },
  nodeFieldChanged: (current: LoopEditorState, event: LoopEditorEvents["nodeFieldChanged"]) => ({
    ...current,
    nodes: event.nodes,
    isDirty: true,
    publishError: null,
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
