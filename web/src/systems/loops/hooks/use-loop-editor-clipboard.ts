import { useState } from "react";

import {
  copyEditorSelection,
  pasteEditorClipboard,
  type LoopEditorClipboardContent,
} from "../lib/loop-editor-clipboard";
import type { EditorEdge, EditorNode } from "../lib/codec";

interface UseLoopEditorClipboardOptions {
  definitionEditable: boolean;
  edges: EditorEdge[];
  nodes: EditorNode[];
  pasteNodes: (edges: EditorEdge[], nodes: EditorNode[], selectedNodeId: string) => void;
}

export function useLoopEditorClipboard({
  definitionEditable,
  edges,
  nodes,
  pasteNodes,
}: UseLoopEditorClipboardOptions) {
  const [clipboard, setClipboard] = useState<LoopEditorClipboardContent | null>(null);

  const paste = (content: LoopEditorClipboardContent): void => {
    if (!definitionEditable) return;
    const pasted = pasteEditorClipboard(nodes, edges, content);
    if (!pasted) return;
    pasteNodes(pasted.edges, pasted.nodes, pasted.selectedNodeId);
  };

  return {
    canPaste: clipboard !== null,
    copyNodes: (nodeIds: string[]) => {
      if (nodeIds.length === 0) return;
      setClipboard(copyEditorSelection(nodes, edges, nodeIds));
    },
    duplicateNodes: (nodeIds: string[]) => {
      if (nodeIds.length === 0) return;
      paste(copyEditorSelection(nodes, edges, nodeIds));
    },
    pasteNodes: () => {
      if (clipboard) paste(clipboard);
    },
  };
}
