import { useState } from "react";

import type { LoopEditorConnectionDrop } from "../components/editor/loop-editor-canvas";

export interface LoopEditorOverlays {
  quickAdd: { position: { x: number; y: number } | null } | null;
  connectionDrop: LoopEditorConnectionDrop | null;
  openQuickAdd: (position?: { x: number; y: number }) => void;
  setQuickAddOpen: (open: boolean) => void;
  openConnectionDrop: (drop: LoopEditorConnectionDrop) => void;
  closeConnectionDrop: () => void;
}

export function useLoopEditorOverlays(): LoopEditorOverlays {
  const [quickAdd, setQuickAdd] = useState<{ position: { x: number; y: number } | null } | null>(
    null
  );
  const [connectionDrop, setConnectionDrop] = useState<LoopEditorConnectionDrop | null>(null);

  return {
    quickAdd,
    connectionDrop,
    openQuickAdd: position => setQuickAdd({ position: position ?? null }),
    setQuickAddOpen: open =>
      setQuickAdd(current => (open ? (current ?? { position: null }) : null)),
    openConnectionDrop: drop => setConnectionDrop(drop),
    closeConnectionDrop: () => setConnectionDrop(null),
  };
}
