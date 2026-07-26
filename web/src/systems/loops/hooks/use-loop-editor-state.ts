import { useReducer, type SetStateAction } from "react";

import type { EditorEdge, EditorNode, RawLoopNode } from "../lib/codec";
import { emptyLintState, type LoopLintState } from "../lib/loop-editor-lint";
import { uniqueNodeId, type PaletteItem } from "../lib/loop-palette";
import type { LoopDefinition } from "../types";

export type LoopEditorView = "graph" | "dsl";
export type LoopEditorSidebarTab = "contract" | "node";

interface LoopEditorState {
  baseDefinition: LoopDefinition | null;
  edges: EditorEdge[];
  isDirty: boolean;
  lint: LoopLintState;
  nodes: EditorNode[];
  positionsDirty: boolean;
  publishError: string | null;
  selectedNodeId: string | null;
  selectionSeq: number;
  sidebarTab: LoopEditorSidebarTab;
  structuralRevision: number;
  validateFailed: boolean;
  view: LoopEditorView;
}

interface LoopEditorStateAction {
  update: (current: LoopEditorState) => LoopEditorState;
}

function createLoopEditorState(): LoopEditorState {
  return {
    baseDefinition: null,
    edges: [],
    isDirty: false,
    lint: emptyLintState(),
    nodes: [],
    positionsDirty: false,
    publishError: null,
    selectedNodeId: null,
    selectionSeq: 0,
    sidebarTab: "contract",
    structuralRevision: 0,
    validateFailed: false,
    view: "graph",
  };
}

function updateLoopEditorState(
  current: LoopEditorState,
  action: LoopEditorStateAction
): LoopEditorState {
  return action.update(current);
}

function nodeKind(raw: RawLoopNode): string {
  return typeof raw.kind === "string" ? raw.kind : "";
}

function nextDropPosition(nodes: readonly EditorNode[]): { x: number; y: number } {
  if (nodes.length === 0) return { x: 40, y: 40 };
  const rightmost = nodes.reduce((max, node) => (node.position.x > max.position.x ? node : max));
  return { x: rightmost.position.x + 200, y: rightmost.position.y };
}

export function useLoopEditorState() {
  const [state, dispatch] = useReducer(updateLoopEditorState, undefined, createLoopEditorState);
  const setField = <Field extends keyof LoopEditorState>(
    field: Field,
    value: SetStateAction<LoopEditorState[Field]>
  ) => {
    dispatch({
      update: current => {
        const currentValue = current[field];
        const nextValue =
          typeof value === "function"
            ? (value as (previous: LoopEditorState[Field]) => LoopEditorState[Field])(currentValue)
            : value;
        return { ...current, [field]: nextValue };
      },
    });
  };

  const initialize = (definition: LoopDefinition, edges: EditorEdge[], nodes: EditorNode[]) => {
    dispatch({
      update: current => ({
        ...createLoopEditorState(),
        baseDefinition: definition,
        edges,
        nodes,
        selectedNodeId: nodes[0]?.id ?? null,
        structuralRevision: current.structuralRevision + 1,
      }),
    });
  };

  const addNode = (item: PaletteItem) => {
    dispatch({
      update: current => {
        const existing = new Set(current.nodes.map(node => node.id));
        const id = uniqueNodeId(item.idBase, existing);
        const raw = item.buildRaw(id);
        const node: EditorNode = {
          id,
          type: "loopNode",
          position: nextDropPosition(current.nodes),
          data: { raw, nodeClass: item.nodeClass, kind: nodeKind(raw), hasError: false },
        };
        return {
          ...current,
          isDirty: true,
          nodes: [...current.nodes, node],
          selectedNodeId: id,
          selectionSeq: current.selectionSeq + 1,
          sidebarTab: "node",
          structuralRevision: current.structuralRevision + 1,
        };
      },
    });
  };

  return {
    ...state,
    addNode,
    initialize,
    setBaseDefinition: (value: SetStateAction<LoopDefinition | null>) =>
      setField("baseDefinition", value),
    setDirty: (value: SetStateAction<boolean>) => setField("isDirty", value),
    setEdges: (value: SetStateAction<EditorEdge[]>) => setField("edges", value),
    setLint: (value: SetStateAction<LoopLintState>) => setField("lint", value),
    setNodes: (value: SetStateAction<EditorNode[]>) => setField("nodes", value),
    setPositionsDirty: (value: SetStateAction<boolean>) => setField("positionsDirty", value),
    setPublishError: (value: SetStateAction<string | null>) => setField("publishError", value),
    setSelectedNodeId: (value: SetStateAction<string | null>) => setField("selectedNodeId", value),
    setSelectionSeq: (value: SetStateAction<number>) => setField("selectionSeq", value),
    setSidebarTab: (value: SetStateAction<LoopEditorSidebarTab>) => setField("sidebarTab", value),
    markStructureChanged: () =>
      setField("structuralRevision", currentRevision => currentRevision + 1),
    setValidateFailed: (value: SetStateAction<boolean>) => setField("validateFailed", value),
    setView: (value: SetStateAction<LoopEditorView>) => setField("view", value),
  };
}
