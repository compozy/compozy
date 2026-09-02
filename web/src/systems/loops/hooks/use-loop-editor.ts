import { useEffect, useEffectEvent, useState } from "react";
import type { Connection, EdgeChange, NodeChange } from "@xyflow/react";

import { useLoop, useLoopAnnotations } from "./use-loops";
import { usePatchLoop, usePutLoopAnnotations, useValidateLoop } from "./use-loop-actions";
import type { EditorEdge, EditorNode } from "../lib/codec";
import type { DslLine } from "../lib/loop-dsl";
import {
  withLoopContractField,
  withLoopContractPath,
  type EditableLoopContractField,
} from "../lib/loop-editor-definition";
import { isNodeIdPath, renameNodeId, type NodeFieldEdit } from "../lib/loop-editor-draft";
import type { LoopLintState } from "../lib/loop-editor-lint";
import { layoutEditorGraph } from "../lib/loop-editor-layout";
import type { FieldPath, FieldSpec } from "../lib/loop-node-schema";
import { loopEditorViewModel, type LoopEditorStatus } from "../lib/loop-editor-view-model";
import type { PaletteItem } from "../lib/loop-palette";
import type { LoopDefinition, LoopDetail, LoopValidationIssue } from "../types";
import {
  useLoopEditorState,
  type LoopEditorSidebarTab,
  type LoopEditorView,
} from "./use-loop-editor-state";
import { useLoopEditorClipboard } from "./use-loop-editor-clipboard";
import { useLoopEditorSynchronization } from "./use-loop-editor-synchronization";
import { useLoopEditorGraphActions } from "./use-loop-editor-graph-actions";

export type { LoopEditorStatus } from "../lib/loop-editor-view-model";
export type { LoopEditorSidebarTab, LoopEditorView } from "./use-loop-editor-state";

export interface UseLoopEditorResult {
  status: LoopEditorStatus;
  loop: LoopDetail | undefined;
  definition: LoopDefinition | undefined;
  errorMessage: string | undefined;
  version: number | undefined;
  nodes: EditorNode[];
  edges: EditorEdge[];
  selectedNode: EditorNode | null;
  selectedFields: FieldSpec[];
  /** Increments only on a selection switch (not a rename) — the inspector's remount key. */
  selectionSeq: number;
  sidebarTab: LoopEditorSidebarTab;
  selectSidebarTab: (tab: LoopEditorSidebarTab) => void;
  view: LoopEditorView;
  selectView: (view: LoopEditorView) => void;
  isDirty: boolean;
  positionsDirty: boolean;
  /** Definition writes are allowed only for workspace-owned loops. */
  definitionEditable: boolean;
  lint: LoopLintState;
  validateFailed: boolean;
  publishDisabled: boolean;
  busy: boolean;
  publishError: string | null;
  /** Discriminator for `publishError`: a 422 validation rejection vs a transport/unknown failure. */
  publishFailureKind: "rejected" | "transport" | null;
  /** The issue list a publish 422 returned; empty for a transport failure. */
  publishRejectedIssues: LoopValidationIssue[];
  dslLines: DslLine[];
  onNodesChange: (changes: NodeChange<EditorNode>[]) => void;
  onEdgesChange: (changes: EdgeChange<EditorEdge>[]) => void;
  onConnect: (connection: Connection) => void;
  selectNode: (id: string | null) => void;
  revealNode: (id: string) => void;
  /** Increments on Reveal node so the canvas can center without a stored flow handle. */
  revealSeq: number;
  /** True when a 422 landed after the draft moved on — the strip lists issues. */
  publishRejectedDockStale: boolean;
  changeField: (path: FieldPath, value: unknown) => void;
  /** Applies several path writes to the selected node as ONE draft transition. */
  changeFields: (edits: NodeFieldEdit[]) => void;
  changeContract: (field: EditableLoopContractField, value: string) => void;
  changeContractPath: (path: FieldPath, value: unknown) => void;
  addNode: (item: PaletteItem, position?: { x: number; y: number }) => void;

