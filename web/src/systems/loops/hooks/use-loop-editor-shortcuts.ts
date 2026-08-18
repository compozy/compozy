import { useEffect, useEffectEvent } from "react";

import { isEditableTarget } from "@/systems/session";

export interface LoopEditorShortcutHandlers {
  onQuickAdd: () => void;

  onTogglePalette: () => void;

  onToggleInspector: () => void;
}

const CANVAS_SELECTOR = ".react-flow";

function isInsideCanvas(target: EventTarget | null): boolean {
  return target instanceof Element && target.closest(CANVAS_SELECTOR) !== null;
}

export function useLoopEditorShortcuts(
  enabled: boolean,
  handlers: LoopEditorShortcutHandlers
): void {
  const handleKey = useEffectEvent((event: KeyboardEvent) => {
    if (event.metaKey || event.ctrlKey || event.altKey || event.shiftKey) return;
    if (isEditableTarget(event.target)) return;
    if (!isInsideCanvas(event.target)) return;
    if (event.key === "a") {
      event.preventDefault();
      handlers.onQuickAdd();
      return;
    }
    if (event.key === "[") {
      event.preventDefault();
      handlers.onTogglePalette();
      return;
    }
    if (event.key === "]") {
      event.preventDefault();
      handlers.onToggleInspector();
    }
  });

  useEffect(() => {
    if (!enabled) return undefined;
    document.addEventListener("keydown", handleKey);
    return () => document.removeEventListener("keydown", handleKey);
  }, [enabled]);
}
