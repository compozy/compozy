import type { PaletteViewId } from "./palette-view-registry";

type PaletteViewRaiseListener = (viewId: PaletteViewId) => void;

const listeners = new Set<PaletteViewRaiseListener>();

/**
 * Lets a window raise a palette view through the shell's overlay owner.
 *
 * Overlay open lives in the desktop shell. Windows cannot toggle it directly;
 * they publish here, and the shell opens the palette then pushes the view —
 * the same order ⌘E and `palette.view.sessions` already use.
 */
export function subscribePaletteViewRaise(listener: PaletteViewRaiseListener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function raisePaletteView(viewId: PaletteViewId): void {
  for (const listener of listeners) listener(viewId);
}
