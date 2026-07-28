import { createStoreLogic } from "@xstate/store";

import { notifyUser } from "@/lib/user-feedback";

import { LoopValidationError } from "../adapters/loops-api";
import { editorDefinitionFromLoop } from "../lib/loop-editor-definition";
import {
  graphToDefinition,
  type EditorEdge,
  type EditorNode,
  type RawLoopNode,
} from "../lib/codec";
import {
  applyLintToNodes,
  buildLintState,
  emptyLintState,
  type LoopLintState,
} from "../lib/loop-editor-lint";
import { uniqueNodeId, type PaletteItem } from "../lib/loop-palette";
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

type LoopEditorEvents = {
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

type LoopEditorEmitted = {
  operationFailed: { message: string };
  publishCompleted: { loop: LoopDetail };
};

function createLoopEditorState(scopeGeneration = 0): LoopEditorState {
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

function nodeKind(raw: RawLoopNode): string {
  return typeof raw.kind === "string" ? raw.kind : "";
}

function nextDropPosition(nodes: readonly EditorNode[]): { x: number; y: number } {
  if (nodes.length === 0) return { x: 40, y: 40 };
  const rightmost = nodes.reduce((max, node) => (node.position.x > max.position.x ? node : max));
  return { x: rightmost.position.x + 200, y: rightmost.position.y };
}

function clearValidationPending(current: LoopEditorState, generation: number): LoopEditorState {
  return current.pendingValidationGeneration === generation
    ? { ...current, pendingValidationGeneration: null }
    : current;
}

function clearPublishPending(current: LoopEditorState, generation: number): LoopEditorState {
  return current.pendingPublishGeneration === generation
    ? { ...current, pendingPublishGeneration: null }
    : current;
}

export const loopEditorLogic = createStoreLogic<
  LoopEditorState,
  LoopEditorEvents,
  LoopEditorEmitted,
  undefined
>({
  context: () => createLoopEditorState(),
  on: {
    connectionCreated: (current, event) => ({
      ...current,
      edges: event.edges,
      isDirty: true,
      structuralRevision: current.structuralRevision + 1,
      validationGeneration: current.validationGeneration + 1,
    }),
    contractFieldChanged: (current, event) => ({
      ...current,
      baseDefinition: event.definition,
      isDirty: true,
      publishError: null,
      structuralRevision: current.structuralRevision + 1,
      validationGeneration: current.validationGeneration + 1,
    }),
    draftInitialized: (current, event) => ({
      ...createLoopEditorState(current.scopeGeneration + 1),
      baseDefinition: event.definition,
      edges: event.edges,
      nodes: event.nodes,
      selectedNodeId: event.nodes[0]?.id ?? null,
      structuralRevision: current.structuralRevision + 1,
    }),
    graphEdgesApplied: (current, event) => ({
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
    graphNodesApplied: (current, event) => ({
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
    layoutApplied: (current, event) => ({
      ...current,
      nodes: event.nodes,
      positionsDirty: true,
      positionsGeneration: current.positionsGeneration + 1,
    }),
    nodeAdded: (current, event) => {
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
    nodeFieldChanged: (current, event) => ({
      ...current,
      nodes: event.nodes,
      isDirty: true,
      publishError: null,
      structuralRevision: current.structuralRevision + 1,
      validationGeneration: current.validationGeneration + 1,
    }),
    nodeRenamed: (current, event) => ({
      ...current,
      edges: event.edges,
      nodes: event.nodes,
      selectedNodeId: event.selectedNodeId,
      isDirty: true,
      publishError: null,
      structuralRevision: current.structuralRevision + 1,
      validationGeneration: current.validationGeneration + 1,
    }),
    nodeSelectionChanged: (current, event) => ({
      ...current,
      selectedNodeId: event.id,
      selectionSeq: current.selectionSeq + 1,
      sidebarTab: event.id === null ? current.sidebarTab : "node",
      view: event.reveal ? "graph" : current.view,
    }),
    positionsSaveFailed: (current, event, enqueue) => {
      if (
        current.scopeGeneration !== event.scopeGeneration ||
        current.pendingPositionsGeneration !== event.generation
      ) {
        return;
      }
      enqueue.effect(() =>
        notifyUser({ message: "Could not save node positions. Try again.", tone: "error" })
      );
      return { ...current, pendingPositionsGeneration: null };
    },
    positionsSaveRequested: (current, event, enqueue) => {
      if (!current.positionsDirty || current.pendingPositionsGeneration !== null) return;
      const generation = current.positionsGeneration;
      const scopeGeneration = current.scopeGeneration;
      const annotations = current.nodes.map(node => ({
        node_id: node.id,
        x: Math.round(node.position.x),
        y: Math.round(node.position.y),
      }));
      enqueue.effect(async ({ trigger }) => {
        try {
          await event.execute(annotations);
          trigger.positionsSaveSucceeded({ generation, scopeGeneration });
        } catch {
          trigger.positionsSaveFailed({ generation, scopeGeneration });
        }
      });
      return { ...current, pendingPositionsGeneration: generation };
    },
    positionsSaveSucceeded: (current, event) => {
      if (
        current.scopeGeneration !== event.scopeGeneration ||
        current.pendingPositionsGeneration !== event.generation
      ) {
        return;
      }
      return {
        ...current,
        pendingPositionsGeneration: null,
        positionsDirty:
          current.positionsGeneration === event.generation ? false : current.positionsDirty,
      };
    },
    publishFailed: (current, event) => {
      if (
        current.scopeGeneration !== event.scopeGeneration ||
        current.pendingPublishGeneration !== event.generation
      ) {
        return;
      }
      const next = clearPublishPending(current, event.generation);
      return { ...next, publishError: event.error };
    },
    publishRejected: (current, event) => {
      if (
        current.scopeGeneration !== event.scopeGeneration ||
        current.pendingPublishGeneration !== event.generation
      ) {
        return;
      }
      const next = clearPublishPending(current, event.generation);
      const lint = buildLintState(event.result);
      const publishError = `Publish rejected — ${lint.errorCount} issue${lint.errorCount === 1 ? "" : "s"} to resolve.`;
      if (current.structuralRevision !== event.revision) {
        return { ...next, publishError };
      }
      return {
        ...next,
        lint,
        nodes: applyLintToNodes(current.nodes, lint.byNode),
        publishError,
      };
    },
    publishRequested: (current, event, enqueue) => {
      if (!current.baseDefinition || current.pendingPublishGeneration !== null) return;
      const definition = graphToDefinition(current.baseDefinition, current.nodes, current.edges);
      const expectedVersion = current.baseDefinition.meta.version ?? null;
      const generation = current.publishGeneration + 1;
      const revision = current.structuralRevision;
      const scopeGeneration = current.scopeGeneration;
      enqueue.effect(async ({ trigger }) => {
        try {
          const loop = await event.execute(definition, expectedVersion);
          trigger.publishSucceeded({ generation, loop, revision, scopeGeneration });
        } catch (error) {
          if (error instanceof LoopValidationError) {
            trigger.publishRejected({
              generation,
              result: error.result,
              revision,
              scopeGeneration,
            });
            return;
          }
          trigger.publishFailed({
            error: error instanceof Error ? error.message : "Failed to publish loop",
            generation,
            revision,
            scopeGeneration,
          });
        }
      });
      return {
        ...current,
        pendingPublishGeneration: generation,
        publishError: null,
        publishGeneration: generation,
        validationGeneration: current.validationGeneration + 1,
      };
    },
    publishSucceeded: (current, event, enqueue) => {
      if (
        current.scopeGeneration !== event.scopeGeneration ||
        current.pendingPublishGeneration !== event.generation
      ) {
        return;
      }
      const next = clearPublishPending(current, event.generation);
      const baseDefinition = editorDefinitionFromLoop(event.loop);
      enqueue.effect(() =>
        notifyUser({
          message: `Published ${event.loop.name} v${event.loop.version}`,
          tone: "success",
        })
      );
      enqueue.emit.publishCompleted({ loop: event.loop });
      if (current.structuralRevision !== event.revision) {
        return { ...next, baseDefinition, isDirty: true };
      }
      const lint = buildLintState({ valid: true, errors: [] });
      return {
        ...next,
        baseDefinition,
        isDirty: false,
        lint,
        nodes: applyLintToNodes(current.nodes, lint.byNode),
      };
    },
    sidebarTabSelected: (current, event) => ({ ...current, sidebarTab: event.tab }),
    validationRequested: (current, event, enqueue) => {
      if (!current.baseDefinition) return;
      const definition = graphToDefinition(current.baseDefinition, current.nodes, current.edges);
      const generation = current.validationGeneration + 1;
      const scopeGeneration = current.scopeGeneration;
      enqueue.effect(async ({ trigger }) => {
        try {
          trigger.validationSucceeded({
            generation,
            result: await event.execute(definition),
            scopeGeneration,
          });
        } catch {
          trigger.validationUnavailable({
            generation,
            notifyFailure: event.notifyFailure,
            scopeGeneration,
          });
        }
      });
      return {
        ...current,
        pendingValidationGeneration: generation,
        validationGeneration: generation,
      };
    },
    validationSucceeded: (current, event) => {
      if (current.scopeGeneration !== event.scopeGeneration) return;
      const next = clearValidationPending(current, event.generation);
      if (event.generation !== current.validationGeneration) {
        return next === current ? undefined : next;
      }
      const lint = buildLintState(event.result);
      return {
        ...next,
        lint,
        nodes: applyLintToNodes(current.nodes, lint.byNode),
        validateFailed: false,
      };
    },
    validationUnavailable: (current, event, enqueue) => {
      if (current.scopeGeneration !== event.scopeGeneration) return;
      const next = clearValidationPending(current, event.generation);
      if (event.generation !== current.validationGeneration) {
        return next === current ? undefined : next;
      }
      if (event.notifyFailure) {
        enqueue.effect(() =>
          notifyUser({
            message: "Validation could not reach the daemon. Try again.",
            tone: "error",
          })
        );
      }
      return {
        ...next,
        validateFailed: true,
        lint: current.lint.validated ? { ...current.lint, validated: false } : current.lint,
      };
    },
    viewSelected: (current, event) => ({ ...current, view: event.view }),
  },
});
