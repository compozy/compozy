import { type EnqueueObject, type ExtractEvents } from "@xstate/store";

import type { EditorEdge, EditorNode } from "../lib/codec";
import type { NodeFieldEdit } from "../lib/loop-editor-draft";
import { emptyLintState, type LoopLintState } from "../lib/loop-editor-lint";
import type { PaletteItem } from "../lib/loop-palette";
import type { LoopDefinition, LoopDetail, LoopValidationIssue, ValidateLoopResult } from "../types";

export type LoopEditorView = "graph" | "dsl";
export type LoopEditorSidebarTab = "contract" | "node";

export interface LoopEditorState {
  annotationsUnavailable: boolean;
  automaticValidationRevision: number | null;
  baseDefinition: LoopDefinition | null;
  edges: EditorEdge[];
  isDirty: boolean;
  initializedSourceKey: string | null;
  lint: LoopLintState;
  nodes: EditorNode[];
  pendingPositionsGeneration: number | null;
  pendingPublishGeneration: number | null;
  pendingValidationGeneration: number | null;
  positionsDirty: boolean;
  positionsGeneration: number;
  publishError: string | null;
  /** Discriminator for `publishError`: a 422 validation rejection vs a transport/unknown failure. */
  publishFailureKind: "rejected" | "transport" | null;
  /**
   * The issues a publish 422 returned, kept beside the message so the rejected strip can list
   * the daemon's own verdict even while the dock is collapsed. Empty for a transport failure,
   * which has no issue list to show.
   */
  publishRejectedIssues: LoopValidationIssue[];
  /**
   * True when a 422 landed after the draft moved on — the dock is intentionally
   * stale, so the strip must carry the issue list.
   */
  publishRejectedDockStale: boolean;
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
  annotationsStatusObserved: { failed: boolean; notify: () => void };
  automaticValidationCancelled: Record<never, never>;
  automaticValidationElapsed: { revision: number };
  automaticValidationRequested: { enabled: boolean; execute: () => void; revision: number };
  connectionCreated: { edges: EditorEdge[] };
  contractFieldChanged: { definition: LoopDefinition | null };
  draftInitialized: {
    definition: LoopDefinition;
    edges: EditorEdge[];
    nodes: EditorNode[];
    sourceKey?: string;
  };
  graphEdgesApplied: { edges: EditorEdge[]; structureChanged: boolean };
  graphNodesApplied: {
    nodes: EditorNode[];
    positionsChanged: boolean;
    structureChanged: boolean;
  };
  layoutApplied: { nodes: EditorNode[] };
  lifecycleDisposed: Record<never, never>;

  nodeAdded: { item: PaletteItem; position?: { x: number; y: number } };

  nodeAddedWithEdge: {
    item: PaletteItem;
    position: { x: number; y: number };
    source: string;
  };
  nodeFieldChanged: { edges?: EditorEdge[]; nodes: EditorNode[] };
  nodeFieldsEdited: { edits: NodeFieldEdit[]; nodeId: string };

  nodesDeleted: { nodeIds: string[] };

  nodesPasted: { edges: EditorEdge[]; nodes: EditorNode[]; selectedNodeId: string };
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
    annotationsUnavailable: false,
    automaticValidationRevision: null,
    baseDefinition: null,
    edges: [],
    isDirty: false,
    initializedSourceKey: null,
    lint: emptyLintState(),
    nodes: [],
    pendingPositionsGeneration: null,
    pendingPublishGeneration: null,
    pendingValidationGeneration: null,
    positionsDirty: false,
    positionsGeneration: 0,
    publishError: null,
    publishFailureKind: null,
    publishRejectedIssues: [],
    publishRejectedDockStale: false,
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
