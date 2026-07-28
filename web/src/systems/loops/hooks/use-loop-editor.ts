import { useEffect, useEffectEvent, useRef } from "react";
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
  type EditableLoopContractField,
} from "../lib/loop-editor-definition";
import { isNodeIdPath, renameNodeId, setNodeField } from "../lib/loop-editor-draft";
import type { LoopLintState } from "../lib/loop-editor-lint";
import { layoutEditorGraph } from "../lib/loop-editor-layout";
import { buildNodeFields, type FieldPath, type FieldSpec } from "../lib/loop-node-schema";
import type { PaletteItem } from "../lib/loop-palette";
import type { LoopDefinition, LoopDetail } from "../types";
import {
  useLoopEditorState,
  type LoopEditorSidebarTab,
  type LoopEditorView,
} from "./use-loop-editor-state";

const AUTO_VALIDATE_DEBOUNCE_MS = 400;

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
  lint: LoopLintState;
  validateFailed: boolean;
  publishDisabled: boolean;
  busy: boolean;
  publishError: string | null;
  dslLines: DslLine[];
  onNodesChange: (changes: NodeChange<EditorNode>[]) => void;
  onEdgesChange: (changes: EdgeChange<EditorEdge>[]) => void;
  onConnect: (connection: Connection) => void;
  selectNode: (id: string | null) => void;
  revealNode: (id: string) => void;
  changeField: (path: FieldPath, value: unknown) => void;
  changeContract: (field: EditableLoopContractField, value: string) => void;
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
  onPublished?: (loop: LoopDetail) => void
): UseLoopEditorResult {
  const enabled = workspaceId !== "" && name !== "";
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
    initialize,
    isDirty,
    lint,
    nodes,
    positionsDirty,
    publishError,
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

  const initedKeyRef = useRef<string | null>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const annotationsErrorNotifiedRef = useRef(false);
  const handlePublished = useEffectEvent((loop: LoopDetail) => {
    onPublished?.(loop);
  });

  // Seed the editable draft once the definition + settled sidecar arrive. This syncs
  // server state into local editor state (a legit external-system → draft sync).
  useEffect(() => {
    const loop = loopQuery.data;
    if (!loop || annotationsQuery.isLoading) return;
    const definition = editorDefinitionFromLoop(loop);
    const key = `${workspaceId}:${name}`;
    if (initedKeyRef.current === key) return;
    initedKeyRef.current = key;
    const graph = definitionToGraph(definition);
    const laid = layoutEditorGraph(graph.nodes, graph.edges, annotationsQuery.data ?? []);
    initialize(definition, graph.edges, laid);
  }, [
    loopQuery.data,
    annotationsQuery.data,
    annotationsQuery.isLoading,
    workspaceId,
    name,
    initialize,
  ]);

  // Positions are cosmetic (auto-layout is the fallback), but a broken sidecar should be
  // observable, not silently swallowed. Surface it once per error, non-blocking.
  useEffect(() => {
    if (annotationsQuery.isError && !annotationsErrorNotifiedRef.current) {
      annotationsErrorNotifiedRef.current = true;
      toast.error("Could not load saved node positions — using auto-layout.");
    }
    if (!annotationsQuery.isError) annotationsErrorNotifiedRef.current = false;
  }, [annotationsQuery.isError]);

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
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      void runAutoValidation();
    }, AUTO_VALIDATE_DEBOUNCE_MS);
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [structuralRevision]);

  const onNodesChange = (changes: NodeChange<EditorNode>[]) => {
    let positionsChanged = false;
    let structureChanged = false;
    for (const change of changes) {
      if (change.type === "position") positionsChanged = true;
      if (change.type === "remove") structureChanged = true;
    }
    applyGraphNodes(applyNodeChanges(changes, nodes), positionsChanged, structureChanged);
  };

  const onEdgesChange = (changes: EdgeChange<EditorEdge>[]) => {
    applyGraphEdges(
      applyEdgeChanges(changes, edges),
      changes.some(change => change.type === "remove")
    );
  };

  const onConnect = (connection: Connection) => {
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
  };

  const changeField = (path: FieldPath, value: unknown) => {
    const targetId = selectedNodeId;
    if (!targetId) return;
    if (isNodeIdPath(path)) {
      const newId = String(value).trim();
      if (newId === "" || newId === targetId) return;
      // Reject a rename onto an id another node already uses — two nodes sharing an id
      // would duplicate React Flow keys and make selection ambiguous before the daemon
      // rejects it. The author keeps the old id until they pick a free one.
      if (nodes.some(node => node.id === newId)) return;
      const renamed = renameNodeId(nodes, edges, targetId, newId);
      renameNode(renamed.edges, renamed.nodes, newId);
    } else {
      changeNodeField(setNodeField(nodes, targetId, path, value));
    }
  };

  const changeContract = (field: EditableLoopContractField, value: string) => {
    changeEditorContract(
      baseDefinition ? withLoopContractField(baseDefinition, field, value) : null
    );
  };

  const publish = () => {
    // Publish validates atomically on the daemon. Its verdict must not be overwritten by an
    // older passive validation that was queued or already in flight for the same draft.
    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
      debounceRef.current = null;
    }

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
    lint,
    validateFailed,
    // Gated on known blocking errors only: when no verdict exists (e.g. validate is
    // unreachable) Publish stays enabled because publish runs the shared linter atomically
    // and returns a 422 the editor maps onto nodes — no invalid definition can ship.
    publishDisabled: loopQuery.data?.source !== "workspace" || lint.hasBlockingErrors || busy,
    busy,
    publishError,
    dslLines,
    onNodesChange,
    onEdgesChange,
    onConnect,
    selectNode,
    revealNode,
    changeField,
    changeContract,
    addNode,
    autoLayout,
    validate: () => runValidation({ notify: true }),
    publish,
    savePositions,
  };
}
