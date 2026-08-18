import { useRef } from "react";
import { useReactFlow } from "@xyflow/react";

import type { LoopEditorNodeActions } from "../components/editor/loop-editor-node";
import type { UseLoopEditorResult } from "./use-loop-editor";
import { useLoopEditorChrome } from "./use-loop-editor-chrome";
import { useLoopEditorOverlays } from "./use-loop-editor-overlays";
import { useLoopEditorShortcuts } from "./use-loop-editor-shortcuts";
import { EDITOR_NODE_HEIGHT, EDITOR_NODE_WIDTH } from "../lib/loop-editor-layout";
import type { PaletteItem } from "../lib/loop-palette";

function actionTargets(selectedNodeIds: readonly string[], nodeId: string): string[] {
  return selectedNodeIds.includes(nodeId) ? [...selectedNodeIds] : [nodeId];
}

export function useLoopEditorReadyActions(editor: UseLoopEditorResult) {
  const readOnly = !editor.definitionEditable;
  const chrome = useLoopEditorChrome();
  const overlays = useLoopEditorOverlays();
  const editorRoot = useRef<HTMLDivElement>(null);
  const { screenToFlowPosition } = useReactFlow();

  const selectNode = (id: string | null) => {
    editor.selectNode(id);
    if (id !== null) chrome.openInspector();
  };
  const revealNode = (id: string) => {
    editor.revealNode(id);
    chrome.openInspector();
  };
  const addNode = (item: PaletteItem, position?: { x: number; y: number }) => {
    if (readOnly) return;
    editor.addNode(item, position);
    chrome.openInspector();
  };
  const openQuickAddAtCenter = () => {
    const canvas = editorRoot.current?.querySelector<HTMLElement>(
      '[data-testid="loop-editor-canvas"]'
    );
    if (!canvas) return;
    const rect = canvas.getBoundingClientRect();
    const center = screenToFlowPosition({
      x: rect.left + rect.width / 2,
      y: rect.top + rect.height / 2,
    });
    overlays.openQuickAdd({
      x: center.x - EDITOR_NODE_WIDTH / 2,
      y: center.y - EDITOR_NODE_HEIGHT / 2,
    });
  };

  const nodeActions: LoopEditorNodeActions = {
    readOnly,
    canPaste: editor.canPaste,
    onCopy: nodeId => editor.copyNodes(actionTargets(editor.selectedNodeIds, nodeId)),
    onDuplicate: nodeId => editor.duplicateNodes(actionTargets(editor.selectedNodeIds, nodeId)),
    onPaste: () => editor.pasteNodes(),
    onRename: selectNode,
    onDelete: nodeId => editor.deleteNodes(actionTargets(editor.selectedNodeIds, nodeId)),
  };

  useLoopEditorShortcuts(editor.view === "graph", {
    onQuickAdd: openQuickAddAtCenter,
    onTogglePalette: chrome.togglePalette,
    onToggleInspector: chrome.toggleInspector,
  });

  return { addNode, chrome, editorRoot, nodeActions, overlays, readOnly, revealNode, selectNode };
}
