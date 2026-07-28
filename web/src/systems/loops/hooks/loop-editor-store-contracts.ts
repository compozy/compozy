import { type EnqueueObject, type ExtractEvents } from "@xstate/store";

import type { EditorEdge, EditorNode } from "../lib/codec";
import { emptyLintState, type LoopLintState } from "../lib/loop-editor-lint";
import type { PaletteItem } from "../lib/loop-palette";
import type { LoopDefinition, LoopDetail, ValidateLoopResult } from "../types";

export type LoopEditorView = "graph" | "dsl";
export type LoopEditorSidebarTab = "contract" | "node";

export interface LoopEditorState {
  baseDefinition: LoopDefinition | null;
  edges: EditorEdge[];
  isDirty: boolean;
  lint: LoopLintState;
  nodes: EditorNode[];
  pendingPositionsGeneration: number | null;
  pendingPublishGeneration: number | null;
  pendingValidationGeneration: number | null;
  positionsDirty: boolean;
  positionsGeneration: number;
  publishError: string | null;
  publishGeneration: number;
  scopeGeneration: number;
  selectedNodeId: string | null;
  selectionSeq: number;
  sidebarTab: LoopEditorSidebarTab;
  structuralRevision: number;
  validateFailed: boolean;
  validationGeneration: number;
  view: LoopEditorView;
}

export interface LoopPositionAnnotation {
  node_id: string;
  x: number;
  y: number;
}

export type LoopEditorEvents = {
  connectionCreated: { edges: EditorEdge[] };
  contractFieldChanged: { definition: LoopDefinition | null };
  draftInitialized: { definition: LoopDefinition; edges: EditorEdge[]; nodes: EditorNode[] };
  graphEdgesApplied: { edges: EditorEdge[]; structureChanged: boolean };
  graphNodesApplied: {
    nodes: EditorNode[];
    positionsChanged: boolean;
    structureChanged: boolean;
  };
  layoutApplied: { nodes: EditorNode[] };
  nodeAdded: { item: PaletteItem };
  nodeFieldChanged: { nodes: EditorNode[] };
  nodeRenamed: { edges: EditorEdge[]; nodes: EditorNode[]; selectedNodeId: string };
  nodeSelectionChanged: { id: string | null; reveal: boolean };
  positionsSaveFailed: { generation: number; scopeGeneration: number };
  positionsSaveRequested: {
    execute: (annotations: LoopPositionAnnotation[]) => Promise<unknown>;
  };
  positionsSaveSucceeded: { generation: number; scopeGeneration: number };
  publishFailed: {
    error: string;
    generation: number;
    revision: number;
    scopeGeneration: number;
  };
  publishRejected: {
    generation: number;
    result: ValidateLoopResult;
    revision: number;
    scopeGeneration: number;
  };
  publishRequested: {
    execute: (definition: LoopDefinition, expectedVersion: number | null) => Promise<LoopDetail>;
  };
  publishSucceeded: {
    generation: number;
    loop: LoopDetail;
    revision: number;
    scopeGeneration: number;
  };
  sidebarTabSelected: { tab: LoopEditorSidebarTab };
  validationRequested: {
    execute: (definition: LoopDefinition) => Promise<ValidateLoopResult>;
    notifyFailure: boolean;
  };
  validationSucceeded: {
    generation: number;
    result: ValidateLoopResult;
    scopeGeneration: number;
  };
  validationUnavailable: {
    generation: number;
    notifyFailure: boolean;
    scopeGeneration: number;
  };
  viewSelected: { view: LoopEditorView };
};

export type LoopEditorEmitted = {
  publishCompleted: { loop: LoopDetail };
};

export type LoopEditorEnqueue = EnqueueObject<
  LoopEditorState,
  ExtractEvents<LoopEditorEmitted>,
  LoopEditorEvents
>;

export function createLoopEditorState(scopeGeneration = 0): LoopEditorState {
  return {
    baseDefinition: null,
    edges: [],
    isDirty: false,
    lint: emptyLintState(),
    nodes: [],
    pendingPositionsGeneration: null,
    pendingPublishGeneration: null,
    pendingValidationGeneration: null,
    positionsDirty: false,
    positionsGeneration: 0,
    publishError: null,
    publishGeneration: 0,
    scopeGeneration,
    selectedNodeId: null,
    selectionSeq: 0,
    sidebarTab: "contract",
    structuralRevision: 0,
    validateFailed: false,
    validationGeneration: 0,
    view: "graph",
  };
}
