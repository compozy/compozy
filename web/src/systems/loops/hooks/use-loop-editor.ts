import { useEffect, useEffectEvent, useState } from "react";
import {
  addEdge,
  applyEdgeChanges,
  applyNodeChanges,
  type Connection,
  type EdgeChange,
  type NodeChange,
} from "@xyflow/react";
import { toast } from "sonner";

import { useLoop, useLoopAnnotations } from "./use-loops";
import { usePatchLoop, usePutLoopAnnotations, useValidateLoop } from "./use-loop-actions";
import {
  definitionToGraph,
  editorEdgeId,
  graphToDefinition,
  type EditorEdge,
  type EditorNode,
} from "../lib/codec";
import { buildDslView, type DslLine } from "../lib/loop-dsl";
import {
  editorDefinitionFromLoop,
  withLoopContractField,
  withLoopContractPath,
  type EditableLoopContractField,
} from "../lib/loop-editor-definition";
import {
  isNodeIdPath,
  renameNodeId,
  setNodeFields,
  type NodeFieldEdit,
} from "../lib/loop-editor-draft";
import type { LoopLintState } from "../lib/loop-editor-lint";
import { layoutEditorGraph } from "../lib/loop-editor-layout";
import { buildNodeFields, type FieldPath, type FieldSpec } from "../lib/loop-node-schema";
import type { PaletteItem } from "../lib/loop-palette";
import type { LoopDefinition, LoopDetail, LoopValidationIssue } from "../types";
import {
  useLoopEditorState,
  type LoopEditorSidebarTab,
  type LoopEditorView,
} from "./use-loop-editor-state";

export type LoopEditorStatus = "no-workspace" | "loading" | "error" | "ready";
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
  addNode: (item: PaletteItem) => void;
  autoLayout: () => void;
  validate: () => void;
  publish: () => void;
  savePositions: () => void;
}

/**
 * The fork-and-edit editor view-model: it loads the one canonical definition + its
 * position sidecar, holds the draft as editor-session state (no server draft store,
 * §9.13), and drives the bijective codec, the shared-linter validate loop, the publish
 * (expected_version CAS), and the positions save. The GUI never owns invariants — every
 * chip and per-node badge comes from a `validate`/publish verdict.
 */
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
    baseDefinition,
    busy,
    edges,
    initializedSourceKey,
    isDirty,
    lint,
    nodes,
    positionsDirty,
    publishError,
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

  const handlePublished = useEffectEvent((loop: LoopDetail) => {
    onPublished?.(loop);
  });

  // Seed the editable draft once the definition + settled sidecar arrive. This syncs
  // server state into local editor state (a legit external-system → draft sync).
  //
  // A same-name Fork changes only `source`; include it so the workspace copy re-seeds the draft.
  useEffect(() => {
    const loop = loopQuery.data;
    if (!loop || annotationsQuery.isLoading) return;
    const definition = editorDefinitionFromLoop(loop);
    const key = `${workspaceId}:${name}:${loop.source}`;
    if (initializedSourceKey === key) return;
    const graph = definitionToGraph(definition);
    const laid = layoutEditorGraph(graph.nodes, graph.edges, annotationsQuery.data ?? []);
    store.trigger.draftInitialized({ definition, edges: graph.edges, nodes: laid, sourceKey: key });
  }, [
    loopQuery.data,
    annotationsQuery.data,
    annotationsQuery.isLoading,
    workspaceId,
    name,
    initializedSourceKey,
    store,
  ]);

  // Positions are cosmetic (auto-layout is the fallback), but a broken sidecar should be
  // observable, not silently swallowed. Surface it once per error, non-blocking.
  useEffect(() => {
    store.trigger.annotationsStatusObserved({
      failed: annotationsQuery.isError,
      notify: () => toast.error("Could not load saved node positions — using auto-layout."),
    });
  }, [annotationsQuery.isError, store]);

  useEffect(() => {
    const published = store.on("publishCompleted", event => handlePublished(event.loop));
    return () => {
      published.unsubscribe();
    };
  }, [store]);

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

  useEffect(() => () => store.trigger.lifecycleDisposed(), [store]);

  // Positions live in the annotations sidecar and remain editable for read-only definitions.
  const definitionEditable = loopQuery.data?.source === "workspace";

  const onNodesChange = (changes: NodeChange<EditorNode>[]) => {
    // Drop structural changes on a read-only definition; keep position/selection changes so the
    // canvas stays readable and layout (a sidecar concern) still works.
    const allowed = definitionEditable
      ? changes
      : changes.filter(change => change.type !== "remove");
    if (allowed.length === 0) return;
    let positionsChanged = false;
    let structureChanged = false;
    for (const change of allowed) {
      if (change.type === "position") positionsChanged = true;
      if (change.type === "remove") structureChanged = true;
    }
    applyGraphNodes(applyNodeChanges(allowed, nodes), positionsChanged, structureChanged);
  };

  const onEdgesChange = (changes: EdgeChange<EditorEdge>[]) => {
    const allowed = definitionEditable
      ? changes
      : changes.filter(change => change.type !== "remove");
    if (allowed.length === 0) return;
    applyGraphEdges(
      applyEdgeChanges(allowed, edges),
      allowed.some(change => change.type === "remove")
    );
  };

  const onConnect = (connection: Connection) => {
    if (!definitionEditable) return;
    const { source, target } = connection;
    if (!source || !target) return;
    const edge: EditorEdge = {
      id: editorEdgeId(source, target, edges.length),
      source,
      target,
      data: { raw: { from: source, to: target } },
    };
    connectNodes(addEdge(edge, edges));
  };

  // Bumped on a genuine selection *switch* (click / reveal / add) but NOT on a rename of the
  // already-selected node, so the inspector's field container is keyed by this — a rename
  // never remounts it (which would drop focus after each keystroke, R-001 round 7).
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
    changeNodeField(setNodeFields(nodes, targetId, edits));
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

  const selectedNode = nodes.find(node => node.id === selectedNodeId) ?? null;
  const selectedFields = selectedNode
    ? buildNodeFields(selectedNode.data.raw, baseDefinition ?? undefined)
    : [];
  const dslBase = baseDefinition;
  // Only serialize when the DSL panel is visible — skip graph conversion + YAML
  // emission entirely while editing on the Graph canvas.
  const dslLines =
    dslBase && view === "dsl"
      ? buildDslView(graphToDefinition(dslBase, nodes, edges), lint.byNode)
      : [];

  const status: LoopEditorStatus =
    workspaceId === ""
      ? "no-workspace"
      : loopQuery.isLoading
        ? "loading"
        : loopQuery.error || !loopQuery.data
          ? "error"
          : "ready";

  return {
    status,
    loop: loopQuery.data,
    definition: baseDefinition ?? loopQuery.data?.definition,
    errorMessage: loopQuery.error?.message,
    version: baseDefinition?.meta.version ?? loopQuery.data?.version,
    nodes,
    edges,
    selectedNode,
    selectedFields,
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
    publishDisabled: !definitionEditable || lint.hasBlockingErrors || busy,
    busy,
    publishError,
    publishRejectedIssues,
    publishRejectedDockStale,
    revealSeq,
    dslLines,
    onNodesChange,
    onEdgesChange,
    onConnect,
    selectNode,
    revealNode,
    changeField,
    changeFields,
    changeContract,
    changeContractPath,
    addNode: (item: PaletteItem) => {
      if (!definitionEditable) return;
      addNode(item);
    },
    autoLayout,
    validate: () => runValidation({ notify: true }),
    publish,
    savePositions,
  };
}
