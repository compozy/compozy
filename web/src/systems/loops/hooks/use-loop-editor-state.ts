import { useSelector, useStore } from "@xstate/store-react";

import type { EditorEdge, EditorNode } from "../lib/codec";
import type { PaletteItem } from "../lib/loop-palette";
import type { LoopDefinition, LoopDetail, ValidateLoopResult } from "../types";
import {
  loopEditorLogic,
  type LoopEditorSidebarTab,
  type LoopEditorView,
  type LoopPositionAnnotation,
} from "./loop-editor-store";

export type { LoopEditorSidebarTab, LoopEditorView };

export function useLoopEditorState() {
  const store = useStore(loopEditorLogic, undefined);
  const context = useSelector(store, state => state.context);

  return {
    baseDefinition: context.baseDefinition,
    busy:
      context.pendingPositionsGeneration !== null ||
      context.pendingPublishGeneration !== null ||
      context.pendingValidationGeneration !== null,
    edges: context.edges,
    isDirty: context.isDirty,
    lint: context.lint,
    nodes: context.nodes,
    positionsDirty: context.positionsDirty,
    publishError: context.publishError,
    selectedNodeId: context.selectedNodeId,
    selectionSeq: context.selectionSeq,
    sidebarTab: context.sidebarTab,
    store,
    structuralRevision: context.structuralRevision,
    validateFailed: context.validateFailed,
    view: context.view,
    addNode: (item: PaletteItem) => store.trigger.nodeAdded({ item }),
    applyGraphEdges: (edges: EditorEdge[], structureChanged: boolean) =>
      store.trigger.graphEdgesApplied({ edges, structureChanged }),
    applyGraphNodes: (nodes: EditorNode[], positionsChanged: boolean, structureChanged: boolean) =>
      store.trigger.graphNodesApplied({ nodes, positionsChanged, structureChanged }),
    applyLayout: (nodes: EditorNode[]) => store.trigger.layoutApplied({ nodes }),
    changeContract: (definition: LoopDefinition | null) =>
      store.trigger.contractFieldChanged({ definition }),
    changeNodeField: (nodes: EditorNode[]) => store.trigger.nodeFieldChanged({ nodes }),
    connectNodes: (edges: EditorEdge[]) => store.trigger.connectionCreated({ edges }),
    initialize: (definition: LoopDefinition, edges: EditorEdge[], nodes: EditorNode[]) =>
      store.trigger.draftInitialized({ definition, edges, nodes }),
    renameNode: (edges: EditorEdge[], nodes: EditorNode[], selectedNodeId: string) =>
      store.trigger.nodeRenamed({ edges, nodes, selectedNodeId }),
    requestPositionsSave: (execute: (annotations: LoopPositionAnnotation[]) => Promise<unknown>) =>
      store.trigger.positionsSaveRequested({ execute }),
    requestPublish: (
      execute: (definition: LoopDefinition, expectedVersion: number | null) => Promise<LoopDetail>
    ) => store.trigger.publishRequested({ execute }),
    requestValidation: (
      execute: (definition: LoopDefinition) => Promise<ValidateLoopResult>,
      notifyFailure: boolean
    ) => store.trigger.validationRequested({ execute, notifyFailure }),
    selectNode: (id: string | null, reveal = false) =>
      store.trigger.nodeSelectionChanged({ id, reveal }),
    selectSidebarTab: (tab: LoopEditorSidebarTab) => store.trigger.sidebarTabSelected({ tab }),
    selectView: (view: LoopEditorView) => store.trigger.viewSelected({ view }),
  };
}
