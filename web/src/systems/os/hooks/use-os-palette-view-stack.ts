import { windowManagerStore } from "../stores/window-manager-store";
import {
  activePaletteViewId,
  paletteBreadcrumb,
  type PaletteBreadcrumb,
  type PaletteViewFrame,
} from "../lib/palette-view-stack";
import { paletteViewDefinition, type PaletteViewId } from "../lib/palette-view-registry";
import { useWindowPaletteViewStack } from "./use-window-manager-store";

export interface OsPaletteViewStackModel {
  readonly stack: readonly PaletteViewFrame[];
  /** `null` is the root palette. */
  readonly activeViewId: PaletteViewId | null;
  readonly breadcrumb: PaletteBreadcrumb;
  push(viewId: PaletteViewId): void;
  pop(): void;
}

/**
 * The palette's nested-view navigation handle.
 *
 * The path lives in the window-manager store rather than in palette-local
 * state so ⌘E can raise the Sessions view from outside the palette tree, and so
 * the shell can clear it in the same handler that opens or closes the overlay —
 * "reopening starts at root" ends up being a property of the state, not a rule
 * every dismissal path has to remember.
 */
export function useOsPaletteViewStack(): OsPaletteViewStackModel {
  const stack = useWindowPaletteViewStack();
  return {
    stack,
    activeViewId: activePaletteViewId(stack),
    breadcrumb: paletteBreadcrumb(stack.map(frame => paletteViewDefinition(frame.viewId).title)),
    push: viewId => windowManagerStore.trigger.paletteViewPushed({ viewId }),
    pop: () => windowManagerStore.trigger.paletteViewPopped(),
  };
}

/** Opens the palette straight into a view — the ⌘E path (US-030). */
export function openPaletteView(viewId: PaletteViewId): void {
  windowManagerStore.trigger.paletteViewStackSet({ stack: [{ viewId }] });
}

export function resetPaletteViewStack(): void {
  windowManagerStore.trigger.paletteViewStackSet({ stack: [] });
}
