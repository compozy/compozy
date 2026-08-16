/**
 * Pure mechanics behind the palette's nested views (Business Rule 32).
 *
 * The stack is the whole navigation model: an empty stack is the root palette,
 * and every pushed frame is one level deeper. Keeping push/pop/breadcrumb and
 * selection resolution as plain functions means the semantics are provable at
 * any depth, independent of how many views happen to be registered — v1 ships
 * one, and the mechanism is the feature.
 */
import type { PaletteViewId } from "./palette-view-registry";

export interface PaletteViewFrame {
  readonly viewId: PaletteViewId;
}

export const PALETTE_VIEW_STACK_ROOT: readonly PaletteViewFrame[] = [];

export function pushPaletteViewFrame(
  stack: readonly PaletteViewFrame[],
  viewId: PaletteViewId
): readonly PaletteViewFrame[] {
  return [...stack, { viewId }];
}

export function popPaletteViewFrame(
  stack: readonly PaletteViewFrame[]
): readonly PaletteViewFrame[] {
  return stack.length === 0 ? stack : stack.slice(0, -1);
}

/** The level the palette is currently showing; `null` is the root. */
export function activePaletteViewId(stack: readonly PaletteViewFrame[]): PaletteViewId | null {
  return stack.at(-1)?.viewId ?? null;
}

/**
 * How many crumbs the breadcrumb may occupy, the `…` included. Ancestors past
 * the leaf and its parent collapse into that one slot, so a deep stack still
 * reads as a path instead of wrapping the input row.
 */
export const PALETTE_BREADCRUMB_MAX_SLOTS = 3;

const PALETTE_BREADCRUMB_VISIBLE = PALETTE_BREADCRUMB_MAX_SLOTS - 1;

export interface PaletteBreadcrumb {
  /** True when ancestors were collapsed into a leading `…`. */
  readonly truncated: boolean;
  readonly visible: readonly string[];
}

/** Left-truncating breadcrumb: the leaf and its parent always survive. */
export function paletteBreadcrumb(labels: readonly string[]): PaletteBreadcrumb {
  return {
    truncated: labels.length > PALETTE_BREADCRUMB_VISIBLE,
    visible: labels.slice(-PALETTE_BREADCRUMB_VISIBLE),
  };
}

/**
 * Where the keyboard selection lands after the rows underneath it changed
 * (US-029.EC-4, US-030.EC-2).
 *
 * A session that survives the refresh keeps the selection — that is the whole
 * point, since a list that re-sorts itself around live attention would
 * otherwise move the operator's target out from under ⏎. Only a selection whose
 * row actually disappeared moves, and then to whatever now occupies its place,
 * which is the nearest neighbour in both directions.
 */
export function resolvePaletteSelection(
  previous: readonly string[],
  next: readonly string[],
  selected: string
): string {
  if (next.includes(selected)) return selected;
  if (next.length === 0) return "";
  const index = previous.indexOf(selected);
  if (index < 0) return next[0] ?? "";
  return next[Math.min(index, next.length - 1)] ?? "";
}