  addNodeWithEdge: (item: PaletteItem, position: { x: number; y: number }, source: string) => void;

  onNodesDelete: (deleted: EditorNode[]) => void;
  deleteNodes: (nodeIds: string[]) => void;

  selectedNodeIds: string[];
  copyNodes: (nodeIds: string[]) => void;
  duplicateNodes: (nodeIds: string[]) => void;
  pasteNodes: () => void;
  canPaste: boolean;
  autoLayout: () => void;
  validate: () => void;
  publish: () => void;
  savePositions: () => void;
}

export function useLoopEditor(
  workspaceId: string,
  name: string,
  onPublished?: (loop: LoopDetail) => void,
  liveDataEnabled = true
): UseLoopEditorResult {
  const enabled = workspaceId !== "" && name !== "" && liveDataEnabled;
  const loopQuery = useLoop(workspaceId, name, enabled);
  const annotationsQuery = useLoopAnnotations(workspaceId, name, enabled);
  const validateMutation = useValidateLoop();
  const patchMutation = usePatchLoop();
  const annotationsMutation = usePutLoopAnnotations();

  const {
    addNode,
    addNodeWithEdge,
    deleteNodes,
    pasteNodes,
    baseDefinition,
    busy,
    edges,
    initializedSourceKey,
    isDirty,
    lint,
    nodes,
    positionsDirty,
    publishError,
    publishFailureKind,
    publishRejectedIssues,
    publishRejectedDockStale,
    selectedNodeId,
    selectionSeq,
    sidebarTab,
    structuralRevision,
    validateFailed,
    view,
    applyGraphEdges,
    applyGraphNodes,
    applyLayout,
    changeContract: changeEditorContract,
    changeNodeField,
    editNodeFields,
    connectNodes,
    renameNode,
    requestPositionsSave,
    requestPublish,
    requestValidation,
    selectNode: selectEditorNode,
    selectSidebarTab,
    selectView,
    store,
  } = useLoopEditorState();

  const [revealSeq, setRevealSeq] = useState(0);

  useLoopEditorSynchronization({
    annotationsQuery,
    initializedSourceKey,
    loopQuery,
    name,
    onPublished,
    store,
    workspaceId,
  });

  const runValidation = (options: { notify?: boolean } = {}) =>
    requestValidation(
      definition =>
        validateMutation.mutateAsync({
          workspaceId,
          name,
          data: { definition },
        }),
      options.notify ?? false
    );

  // Live re-lint after structural edits so the chips + Publish gate stay truthful.
  // The Effect Event reads the latest draft without making the debounce depend on
  // the render-local validation function identity.
  const runAutoValidation = useEffectEvent(() => runValidation());
  useEffect(() => {
    store.trigger.automaticValidationRequested({
      enabled,
      execute: runAutoValidation,
      revision: structuralRevision,
    });
  }, [enabled, store, structuralRevision]);

  // Positions live in the annotations sidecar and remain editable for read-only definitions.
  const definitionEditable = loopQuery.data?.source === "workspace";
  const clipboard = useLoopEditorClipboard({
    definitionEditable,
    edges,
    nodes,
    pasteNodes,
  });
  const graphActions = useLoopEditorGraphActions({
    actions: { applyGraphEdges, applyGraphNodes, changeNodeField, connectNodes, deleteNodes },
    definitionEditable,
    edges,
    nodes,
  });

  const selectNode = (id: string | null) => {
    selectEditorNode(id);
  };
  const revealNode = (id: string) => {
    selectEditorNode(id, true);
    setRevealSeq(seq => seq + 1);
  };

  const changeFields = (edits: NodeFieldEdit[]) => {
    const targetId = selectedNodeId;
    if (!definitionEditable || !targetId || edits.length === 0) return;
    editNodeFields(targetId, edits);
  };

  const changeField = (path: FieldPath, value: unknown) => {
    const targetId = selectedNodeId;
    if (!definitionEditable || !targetId) return;
    if (isNodeIdPath(path)) {
      const newId = String(value).trim();
      if (newId === "" || newId === targetId) return;
      // Reject a rename onto an id another node already uses — two nodes sharing an id
      // would duplicate React Flow keys and make selection ambiguous before the daemon
      // rejects it. The author keeps the old id until they pick a free one.
      if (nodes.some(node => node.id === newId)) return;
      const renamed = renameNodeId(nodes, edges, targetId, newId);
      renameNode(renamed.edges, renamed.nodes, newId);
      return;
    }
    changeFields([{ path, value }]);
  };

  const changeContract = (field: EditableLoopContractField, value: string) => {
    if (!definitionEditable || !baseDefinition) return;
    changeEditorContract(withLoopContractField(baseDefinition, field, value));
  };

  const changeContractPath = (path: FieldPath, value: unknown) => {
    if (!definitionEditable || !baseDefinition) return;
    changeEditorContract(withLoopContractPath(baseDefinition, path, value));
  };

  const publish = () => {
    // Publish validates atomically on the daemon. Its verdict must not be overwritten by an
    // older passive validation that was queued or already in flight for the same draft.
    store.trigger.automaticValidationCancelled();

    requestPublish((definition, expectedVersion) =>
      patchMutation.mutateAsync({
        workspaceId,
        name,
        data: { definition, expected_version: expectedVersion },
      })
    );
  };

  const autoLayout = () => {
    applyLayout(layoutEditorGraph(nodes, edges, []));
  };

  const savePositions = () =>
    requestPositionsSave(annotations =>
      annotationsMutation.mutateAsync({ workspaceId, name, data: { annotations } })
    );

  const viewModel = loopEditorViewModel({
    baseDefinition,
    busy,
    definitionEditable,
    edges,
    lint,
    loop: loopQuery.data,
    queryError: loopQuery.error,
    queryLoading: loopQuery.isLoading,
    nodes,
    selectedNodeId,
    view,
    workspaceId,
  });

  return {
    status: viewModel.status,
    loop: loopQuery.data,
    definition: viewModel.definition,
    errorMessage: viewModel.errorMessage,
    version: viewModel.version,
    nodes,
    edges,
    selectedNode: viewModel.selectedNode,
    selectedFields: viewModel.selectedFields,
    selectionSeq,
    sidebarTab,
    selectSidebarTab,
    view,
    selectView,
    isDirty,
    positionsDirty,
    definitionEditable,
    lint,
    validateFailed,
    // Gated on known blocking errors only: when no verdict exists (e.g. validate is
    // unreachable) Publish stays enabled because publish runs the shared linter atomically
    // and returns a 422 the editor maps onto nodes — no invalid definition can ship.
    publishDisabled: viewModel.publishDisabled,
    busy,
    publishError,
    publishFailureKind,
    publishRejectedIssues,
    publishRejectedDockStale,
    revealSeq,
    dslLines: viewModel.dslLines,
    onNodesChange: graphActions.onNodesChange,
    onEdgesChange: graphActions.onEdgesChange,
    onConnect: graphActions.onConnect,
    selectNode,
    revealNode,
    changeField,
    changeFields,
    changeContract,
    changeContractPath,
    addNode: (item: PaletteItem, position?: { x: number; y: number }) => {
      if (!definitionEditable) return;
      addNode(item, position);
    },
    addNodeWithEdge: (item: PaletteItem, position: { x: number; y: number }, source: string) => {
      if (!definitionEditable) return;
      addNodeWithEdge(item, position, source);
    },
    onNodesDelete: graphActions.onNodesDelete,
    deleteNodes: (nodeIds: string[]) => {
      if (!definitionEditable) return;
      deleteNodes(nodeIds);
    },

    selectedNodeIds: viewModel.selectedNodeIds,
    copyNodes: clipboard.copyNodes,
    duplicateNodes: clipboard.duplicateNodes,
    pasteNodes: clipboard.pasteNodes,
    canPaste: clipboard.canPaste,
    autoLayout,
    validate: () => runValidation({ notify: true }),
    publish,
    savePositions,
  };
}
