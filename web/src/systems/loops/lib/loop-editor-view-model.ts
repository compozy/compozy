import { graphToDefinition, type EditorEdge, type EditorNode } from "./codec";
import { buildDslView } from "./loop-dsl";
import { forwardNodeIds } from "./loop-editor-route-edges";
import { buildNodeFields } from "./loop-node-schema";
import type { LoopLintState } from "./loop-editor-lint";
import type { LoopDefinition, LoopDetail } from "../types";
import type { LoopEditorView } from "../hooks/use-loop-editor-state";

export type LoopEditorStatus = "no-workspace" | "loading" | "error" | "ready";

interface LoopEditorViewModelInput {
  baseDefinition: LoopDefinition | null;
  busy: boolean;
  definitionEditable: boolean;
  edges: EditorEdge[];
  lint: LoopLintState;
  loop: LoopDetail | undefined;
  queryError: Error | null;
  queryLoading: boolean;
  nodes: EditorNode[];
  selectedNodeId: string | null;
  view: LoopEditorView;
  workspaceId: string;
}

export function loopEditorViewModel({
  baseDefinition,
  busy,
  definitionEditable,
  edges,
  lint,
  loop,
  queryError,
  queryLoading,
  nodes,
  selectedNodeId,
  view,
  workspaceId,
}: LoopEditorViewModelInput) {
  const selectedNodeIds: string[] = [];
  for (const node of nodes) {
    if (node.selected) selectedNodeIds.push(node.id);
  }
  const selectedNode = nodes.find(node => node.id === selectedNodeId) ?? null;
  const selectedFields = selectedNode
    ? buildNodeFields(selectedNode.data.raw, baseDefinition ?? undefined, {
        forwardNodeIds: forwardNodeIds(nodes, edges, selectedNode.id),
      })
    : [];
  const dslLines =
    baseDefinition && view === "dsl"
      ? buildDslView(graphToDefinition(baseDefinition, nodes, edges), lint.byNode)
      : [];
  const status: LoopEditorStatus =
    workspaceId === ""
      ? "no-workspace"
      : queryLoading
        ? "loading"
        : queryError || !loop
          ? "error"
          : "ready";
  return {
    definition: baseDefinition ?? loop?.definition,
    dslLines,
    errorMessage: queryError?.message,
    publishDisabled: !definitionEditable || lint.hasBlockingErrors || busy,
    selectedFields,
    selectedNode,
    selectedNodeIds,
    status,
    version: baseDefinition?.meta.version ?? loop?.version,
  };
}
